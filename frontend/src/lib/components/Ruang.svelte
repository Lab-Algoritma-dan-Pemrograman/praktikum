<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { labelJenis, renderMath } from '$lib/utils';
	import { sanitizeSoal } from '$lib/sanitize';
	import CodeEditor from './CodeEditor.svelte';
	import Countdown from './Countdown.svelte';
	import { Save, ChevronLeft, ChevronRight, Lock } from 'lucide-svelte';
	import { confirmAction } from '$lib/stores/confirm';
	import type { RuangCourse } from '$lib/types';

	let { aktivasiSesiId, courseId }: { aktivasiSesiId: number; courseId: number } = $props();

	let ruang = $state<RuangCourse | null>(null);
	let answers = $state<Record<number, string>>({});
	let err = $state('');
	let info = $state('');
	let loading = $state(true);
	let submitting = $state(false);
	let dirty = new Set<number>();
	let saveTimer: ReturnType<typeof setInterval>;
	
	let inputToken = $state('');
	let starting = $state(false);

	let locked = $derived(!ruang || !ruang.is_open || ruang.status === 'selesai');

	// --- Navigasi soal ---
	let activeIndex = $state(0);
	let currentSoal = $derived(ruang?.soal[activeIndex] ?? null);

	async function goToSoal(index: number) {
		// Auto-save soal sebelumnya saat pindah
		if (currentSoal && dirty.has(currentSoal.soal_terpilih_id)) {
			await saveOne(currentSoal.soal_terpilih_id);
		}
		activeIndex = index;
	}

	async function load(start = false) {
		try {
			if (start) {
				ruang = await api.post<RuangCourse>('/api/praktikum/mulai', {
					aktivasi_sesi_id: aktivasiSesiId,
					course_id: courseId,
					token: inputToken ? inputToken : null
				});
			} else {
				ruang = await api.get<RuangCourse>(
					`/api/praktikum/ruang?aktivasi_sesi_id=${aktivasiSesiId}&course_id=${courseId}`
				);
			}
			const map: Record<number, string> = {};
			for (const s of ruang.soal) map[s.soal_terpilih_id] = s.jawaban_teks ?? '';
			answers = map;
		} catch (e) {
			err = (e as Error).message;
		} finally {
			loading = false;
		}
	}

	async function saveOne(soalTerpilihId: number) {
		if (locked) return;
		try {
			await api.post('/api/praktikum/autosave', {
				soal_terpilih_id: soalTerpilihId,
				jawaban_teks: answers[soalTerpilihId] ?? ''
			});
			dirty.delete(soalTerpilihId);
			info = 'Tersimpan otomatis ' + new Date().toLocaleTimeString();
		} catch (e) {
			err = (e as Error).message;
		}
	}

	function markDirty(id: number) {
		dirty.add(id);
	}

	async function flush() {
		for (const id of Array.from(dirty)) await saveOne(id);
	}

	async function submit() {
		if (!await confirmAction({
			title: 'Kumpulkan Jawaban?',
			message: 'Apakah Anda yakin ingin mengumpulkan jawaban? Anda tidak dapat mengubahnya lagi setelah dikumpulkan.'
		})) return;
		submitting = true;
		try {
			await flush();
			await api.post('/api/praktikum/submit', {
				aktivasi_sesi_id: aktivasiSesiId,
				course_id: courseId
			});
			info = 'Jawaban berhasil di-submit.';
			await load();
			setTimeout(() => goto('/praktikum/dashboard'), 1000);
		} catch (e) {
			err = (e as Error).message;
		} finally {
			submitting = false;
		}
	}

	async function onExpire() {
		info = 'Waktu habis. Jawaban otomatis ter-submit.';
		await flush().catch(() => {});
		await load();
		setTimeout(() => goto('/praktikum/dashboard'), 1500);
	}

	onMount(async () => {
		await load();
		// Mulai otomatis jika tidak butuh token, belum mulai & masih terbuka.
		if (ruang && ruang.is_open && ruang.status !== 'selesai' && !ruang.waktu_mulai) {
			if (!ruang.require_token) {
				await load(true);
			}
		}
		// Auto-save berkala tiap 15 detik.
		saveTimer = setInterval(flush, 15000);
	});

	onDestroy(() => clearInterval(saveTimer));
