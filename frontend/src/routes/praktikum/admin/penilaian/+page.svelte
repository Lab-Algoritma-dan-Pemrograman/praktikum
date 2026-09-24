<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { labelJenis, labelShift, renderMath } from '$lib/utils';
	import { sanitizeSoal } from '$lib/sanitize';
	import { ChevronDown, ChevronUp, Search, SlidersHorizontal, Check, ArrowUp } from 'lucide-svelte';
	import { confirmAction } from '$lib/stores/confirm';
	import type { Kelas } from '$lib/types';

	interface AktivasiSesi {
		id: number; sesi_praktikum_id: number; kelas_id: number; shift: number;
		sesi?: { judul_sesi: string };
		kelas?: { nama_kelas: string };
		aktivasi_courses?: { id: number; course_id: number; course?: { jenis: string; judul: string } }[];
	}
	interface RekapItem {
		jawaban_id: number; mahasiswa_id: number; nama_mahasiswa: string; nim: string;
		soal_id: number; teks_soal: string; poin: number;
		jawaban_teks: string; is_submitted: boolean;
		nilai: number | null; feedback: string | null;
	}

	let aktivasiList = $state<AktivasiSesi[]>([]);
	let err = $state(''); let msg = $state('');

	let selectedAktivasi = $state<AktivasiSesi | null>(null);
	let selectedCourseId = $state<number | null>(null);
	let rekap = $state<RekapItem[]>([]);
	let loading = $state(false);

	// Filtering state
	let searchFilter = $state('');
	let statusFilter = $state<'semua' | 'dinilai' | 'belum'>('semua');

	// Expandable state
	let expandedIds = $state<Set<number>>(new Set());

	// AI grading sinkron
	let aiRunning = $state(false);
	let aiProcessed = $state(0);
	let aiTotal = $state(0);
	let aiFailed = $state(0);
	let aiFailedItems = $state<{ jawaban_id: number; nama: string; nim: string }[]>([]);
	let aiStopRequested = false;

	let nilaiEdits = $state<Record<number, { nilai: number; feedback: string }>>({});
	// Checklist bulk-select untuk grading sebagian
	let selected = $state<Record<number, boolean>>({});
	let selectedCount = $derived(rekap.filter((r) => selected[r.jawaban_id]).length);

	let expandedStudentIds = $state<Set<number>>(new Set());

	async function loadAktivasi() {
		try { aktivasiList = (await api.get<AktivasiSesi[]>('/api/admin/aktivasi')) ?? []; }
		catch (e) { err = (e as Error).message; }
	}

	// --- Bulk lintassesi ---
	let bulkSelected = $state<Record<number, boolean>>({});
	const bulkSelectedCount = $derived(aktivasiList.filter((a) => bulkSelected[a.id]).length);
	
	function toggleAllBulk() {
		const semua = bulkSelectedCount === aktivasiList.length;
		const s: Record<number, boolean> = {};
		if (!semua) for (const a of aktivasiList) s[a.id] = true;
		bulkSelected = s;
	}
	
	async function gradeBulkSessions() {
		const sessions = aktivasiList.filter((a) => bulkSelected[a.id]);
		if (sessions.length === 0) { msg = 'Pilih minimal 1 sesi.'; return; }
		if (!await confirmAction({
			title: 'Mulai Penilaian AI?',
			message: `Apakah Anda yakin ingin memulai penilaian AI untuk SEMUA jawaban belum dinilai di ${sessions.length} sesi terpilih (semua course)?`
		})) return;
		err = ''; msg = '';
		const ids: number[] = [];
		try {
			for (const a of sessions) {
				for (const ac of a.aktivasi_courses ?? []) {
					const res = await api.get<{ jawaban_ids: number[]; total: number }>(
						`/api/admin/penilaian/ai-grade/targets?aktivasi_sesi_id=${a.id}&course_id=${ac.course_id}`
					);
					ids.push(...(res?.jawaban_ids ?? []));
				}
			}
		} catch (e) { err = (e as Error).message; return; }
		await gradeIds(ids);
	}

	// Model AI grading (override)
	let aiModel = $state('');
	let aiModelSaving = $state(false);
	let aiModelEffective = $state('');
	let aiModelDefault = $state('');
	
	async function loadAiModel() {
		try {
			const m = await api.get<{ effective: string; override: string; default: string }>('/api/admin/konfigurasi/ai-model');
			aiModel = m?.override ?? '';
			aiModelEffective = m?.effective ?? '';
			aiModelDefault = m?.default ?? '';
		} catch { /* ignore */ }
	}
	
	async function saveAiModel() {
		aiModelSaving = true; err = ''; msg = '';
		try {
			await api.post('/api/admin/konfigurasi', { key: 'ai_model', value: aiModel.trim() });
			msg = aiModel.trim() ? `Model AI di-set ke "${aiModel.trim()}".` : 'Model AI dikembalikan ke default server.';
			await loadAiModel();
		} catch (e) { err = (e as Error).message; }
		finally { aiModelSaving = false; }
	}

	let showScrollTop = $state(false);

	function scrollToTop() {
		window.scrollTo({ top: 0, behavior: 'smooth' });
	}

	onMount(() => {
		loadAktivasi();
		loadAiModel();
		const handleScroll = () => {
			showScrollTop = window.scrollY > 300;
		};
		window.addEventListener('scroll', handleScroll);
		return () => window.removeEventListener('scroll', handleScroll);
	});

	async function selectAktivasi(a: AktivasiSesi) {
		selectedAktivasi = a; selectedCourseId = null; rekap = [];
		try {
			const detail = await api.get<AktivasiSesi>(`/api/admin/aktivasi/${a.id}`);
			if (detail) selectedAktivasi = detail;
		} catch (e) { err = (e as Error).message; }
	}

	async function loadRekap(courseId: number) {
		if (!selectedAktivasi) return;
		selectedCourseId = courseId; loading = true; err = '';
		try {
			const res = await api.get<{ items: RekapItem[] }>(
				`/api/admin/penilaian/rekap?aktivasi_sesi_id=${selectedAktivasi.id}&course_id=${courseId}`
			);
			rekap = res?.items ?? [];
			const edits: Record<number, { nilai: number; feedback: string }> = {};
			for (const r of rekap) {
				edits[r.jawaban_id] = { nilai: r.nilai ?? 0, feedback: r.feedback ?? '' };
			}
			nilaiEdits = edits;
			expandedStudentIds = new Set(); // reset expand status
		} catch (e) { err = (e as Error).message; }
		finally { loading = false; }
	}

	async function simpanNilai(jawabanId: number) {
		err = ''; msg = '';
		const edit = nilaiEdits[jawabanId];
		if (!edit) return;
		try {
			await api.post('/api/admin/penilaian', {
				jawaban_id: jawabanId,
				nilai: Number(edit.nilai),
				feedback: edit.feedback || null
			});
			msg = 'Nilai disimpan.';
			if (selectedCourseId) await loadRekap(selectedCourseId);
		} catch (e) { err = (e as Error).message; }
	}

	const AI_GAP_MS = 1100;

	async function gradeIds(ids: number[]) {
		if (ids.length === 0) { msg = 'Tidak ada jawaban untuk dinilai.'; return; }
		err = ''; msg = '';
		aiProcessed = 0; aiFailed = 0; aiFailedItems = []; aiStopRequested = false;
		aiTotal = ids.length; aiRunning = true;

		const gradeOne = async (id: number) => {
			try { await api.post('/api/admin/penilaian/ai-grade/one', { jawaban_id: id }); }
			catch {
				aiFailed++;
				const r = rekap.find((x) => x.jawaban_id === id);
				aiFailedItems = [...aiFailedItems, { jawaban_id: id, nama: r?.nama_mahasiswa ?? '?', nim: r?.nim ?? '-' }];
			}
			aiProcessed++;
		};

		try {
			const running: Promise<void>[] = [];
			for (const id of ids) {
				if (aiStopRequested) break;
				running.push(gradeOne(id));
				await new Promise((r) => setTimeout(r, AI_GAP_MS));
			}
			await Promise.all(running);
		} finally {
			aiRunning = false;
			const sukses = aiProcessed - aiFailed;
			msg = `AI grading selesai: ${sukses} berhasil`
				+ (aiFailed ? `, ${aiFailed} gagal (lihat daftar di bawah)` : '')
				+ (aiStopRequested ? ' — dihentikan' : '') + '.';
			if (selectedCourseId) await loadRekap(selectedCourseId);
		}
	}

	async function startAIGrading() {
		if (!selectedAktivasi || !selectedCourseId) return;
		if (!await confirmAction({
			title: 'Penilaian AI Massal?',
			message: 'Apakah Anda yakin ingin memulai penilaian AI untuk SEMUA jawaban yang belum dinilai?'
		})) return;
		try {
			const res = await api.get<{ jawaban_ids: number[]; total: number }>(
				`/api/admin/penilaian/ai-grade/targets?aktivasi_sesi_id=${selectedAktivasi.id}&course_id=${selectedCourseId}`
			);
			await gradeIds(res?.jawaban_ids ?? []);
		} catch (e) { err = (e as Error).message; }
	}

	async function retryFailed() {
		const ids = aiFailedItems.map((x) => x.jawaban_id);
		if (ids.length === 0) return;
		await gradeIds(ids);
	}

	async function gradeSelected() {
		const ids = rekap.filter((r) => selected[r.jawaban_id]).map((r) => r.jawaban_id);
		if (ids.length === 0) { msg = 'Belum ada jawaban yang dipilih.'; return; }
		if (!await confirmAction({
			title: 'Nilai Terpilih dengan AI?',
			message: `Apakah Anda yakin ingin menilai ${ids.length} jawaban terpilih dengan AI?`
		})) return;
		await gradeIds(ids);
		selected = {};
	}

	async function resetSelected() {
		const ids = rekap.filter((r) => selected[r.jawaban_id]).map((r) => r.jawaban_id);
		if (ids.length === 0) { msg = 'Belum ada jawaban yang dipilih.'; return; }
		if (!await confirmAction({
			title: 'Reset Nilai Terpilih?',
			message: `Apakah Anda yakin ingin mereset nilai ${ids.length} jawaban terpilih jadi kosong? Feedback juga akan dihapus.`
		})) return;
		err = ''; msg = '';
		try {
			await api.post('/api/admin/penilaian/bulk-action', { action: 'reset_nilai', jawaban_ids: ids });
			msg = `${ids.length} nilai direset.`;
			selected = {};
			if (selectedCourseId) await loadRekap(selectedCourseId);
		} catch (e) { err = (e as Error).message; }
	}

	async function resetSemua() {
		const ids = rekap.map((r) => r.jawaban_id);
		if (ids.length === 0) { msg = 'Tidak ada jawaban untuk direset.'; return; }
		if (!await confirmAction({
			title: 'GENOSIDA NILAI!',
			message: `Apakah Anda yakin ingin mereset SEMUA ${ids.length} nilai di course ini jadi kosong? (Jawaban TIDAK dihapus, feedback ikut terhapus). Tindakan ini tidak bisa dibatalkan!`
		})) return;
		if (!await confirmAction({
			title: 'Konfirmasi Ulang Genosida',
			message: `Yakin? Ini menimpa nilai ${ids.length} jawaban (termasuk yang sudah dinilai).`
		})) return;
		err = ''; msg = '';
		try {
			await api.post('/api/admin/penilaian/bulk-action', { action: 'reset_nilai', jawaban_ids: ids });
			msg = `${ids.length} nilai direset (genosida).`;
			selected = {};
			if (selectedCourseId) await loadRekap(selectedCourseId);
		} catch (e) { err = (e as Error).message; }
	}

	function pilihSemua() {
		const s: Record<number, boolean> = {};
		for (const r of filteredRekap) s[r.jawaban_id] = true;
		selected = s;
	}
	function pilihBelumDinilai() {
		const s: Record<number, boolean> = {};
		for (const r of filteredRekap) if (r.is_submitted && r.nilai == null && r.jawaban_teks?.trim()) s[r.jawaban_id] = true;
		selected = s;
	}
	function kosongkanPilihan() { selected = {}; }

	function stopAIGrading() { aiStopRequested = true; }

	function toggleStudentExpand(id: number) {
		const s = new Set(expandedStudentIds);
		s.has(id) ? s.delete(id) : s.add(id);
		expandedStudentIds = s;
	}

	function toggleStudentCheckbox(g: any, event: Event) {
		const checked = (event.target as HTMLInputElement).checked;
		const nextSelected = { ...selected };
		for (const a of g.answers) {
			nextSelected[a.jawaban_id] = checked;
		}
		selected = nextSelected;
	}

	// Filter computed
	let filteredRekap = $derived(
		rekap.filter((r) => {
			const query = searchFilter.toLowerCase().trim();
			const matchSearch = query === '' || r.nama_mahasiswa.toLowerCase().includes(query) || r.nim.toLowerCase().includes(query);
			
			const matchStatus =
				statusFilter === 'semua' ||
				(statusFilter === 'dinilai' && r.nilai !== null) ||
				(statusFilter === 'belum' && r.nilai === null);
				
			return matchSearch && matchStatus;
		})
	);

	let groupedRekap = $derived.by(() => {
		const groupsMap = new Map<number, {
			mahasiswa_id: number;
			nama_mahasiswa: string;
			nim: string;
			is_submitted: boolean;
			answers: {
				index: number;
				jawaban_id: number;
				soal_id: number;
				teks_soal: string;
				poin: number;
				jawaban_teks: string;
				nilai: number | null;
				feedback: string | null;
			}[];
		}>();

		for (const r of filteredRekap) {
			if (!groupsMap.has(r.mahasiswa_id)) {
				groupsMap.set(r.mahasiswa_id, {
					mahasiswa_id: r.mahasiswa_id,
					nama_mahasiswa: r.nama_mahasiswa,
					nim: r.nim,
					is_submitted: r.is_submitted,
					answers: []
				});
			}
			const group = groupsMap.get(r.mahasiswa_id)!;
			group.answers.push({
				index: group.answers.length + 1,
				jawaban_id: r.jawaban_id,
				soal_id: r.soal_id,
				teks_soal: r.teks_soal,
				poin: r.poin,
				jawaban_teks: r.jawaban_teks,
				nilai: r.nilai,
				feedback: r.feedback
			});
		}
		return Array.from(groupsMap.values());
	});