</script>

{#if loading}
	<p class="text-ink-caption">Memuat ruang pengerjaan…</p>
{:else if err && !ruang}
	<p class="rounded-lg bg-state-error-bg p-3 text-state-error">{err}</p>
{:else if ruang}
	{#if !ruang.waktu_mulai && ruang.is_open && ruang.status !== 'selesai' && ruang.require_token}
		<!-- LOCK SCREEN -->
		<div class="card max-w-md mx-auto mt-12 text-center p-8 border-t-4 border-t-primary">
			<h2 class="text-2xl font-bold mb-2">Sesi Ujian Terkunci</h2>
			<p class="text-ink-caption mb-6">Masukkan PIN Ujian yang diberikan oleh Asisten untuk memulai pengerjaan.</p>
			
			<div class="mb-4">
				<input type="text" class="input text-center text-2xl font-mono tracking-widest uppercase" placeholder="PIN 6 DIGIT" maxlength="6" bind:value={inputToken} oninput={() => { inputToken = inputToken.toUpperCase(); }} />
			</div>
			
			{#if err}<p class="mb-4 text-sm text-state-error">{err}</p>{/if}
			
			<button class="btn-primary w-full" disabled={!inputToken || starting} onclick={async () => {
				starting = true;
				err = '';
				await load(true);
				starting = false;
			}}>
				{starting ? 'Memverifikasi...' : 'Mulai Ujian'}
			</button>
		</div>
	{:else}
		<div class="mb-4 flex flex-wrap items-center justify-between gap-3">
			<div>
				<h1 class="text-2xl font-bold text-ink-heading">{labelJenis(ruang.jenis)}</h1>
			</div>
			<div class="flex items-center gap-3">
				{#if ruang.deadline && !locked}
					<Countdown deadline={ruang.deadline} {onExpire} />
				{/if}
				{#if ruang.status === 'selesai'}
					<span class="badge bg-state-success-bg text-state-success">Selesai</span>
				{:else if !ruang.is_open}
					<span class="badge bg-gray-100 text-ink-caption">Ditutup admin</span>
				{/if}
			</div>
		</div>

		<!-- Ucapan Motivasi Ujian -->
		<div class="mb-4 rounded-2xl border border-primary/10 bg-gradient-to-r from-primary/5 to-transparent p-4 flex items-center gap-3">
			<span class="text-xl">✨</span>
			<p class="text-sm font-semibold text-slate-800 leading-relaxed">
				Selamat Mengerjakan 🔥💪, Semoga Dilancarkan dan Mendapatkan Nilai Terbaik 🥳🤗🤩
			</p>
		</div>

		{#if err}<p class="mb-3 rounded-lg bg-state-error-bg p-3 text-sm text-state-error">{err}</p>{/if}

		<!-- Jelaskan kenapa editor terkunci. Tanpa ini mahasiswa hanya melihat
		     editor yang tidak bisa diketik tanpa alasan (terjadi 2026-09-28 di PK). -->
		{#if locked}
			<div class="mb-4 rounded-2xl border border-amber-200 bg-amber-50 p-4 flex items-start gap-3">
				<Lock size={18} class="mt-0.5 flex-shrink-0 text-amber-600" />
				<div class="text-sm leading-relaxed text-amber-900">
					{#if ruang.status === 'selesai'}
						<p class="font-bold">Jawaban Anda sudah terkumpul (submit sendiri atau auto-submit saat waktu habis).</p>
						<p class="mt-1">Karena itu editor dikunci dan tidak bisa diketik. Jika asisten membuka kembali sesi ini, muat ulang halaman untuk mengerjakan lagi.</p>
					{:else}
						<p class="font-bold">Sesi ini sedang ditutup oleh asisten.</p>
						<p class="mt-1">Editor dikunci sampai asisten membukanya kembali. Muat ulang halaman setelah dibuka.</p>
					{/if}
				</div>
			</div>
		{/if}
		{#if info}
			<p class="mb-3 rounded-xl border p-3.5 text-xs font-semibold flex items-center gap-2
				{info.includes('Tersimpan otomatis') 
					? 'bg-emerald-50 border-emerald-100 text-emerald-700' 
					: 'bg-state-info-bg border-state-info/10 text-state-info'}">
				{#if info.includes('Tersimpan otomatis')}
					<span class="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-ping"></span>
				{/if}
				{info}
			</p>
		{/if}

	<!-- Progress Bar -->
	{@const answeredCount = ruang.soal.filter((s) => answers[s.soal_terpilih_id]).length}
	{@const progressPct = ruang.soal.length > 0 ? Math.round((answeredCount / ruang.soal.length) * 100) : 0}
	<div class="mb-4 rounded-xl border border-gray-200 bg-white p-4 shadow-sm">
		<div class="flex items-center justify-between mb-2">
			<span class="text-xs font-bold text-ink-caption uppercase tracking-wider">Progress Jawaban</span>
			<span class="text-xs font-bold text-primary">{answeredCount} / {ruang.soal.length} terjawab ({progressPct}%)</span>
		</div>
		<div class="h-2 w-full rounded-full bg-gray-100 overflow-hidden">
			<div class="h-full rounded-full bg-primary transition-all duration-300" style="width: {progressPct}%"></div>
		</div>
	</div>

	<!-- Navigasi nomor soal (scrollable) -->
	<div class="mb-4 overflow-x-auto pb-2 -mx-1 px-1">
		<div class="flex gap-2 min-w-max">
			{#each ruang.soal as s, i}
				<button
					class="grid h-10 w-10 flex-shrink-0 place-items-center rounded-lg text-sm font-bold transition-all duration-200
					{i === activeIndex ? 'bg-primary text-white shadow-lg scale-110' : 'bg-surface-soft text-ink-body hover:bg-primary/10'}
					{answers[s.soal_terpilih_id] ? 'ring-2 ring-state-success ring-offset-1' : ''}"
					onclick={() => goToSoal(i)}
				>
					{i + 1}
				</button>
			{/each}
		</div>
	</div>

	<!-- Soal aktif -->
	{#if currentSoal}
	{@const s = currentSoal}
	<div class="grid grid-cols-1 lg:grid-cols-12 gap-6 items-stretch">
		<div class="lg:col-span-5 flex flex-col">
			<div class="flex-grow flex flex-col overflow-hidden lg:h-[650px] rounded-2xl border-y border-r border-slate-200 border-l-4 border-l-primary bg-white text-slate-800 shadow-sm p-6">
				<!-- Header Soal (Title, Kategori, Jenis, Poin) -->
				<div class="mb-4 flex items-center justify-between pb-3 border-b border-slate-100 flex-shrink-0">
					<div class="flex flex-wrap items-center gap-2">
						<h3 class="text-lg font-bold text-slate-800 flex items-center gap-1.5">
							Soal {activeIndex + 1}
						</h3>
						{#if s.kategori_ujian}<span class="badge bg-slate-100 text-slate-700">{s.kategori_ujian}</span>{/if}
						<span class="badge {s.jenis_soal === 'coding' ? 'bg-emerald-50 text-emerald-700 border border-emerald-100' : 'bg-blue-50 text-blue-700 border border-blue-100'}">{s.jenis_soal}</span>
					</div>
					<span class="text-sm font-bold text-primary bg-primary/5 border border-primary/10 px-2.5 py-1 rounded-lg flex-shrink-0">{s.poin} poin</span>
				</div>
				
				<!-- Isi Soal (Scrollable) -->
				<div class="flex-grow overflow-y-auto pr-1 text-left custom-scrollbar">
					<div class="prose prose-sm max-w-none text-slate-700 leading-relaxed" use:renderMath={s.teks_soal}>
						{@html sanitizeSoal(s.teks_soal)}
					</div>
					{#if s.gambar_url}
						<div class="mt-4 overflow-hidden rounded-xl border border-slate-250 shadow-inner bg-slate-50 p-2">
							<img src={s.gambar_url} alt="Visualisasi Soal" class="mx-auto max-w-full h-auto object-contain rounded-lg" />
						</div>
					{/if}
				</div>
			</div>
		</div>

		<!-- Jendela Kanan: Code Editor / Jawaban Essay -->
		<div class="lg:col-span-7 flex flex-col">
			<div class="flex-grow flex flex-col lg:h-[650px] text-left">
				{#key s.soal_terpilih_id}
					{#if s.jenis_soal === 'coding'}
						<div class="flex-grow flex flex-col min-h-0">
							<CodeEditor
								bind:value={answers[s.soal_terpilih_id]}
								readonly={locked}
								language="c"
								runnable={true}
								height="300px"
								oninput={() => markDirty(s.soal_terpilih_id)}
							/>
						</div>
					{:else}
						<div class="flex-grow flex flex-col min-h-0 bg-white border border-slate-200 shadow-sm rounded-2xl p-6">
							<!-- Header Essay -->
							<div class="mb-4 flex items-center justify-between pb-3 border-b border-slate-100 flex-shrink-0">
								<h4 class="text-sm font-bold text-slate-500 uppercase tracking-wider flex items-center gap-2">
									<span class="w-2.5 h-2.5 rounded-full bg-blue-500 animate-pulse"></span>
									Lembar Kerja Essay
								</h4>
							</div>
							
							<div class="flex-grow flex flex-col min-h-[350px]">
								<textarea
									class="input flex-grow min-h-[250px] font-mono text-sm leading-relaxed p-4 border border-slate-200 focus:border-primary focus:ring-1 focus:ring-primary rounded-xl"
									bind:value={answers[s.soal_terpilih_id]}
									readonly={locked}
									placeholder="Ketikkan jawaban essay Anda di sini..."
									oninput={() => markDirty(s.soal_terpilih_id)}
									onblur={() => saveOne(s.soal_terpilih_id)}
								></textarea>
							</div>
						</div>
					{/if}
				{/key}
			</div>
		</div>
	</div>

	<!-- Prev / Next -->
	<div class="mt-4 flex items-center justify-between gap-3">
		<button
			class="btn-outline inline-flex items-center gap-2 px-5 py-2.5 text-sm font-semibold"
			disabled={activeIndex === 0}
			onclick={() => goToSoal(activeIndex - 1)}
		>
			<ChevronLeft size={18} /> Sebelumnya
		</button>
		<span class="rounded-lg bg-gray-100 px-3 py-1.5 text-sm font-bold text-ink-caption">{activeIndex + 1} / {ruang.soal.length}</span>
		<button
			class="btn-outline inline-flex items-center gap-2 px-5 py-2.5 text-sm font-semibold"
			disabled={activeIndex === ruang.soal.length - 1}
			onclick={() => goToSoal(activeIndex + 1)}
		>
			Selanjutnya <ChevronRight size={18} />
		</button>
	</div>
	{/if}

	{#if !locked}
		<div class="sticky bottom-4 mt-6 flex justify-end">
			<button class="btn-primary px-8 py-3 text-sm font-bold shadow-lg" onclick={submit} disabled={submitting}>
				{submitting ? 'Mengirim…' : 'Submit Jawaban'}
			</button>
		</div>
	{/if}
	{/if}
{/if}