</script>

<div class="space-y-6 text-left">
	<h1 class="text-2xl font-bold text-slate-900 leading-none">Penilaian Mahasiswa</h1>
	<p class="text-sm text-slate-500 font-semibold -mt-2">Penilaian jawaban pretest, posttest, keterampilan, dan ujian menggunakan asisten AI.</p>

	{#if msg}<p class="rounded-lg bg-state-success-bg p-3 text-sm text-state-success font-semibold">{msg}</p>{/if}
	{#if err}<p class="rounded-lg bg-state-error-bg p-3 text-sm text-state-error font-semibold">{err}</p>{/if}

	{#if !aiRunning && aiFailedItems.length > 0}
		<div class="rounded-lg border border-state-error/30 bg-state-error-bg p-3 text-sm">
			<div class="mb-2 flex flex-wrap items-center justify-between gap-2">
				<span class="font-medium text-state-error">{aiFailedItems.length} jawaban gagal dinilai:</span>
				<button class="btn-outline border-state-error px-3 py-1 text-xs text-state-error hover:bg-state-error hover:text-white" onclick={retryFailed}>Ulangi yang gagal ({aiFailedItems.length})</button>
			</div>
			<ul class="ml-4 list-disc space-y-0.5 text-ink-body">
				{#each aiFailedItems as f}
					<li>{f.nama} <span class="text-ink-caption">({f.nim})</span></li>
				{/each}
			</ul>
		</div>
	{/if}

	<div class="grid gap-6 lg:grid-cols-4 items-start">
		<!-- Sidebar Aktivasi -->
		<div class="card">
			<div class="mb-3 flex items-center justify-between">
				<h2 class="text-base font-bold text-slate-800">Aktivasi Sesi</h2>
				{#if aktivasiList.length}
					<button class="text-xs font-bold text-primary hover:underline" onclick={toggleAllBulk}>
						{bulkSelectedCount === aktivasiList.length ? 'Batal semua' : 'Centang semua'}
					</button>
				{/if}
			</div>
			
			{#if bulkSelectedCount > 0}
				<button class="btn-primary mb-3 w-full py-2 text-xs font-bold shadow-sm" onclick={gradeBulkSessions} disabled={aiRunning}>
					Nilai AI ({bulkSelectedCount} sesi)
				</button>
			{/if}
			
			<div class="space-y-2 max-h-[400px] overflow-y-auto">
				{#each aktivasiList as a}
					<div class="flex items-center gap-2">
						<input
							type="checkbox"
							class="shrink-0 rounded border-slate-350"
							checked={!!bulkSelected[a.id]}
							onchange={(e) => (bulkSelected = { ...bulkSelected, [a.id]: (e.target as HTMLInputElement).checked })}
						/>
						<button
							class="flex-1 rounded-xl border p-3 text-left transition hover:bg-slate-50/80 {selectedAktivasi?.id === a.id ? 'border-primary bg-slate-50' : 'border-gray-200'}"
							onclick={() => selectAktivasi(a)}
						>
							<p class="font-bold text-xs text-slate-800 truncate">{a.sesi?.judul_sesi ?? `Sesi #${a.sesi_praktikum_id}`}</p>
							<p class="text-[10px] text-slate-400 font-bold mt-0.5 truncate">{a.kelas?.nama_kelas ?? a.kelas_id} · {labelShift(a.shift)}</p>
						</button>
					</div>
				{/each}
			</div>
		</div>

		<!-- Main Panel Rekap Penilaian -->
		<div class="lg:col-span-3 space-y-4">
			{#if selectedAktivasi}
				<!-- Tab Courses -->
				<div class="flex flex-wrap gap-2">
					{#each selectedAktivasi.aktivasi_courses ?? [] as ac}
						<button
							class="badge cursor-pointer px-3 py-1.5 font-bold transition-all text-xs {selectedCourseId === ac.course_id ? 'bg-primary text-white border-primary shadow-sm' : 'bg-slate-100 border-slate-200 text-slate-600 hover:bg-slate-200'}"
							onclick={() => loadRekap(ac.course_id)}
						>{ac.course?.judul ?? labelJenis(ac.course?.jenis ?? '')}</button>
					{/each}
				</div>

				{#if loading}
					<p class="text-ink-caption">Memuat rekap…</p>
				{:else if selectedCourseId && rekap.length === 0}
					<p class="text-slate-400 font-semibold py-8 text-center bg-slate-50 border border-slate-150 rounded-2xl">Belum ada jawaban praktikan yang ter-submit pada course ini.</p>
				{:else if rekap.length > 0}
					<!-- AI Tooling Container -->
					{#if aiRunning}
						<div class="rounded-2xl border border-primary/20 bg-primary/5 p-4 space-y-3 shadow-inner">
							<div class="flex items-center justify-between">
								<h3 class="font-bold text-sm text-primary">AI Grading sedang berjalan…</h3>
								<button class="btn-outline border-state-error px-3 py-1 text-xs font-bold text-state-error hover:bg-state-error hover:text-white" onclick={stopAIGrading}>Hentikan</button>
							</div>
							<div class="h-2 w-full overflow-hidden rounded-full bg-gray-200">
								<div class="h-full bg-primary transition-all duration-300 animate-pulse" style="width: {aiTotal > 0 ? (aiProcessed / aiTotal) * 100 : 0}%"></div>
							</div>
							<p class="text-xs font-bold text-slate-600">
								Diproses: {aiProcessed} / {aiTotal} {aiFailed ? `· Gagal: ${aiFailed}` : ''}
							</p>
						</div>
					{:else}
						<div class="rounded-2xl border border-gray-200 bg-slate-50/60 p-4 space-y-4 shadow-sm">
							<div class="flex flex-wrap items-start justify-between gap-3">
								<div>
									<h3 class="font-bold text-slate-800 text-sm">Otomatisasi Penilaian (AI)</h3>
									<p class="text-xs text-slate-500 font-semibold mt-0.5">Nilai jawaban praktikan yang tercentang secara batch/sekaligus.</p>
								</div>
								<div class="flex flex-wrap gap-1.5">
									<button class="btn-outline py-1.5 px-3 text-xs font-bold" onclick={gradeSelected} disabled={selectedCount === 0}>Nilai Terpilih ({selectedCount})</button>
									<button class="btn-outline border-state-error/30 py-1.5 px-3 text-xs font-bold text-state-error hover:bg-state-error" onclick={resetSelected} disabled={selectedCount === 0}>Reset Terpilih</button>
									<button class="btn-outline border-state-error/30 py-1.5 px-3 text-xs font-bold text-state-error hover:bg-state-error" onclick={resetSemua}>Reset Semua</button>
									<button class="btn-primary py-1.5 px-3.5 text-xs flex items-center gap-1" onclick={startAIGrading}>
										Nilai Semua
									</button>
								</div>
							</div>
							
							<div class="flex flex-wrap gap-4 text-xs font-bold border-t border-slate-200/60 pt-3">
								<button class="text-primary hover:underline" onclick={pilihSemua}>Pilih semua</button>
								<button class="text-primary hover:underline" onclick={pilihBelumDinilai}>Pilih yang belum dinilai</button>
								<button class="text-slate-400 hover:underline" onclick={kosongkanPilihan}>Kosongkan pilihan</button>
							</div>

							<div class="border-t border-slate-200/60 pt-3 flex flex-col gap-2">
								<label for="aiModel" class="block text-[10px] font-bold text-slate-400 uppercase tracking-wider">Override Model AI</label>
								{#if aiModelEffective}
									<p class="text-xs text-slate-500 font-semibold -mt-1">
										Model aktif saat ini: <span class="font-mono text-primary font-bold">{aiModelEffective}</span>
										{#if !aiModel}<span class="text-[10px] text-slate-400 font-bold"> (Default Server)</span>{/if}
									</p>
								{/if}
								<div class="flex flex-wrap items-center gap-2">
									<input id="aiModel" class="input flex-1 min-w-[200px] font-mono text-xs" placeholder={aiModelDefault ? `default: ${aiModelDefault}` : 'Ketik nama model...'} bind:value={aiModel} />
									<button class="btn-outline py-2 text-xs font-bold" onclick={saveAiModel} disabled={aiModelSaving}>{aiModelSaving ? 'Menyimpan…' : 'Simpan Model'}</button>
								</div>
							</div>
						</div>
					{/if}

					<!-- Filter Controls for Student Answers -->
					<div class="flex flex-col gap-3 sm:flex-row sm:items-center rounded-xl border border-slate-200 bg-white p-3 shadow-sm">
						<div class="relative flex-1 max-w-xs">
							<Search size={14} class="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
							<input type="text" placeholder="Cari Nama/NIM..." bind:value={searchFilter} class="input pl-9 w-full text-xs" />
						</div>
						<div class="flex items-center gap-2">
							<SlidersHorizontal size={12} class="text-slate-400" />
							<select bind:value={statusFilter} class="input w-auto text-xs bg-white border border-slate-200">
								<option value="semua">Semua Status</option>
								<option value="dinilai">Sudah Dinilai</option>
								<option value="belum">Belum Dinilai</option>
							</select>
						</div>
					</div>

					<!-- Expandable Answers List Grouped by Student -->
					<div class="space-y-3">
						{#each groupedRekap as g}
							{@const isExpanded = expandedStudentIds.has(g.mahasiswa_id)}
							{@const gradedCount = g.answers.filter(a => a.nilai !== null).length}
							{@const totalQuestions = g.answers.length}
							<div class="rounded-2xl border border-slate-200 bg-white shadow-sm hover:shadow-md transition-all duration-200">
								<!-- Card Header (Student Info Summary) -->
								<button
									class="flex w-full items-center justify-between gap-4 p-4 text-left rounded-2xl hover:bg-slate-50/50 transition-colors"
									onclick={() => toggleStudentExpand(g.mahasiswa_id)}
								>
									<div class="flex items-center gap-3 min-w-0">
										<input
											type="checkbox"
											class="h-4 w-4 rounded border-slate-350 flex-shrink-0"
											checked={g.answers.every(a => selected[a.jawaban_id])}
											onchange={(e) => toggleStudentCheckbox(g, e)}
											onclick={(e) => e.stopPropagation()}
											title="Pilih semua soal mahasiswa ini untuk dinilai AI"
										/>
										<div class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary text-xs font-bold font-mono">
											{g.nama_mahasiswa.substring(0, 2).toUpperCase()}
										</div>
										<div class="min-w-0">
											<span class="font-bold text-slate-800 text-sm block sm:inline mr-2">{g.nama_mahasiswa}</span>
											<span class="text-xs font-mono font-semibold text-slate-400">{g.nim}</span>
										</div>
									</div>

									<div class="flex items-center gap-2.5 flex-shrink-0">
										<span class="badge {g.is_submitted ? 'bg-state-success-bg text-state-success' : 'bg-state-warning-bg text-state-warning'} text-[10px] font-bold">
											{g.is_submitted ? 'Submitted' : 'Belum Submit'}
										</span>

										<span class="badge {gradedCount === totalQuestions ? 'bg-emerald-50 text-emerald-700 border-emerald-250' : 'bg-amber-50 text-amber-700 border-amber-250'} text-[10px] font-bold border">
											{gradedCount} / {totalQuestions} Dinilai
										</span>

										<div class="text-slate-400 transition-transform duration-200 {isExpanded ? 'rotate-180' : ''}">
											<ChevronDown size={16} />
										</div>
									</div>
								</button>

								<!-- Card Body (Expandable list of questions/answers) -->
								{#if isExpanded}
									<div class="border-t border-slate-100 p-5 space-y-6 bg-slate-50/20 rounded-b-2xl">
										{#each g.answers as a, idx}
											<div class="space-y-3.5 {idx > 0 ? 'border-t border-slate-200/60 pt-5' : ''}">
												<!-- Question Header -->
												<div class="flex items-center justify-between gap-2.5">
													<div class="flex items-center gap-2">
														<input
															type="checkbox" class="h-4 w-4 rounded border-slate-350 flex-shrink-0"
															checked={!!selected[a.jawaban_id]}
															onchange={(e) => (selected[a.jawaban_id] = (e.target as HTMLInputElement).checked)}
															title="Pilih untuk dinilai AI"
														/>
														<h4 class="text-sm font-bold text-slate-800">Soal #{a.index} <span class="text-xs text-slate-400 font-semibold">({a.poin} Poin Maks)</span></h4>
													</div>

													<div>
														{#if a.nilai !== null}
															<span class="badge bg-primary/10 text-primary border border-primary/20 text-[10px] font-extrabold">
																Skor: {a.nilai} / {a.poin}
															</span>
														{:else}
															<span class="badge bg-slate-100 border border-slate-250 text-slate-500 text-[10px] font-bold">
																Belum Dinilai
															</span>
														{/if}
													</div>
												</div>

												<!-- Question content -->
												<div class="rounded-xl border border-slate-200 bg-white p-3.5 shadow-sm">
													<p class="text-[10px] font-bold text-slate-450 uppercase tracking-wider mb-1.5">Pertanyaan:</p>
													<div class="prose prose-sm max-w-none text-slate-800 leading-relaxed" use:renderMath>
														{@html sanitizeSoal(a.teks_soal)}
													</div>
												</div>

												<!-- Student's response text -->
												<div class="rounded-xl border border-slate-200 bg-white p-3.5 shadow-sm">
													<p class="text-[10px] font-bold text-slate-450 uppercase tracking-wider mb-1.5">Jawaban Praktikan:</p>
													{#if a.jawaban_teks}
														<pre class="whitespace-pre-wrap font-mono text-xs text-slate-800 bg-slate-50 p-3 rounded-lg border border-slate-100 leading-relaxed overflow-x-auto">{a.jawaban_teks}</pre>
													{:else}
														<p class="text-xs font-semibold text-slate-400 py-1.5 italic">(Tidak ada jawaban/kosong)</p>
													{/if}
												</div>

												<!-- Evaluation score form -->
												{#if nilaiEdits[a.jawaban_id]}
													<div class="rounded-xl border border-slate-200 bg-white p-3.5 shadow-sm space-y-3">
														<p class="text-[10px] font-bold text-slate-450 uppercase tracking-wider border-b border-slate-100 pb-1.5">Koreksi Nilai &amp; Feedback</p>
														<div class="flex flex-col gap-3 sm:flex-row sm:items-end">
															<div class="w-28 flex-shrink-0">
																<label class="label font-bold text-slate-600 text-xs" for={`n${a.jawaban_id}`}>Nilai (Maks: {a.poin})</label>
																<input id={`n${a.jawaban_id}`} type="number" class="input mt-1 w-full text-xs" bind:value={nilaiEdits[a.jawaban_id].nilai} min="0" max={a.poin} />
															</div>
															<div class="flex-1">
																<label class="label font-bold text-slate-600 text-xs" for={`f${a.jawaban_id}`}>Feedback / Ulasan</label>
																<input id={`f${a.jawaban_id}`} class="input mt-1 w-full text-xs" bind:value={nilaiEdits[a.jawaban_id].feedback} placeholder="Tulis masukan asisten..." />
															</div>
															<button class="btn-primary py-2 px-4.5 text-xs font-bold flex items-center gap-1 flex-shrink-0" onclick={() => simpanNilai(a.jawaban_id)}>
																<Check size={14} /> Simpan
															</button>
														</div>
													</div>
												{/if}
											</div>
										{/each}
									</div>
								{/if}
							</div>
						{/each}
					</div>
				{/if}
			{:else}
				<p class="text-slate-400 font-semibold py-8 text-center bg-slate-50 border border-dashed border-slate-200 rounded-2xl">Silakan pilih aktivasi sesi di sebelah kiri dan klik course di atas untuk memulai penilaian.</p>
			{/if}
		</div>
	</div>

	{#if showScrollTop}
		<button
			onclick={scrollToTop}
			class="fixed bottom-6 right-6 z-50 flex h-10 w-10 items-center justify-center rounded-full bg-primary text-white shadow-lg transition-all hover:bg-primary-hover hover:-translate-y-0.5 focus:outline-none"
			aria-label="Kembali ke atas"
		>
			<ArrowUp size={18} />
		</button>
	{/if}
</div>
