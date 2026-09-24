<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { labelJenis, renderMath } from '$lib/utils';
	import { sanitizeSoal } from '$lib/sanitize';
	import { X, Search, ArrowUp, Edit, Trash2, Eye } from 'lucide-svelte';
	import { confirmAction } from '$lib/stores/confirm';
	import type { Sesi, Course, Soal, Kelas } from '$lib/types';
	import RichTextEditor from '$lib/components/RichTextEditor.svelte';

	let sesiList = $state<Sesi[]>([]);
	let kelas = $state<Kelas[]>([]);
	let err = $state(''); let msg = $state('');

	// Search filters
	let sesiSearch = $state('');
	let soalSearch = $state('');

	let sesiForm = $state({ judul_sesi: '', deskripsi: '', urutan: 1, is_ujian_praktik: false });
	let editSesiId = $state<number | null>(null);

	let selectedSesi = $state<Sesi | null>(null);
	let courses = $state<Course[]>([]);
	let courseForm = $state({
		jenis: 'pretest', judul: '', deskripsi: '', durasi_menit: 20,
		kuota_easy: null as number | null, kuota_medium: null as number | null, kuota_hard: null as number | null
	});
	let editCourseId = $state<number | null>(null);

	let selectedCourse = $state<Course | null>(null);
	let soalList = $state<Soal[]>([]);
	let soalForm = $state({
		course_id: 0, jenis_soal: 'essay', difficulty: '' as string,
		kategori_ujian: '' as string, teks_soal: '', gambar_url: '' as string,
		poin: 20, kunci_jawaban: '' as string
	});
	let editSoalId = $state<number | null>(null);
	let showSoalModal = $state(false);

	async function loadSesi() {
		try { sesiList = (await api.get<Sesi[]>('/api/admin/sesi')) ?? []; }
		catch (e) { err = (e as Error).message; }
	}
	async function loadKelas() {
		try { kelas = (await api.get<Kelas[]>('/api/admin/kelas')) ?? []; }
		catch (e) { /* ignore */ }
	}
	let showScrollTop = $state(false);

	function scrollToTop() {
		window.scrollTo({ top: 0, behavior: 'smooth' });
	}

	onMount(() => {
		loadSesi();
		loadKelas();
		const handleScroll = () => {
			showScrollTop = window.scrollY > 300;
		};
		window.addEventListener('scroll', handleScroll);
		return () => window.removeEventListener('scroll', handleScroll);
	});

	function resetSesiForm() { editSesiId = null; sesiForm = { judul_sesi: '', deskripsi: '', urutan: 1, is_ujian_praktik: false }; }
	function editSesi(s: Sesi) {
		editSesiId = s.id;
		sesiForm = { judul_sesi: s.judul_sesi, deskripsi: s.deskripsi, urutan: s.urutan, is_ujian_praktik: s.is_ujian_praktik };
	}

	async function saveSesi() {
		err = ''; msg = '';
		try {
			if (editSesiId) await api.put(`/api/admin/sesi/${editSesiId}`, sesiForm);
			else await api.post('/api/admin/sesi', sesiForm);
			msg = 'Sesi tersimpan.'; resetSesiForm(); await loadSesi();
		} catch (e) { err = (e as Error).message; }
	}
	async function delSesi(id: number) {
		if (!await confirmAction({
			title: 'Hapus Sesi Praktikum?',
			message: 'Apakah Anda yakin ingin menghapus sesi ini? Tindakan ini juga akan `menghapus seluruh course dan soal` di dalamnya.'
		})) return;
		try { await api.del(`/api/admin/sesi/${id}`); if (selectedSesi?.id === id) { selectedSesi = null; courses = []; } await loadSesi(); }
		catch (e) { err = (e as Error).message; }
	}

	async function selectSesi(s: Sesi) {
		selectedSesi = s; selectedCourse = null; soalList = [];
		try { courses = (await api.get<Course[]>(`/api/admin/sesi/${s.id}/course`)) ?? []; }
		catch (e) { err = (e as Error).message; }
	}

	function resetCourseForm() {
		editCourseId = null;
		courseForm = { jenis: 'pretest', judul: '', deskripsi: '', durasi_menit: 20, kuota_easy: null, kuota_medium: null, kuota_hard: null };
	}
	function editCourse(c: Course) {
		editCourseId = c.id;
		courseForm = {
			jenis: c.jenis, judul: c.judul, deskripsi: c.deskripsi, durasi_menit: c.durasi_menit,
			kuota_easy: c.kuota_easy ?? null, kuota_medium: c.kuota_medium ?? null, kuota_hard: c.kuota_hard ?? null
		};
	}

	async function saveCourse() {
		if (!selectedSesi) return;
		err = ''; msg = '';
		try {
			if (editCourseId) await api.put(`/api/admin/course/${editCourseId}`, courseForm);
			else await api.post(`/api/admin/sesi/${selectedSesi.id}/course`, courseForm);
			msg = 'Course tersimpan.'; resetCourseForm(); await selectSesi(selectedSesi);
		} catch (e) { err = (e as Error).message; }
	}
	async function delCourse(id: number) {
		if (!await confirmAction({
			title: 'Hapus Course?',
			message: 'Apakah Anda yakin ingin menghapus course ini? Semua soal di dalamnya akan ikut terhapus.'
		})) return;
		try { await api.del(`/api/admin/course/${id}`); if (selectedCourse?.id === id) { selectedCourse = null; soalList = []; } if (selectedSesi) await selectSesi(selectedSesi); }
		catch (e) { err = (e as Error).message; }
	}

	async function selectCourse(c: Course) {
		selectedCourse = c;
		soalForm.course_id = c.id;
		try {
			soalList = (await api.get<Soal[]>(`/api/admin/soal?course_id=${c.id}`)) ?? [];
			setTimeout(() => {
				document.getElementById('pool-soal-section')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
			}, 80);
		}
		catch (e) { err = (e as Error).message; }
	}

	function resetSoalForm() {
		editSoalId = null;
		soalForm = { course_id: selectedCourse?.id ?? 0, jenis_soal: 'essay', difficulty: '', kategori_ujian: '', teks_soal: '', gambar_url: '', poin: 20, kunci_jawaban: '' };
	}
	// Saran poin sesuai rubrik AI grading (default project lama), berdasar jenis course + difficulty.
	function rubrikPoin(courseJenis: string, difficulty: string): number {
		if (courseJenis === 'keterampilan') return 85;       // SP 35 + BTE 30 + TW 20
		if (courseJenis === 'ujian_praktik') return 45;      // SP 25 + BTE 7 + TW 13
		if (courseJenis === 'pretest') return difficulty === 'medium' ? 15 : difficulty === 'hard' ? 25 : 20;
		if (courseJenis === 'posttest') return difficulty === 'medium' ? 35 : difficulty === 'hard' ? 45 : 20;
		return 20;
	}
	function suggestPoin() {
		soalForm.poin = rubrikPoin(selectedCourse?.jenis ?? '', soalForm.difficulty);
	}
	function openNewSoal() {
		resetSoalForm();
		suggestPoin(); // isi poin awal sesuai rubrik course
		showSoalModal = true;
	}
	function closeSoalModal() {
		showSoalModal = false;
		resetSoalForm();
	}
	function editSoal(s: Soal) {
		editSoalId = s.id;
		soalForm = {
			course_id: s.course_id, jenis_soal: s.jenis_soal,
			difficulty: s.difficulty ?? '', kategori_ujian: s.kategori_ujian ?? '',
			teks_soal: s.teks_soal, gambar_url: s.gambar_url ?? '',
			poin: s.poin, kunci_jawaban: s.kunci_jawaban ?? ''
		};
		showSoalModal = true;
	}

	async function uploadGambar(ev: Event) {
		const input = ev.target as HTMLInputElement;
		if (!input.files?.[0]) return;
		const fd = new FormData();
		fd.append('file', input.files[0]); fd.append('folder', 'flowchart');
		try { const res = await api.upload<{ url: string }>('/api/admin/upload', fd); soalForm.gambar_url = res.url; }
		catch (e) { err = (e as Error).message; }
	}

	async function saveSoal() {
		err = ''; msg = '';
		const body: Record<string, unknown> = {
			course_id: soalForm.course_id, jenis_soal: soalForm.jenis_soal,
			teks_soal: soalForm.teks_soal, poin: Number(soalForm.poin),
			difficulty: soalForm.difficulty || null,
			kategori_ujian: soalForm.kategori_ujian || null,
			gambar_url: soalForm.gambar_url || null,
			kunci_jawaban: soalForm.kunci_jawaban || null
		};
		try {
			if (editSoalId) await api.put(`/api/admin/soal/${editSoalId}`, body);
			else await api.post('/api/admin/soal', body);
			msg = 'Soal tersimpan.'; showSoalModal = false; resetSoalForm(); if (selectedCourse) await selectCourse(selectedCourse);
		} catch (e) { err = (e as Error).message; }
	}
	async function delSoal(id: number) {
		if (!await confirmAction({
			title: 'Hapus Soal?',
			message: 'Apakah Anda yakin ingin menghapus soal ini dari course?'
		})) return;
		try { await api.del(`/api/admin/soal/${id}`); if (selectedCourse) await selectCourse(selectedCourse); }
		catch (e) { err = (e as Error).message; }
	}
</script>

<h1 class="mb-4 text-2xl">Manajemen Sesi & Soal</h1>

{#if msg}<p class="mb-3 rounded-lg bg-state-success-bg p-3 text-sm text-state-success">{msg}</p>{/if}
{#if err}<p class="mb-3 rounded-lg bg-state-error-bg p-3 text-sm text-state-error">{err}</p>{/if}

<div class="grid gap-4 lg:grid-cols-3">
	<div class="card">
		<h2 class="mb-3 text-lg">{editSesiId ? 'Edit' : 'Tambah'} Sesi</h2>
		<label class="label" for="sj">Judul Sesi</label>
		<input id="sj" class="input" bind:value={sesiForm.judul_sesi} placeholder="mis. Modul 1 - Pengenalan Dasar" />
		<label class="label mt-2" for="sd">Deskripsi</label>
		<textarea id="sd" class="input min-h-16" bind:value={sesiForm.deskripsi}></textarea>
		<label class="label mt-2" for="su">Urutan</label>
		<input id="su" type="number" class="input" bind:value={sesiForm.urutan} min="1" />
		<label class="mt-2 flex items-center gap-2 text-sm">
			<input type="checkbox" bind:checked={sesiForm.is_ujian_praktik} /> Sesi Ujian Praktik
		</label>
		<div class="mt-3 flex gap-2">
			<button class="btn-primary" onclick={saveSesi}>Simpan</button>
			{#if editSesiId}<button class="btn-outline" onclick={resetSesiForm}>Batal</button>{/if}
		</div>
	</div>

	<div class="lg:col-span-2">
		<div class="mb-3">
			<div class="relative max-w-xs">
				<Search size={14} class="absolute left-3 top-1/2 -translate-y-1/2 text-ink-caption" />
				<input type="text" placeholder="Cari sesi..." bind:value={sesiSearch} class="input pl-9 w-full text-sm" />
			</div>
		</div>
		<div class="table-wrap">
			<table class="tbl">
				<thead><tr><th>#</th><th>Judul</th><th>Tipe</th><th>Aksi</th></tr></thead>
				<tbody>
					{#each sesiList.filter((s) => sesiSearch === '' || s.judul_sesi.toLowerCase().includes(sesiSearch.toLowerCase())) as s}
						<tr class={selectedSesi?.id === s.id ? 'ring-2 ring-primary' : ''}>
							<td>{s.urutan}</td>
							<td>{s.judul_sesi}</td>
							<td>{s.is_ujian_praktik ? 'Ujian Praktik' : 'Modul'}</td>
							<td class="flex items-center gap-1.5 whitespace-nowrap">
								<button class="inline-flex items-center gap-1 bg-blue-50 hover:bg-blue-650 hover:text-white text-blue-600 px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95 border border-blue-100" onclick={() => selectSesi(s)}>
									<Eye size={12} /> Pilih
								</button>
								<button class="inline-flex items-center gap-1 bg-primary/10 hover:bg-primary hover:text-white text-primary px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95" onclick={() => editSesi(s)}>
									<Edit size={12} /> Edit
								</button>
								<button class="inline-flex items-center gap-1 bg-red-50 hover:bg-red-650 hover:text-white text-red-650 px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95 border border-red-100" onclick={() => delSesi(s.id)}>
									<Trash2 size={12} /> Hapus
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>
</div>

{#if selectedSesi}
	<hr class="my-6 border-gray-200" />
	<h2 class="mb-3 text-xl">Course — {selectedSesi.judul_sesi}</h2>

	<div class="grid gap-4 lg:grid-cols-3">
		<div class="card">
			<h3 class="mb-3 text-lg">{editCourseId ? 'Edit' : 'Tambah'} Course</h3>
			<label class="label" for="cj">Jenis</label>
			<select id="cj" class="input" bind:value={courseForm.jenis}>
				<option value="pretest">Pre-test</option>
				<option value="posttest">Post-test</option>
				<option value="keterampilan">Keterampilan</option>
				<option value="ujian_praktik">Ujian Praktik</option>
			</select>
			<label class="label mt-2" for="ct">Judul</label>
			<input id="ct" class="input" bind:value={courseForm.judul} />
			<label class="label mt-2" for="cd">Deskripsi</label>
			<textarea id="cd" class="input min-h-16" bind:value={courseForm.deskripsi}></textarea>
			<label class="label mt-2" for="cm">Durasi (menit)</label>
			<input id="cm" type="number" class="input" bind:value={courseForm.durasi_menit} min="1" />
			{#if courseForm.jenis === 'pretest' || courseForm.jenis === 'posttest'}
				<p class="label mt-2">Kuota soal acak per difficulty</p>
				<p class="mb-1 text-xs text-ink-caption">Jumlah soal yang diambil acak dari pool. Kosongkan = default ({courseForm.jenis === 'pretest' ? '1/2/2' : '1/1/1'}).</p>
				<div class="grid grid-cols-3 gap-2">
					<div>
						<span class="text-xs text-ink-caption">Easy</span>
						<input type="number" class="input" bind:value={courseForm.kuota_easy} min="0" placeholder="–" />
					</div>
					<div>
						<span class="text-xs text-ink-caption">Medium</span>
						<input type="number" class="input" bind:value={courseForm.kuota_medium} min="0" placeholder="–" />
					</div>
					<div>
						<span class="text-xs text-ink-caption">Hard</span>
						<input type="number" class="input" bind:value={courseForm.kuota_hard} min="0" placeholder="–" />
					</div>
				</div>
			{/if}
			<div class="mt-3 flex gap-2">
				<button class="btn-primary" onclick={saveCourse}>Simpan</button>
				{#if editCourseId}<button class="btn-outline" onclick={resetCourseForm}>Batal</button>{/if}
			</div>
		</div>

		<div class="lg:col-span-2">
			<div class="table-wrap">
				<table class="tbl">
					<thead><tr><th>Jenis</th><th>Judul</th><th>Durasi</th><th>Aksi</th></tr></thead>
					<tbody>
						{#each courses as c}
							<tr class={selectedCourse?.id === c.id ? 'ring-2 ring-primary' : ''}>
								<td>{labelJenis(c.jenis)}</td>
								<td>{c.judul}</td>
								<td>{c.durasi_menit} min</td>
								<td class="flex items-center gap-1.5 whitespace-nowrap">
									<button class="inline-flex items-center gap-1 bg-blue-50 hover:bg-blue-650 hover:text-white text-blue-600 px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95 border border-blue-100" onclick={() => selectCourse(c)}>
										<Eye size={12} /> Soal
									</button>
									<button class="inline-flex items-center gap-1 bg-primary/10 hover:bg-primary hover:text-white text-primary px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95" onclick={() => editCourse(c)}>
										<Edit size={12} /> Edit
									</button>
									<button class="inline-flex items-center gap-1 bg-red-50 hover:bg-red-650 hover:text-white text-red-650 px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95 border border-red-100" onclick={() => delCourse(c.id)}>
										<Trash2 size={12} /> Hapus
									</button>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>
	</div>
{/if}

{#if selectedCourse}
	<div id="pool-soal-section" class="scroll-mt-20">
		<hr class="my-6 border-gray-200" />
	<div class="mb-3 flex flex-wrap items-center justify-between gap-3">
		<h2 class="text-xl">Pool Soal — {labelJenis(selectedCourse.jenis)} ({soalList.length} soal)</h2>
		<div class="flex items-center gap-2">
			<div class="relative">
				<Search size={14} class="absolute left-3 top-1/2 -translate-y-1/2 text-ink-caption" />
				<input type="text" placeholder="Cari soal..." bind:value={soalSearch} class="input pl-9 w-48 text-sm" />
			</div>
			<button class="btn-primary" onclick={openNewSoal}>+ Tambah Soal</button>
		</div>
	</div>

	{#if soalList.length === 0}
		<div class="card text-center text-ink-caption">
			Belum ada soal. Klik <span class="font-semibold text-primary">Tambah Soal</span> untuk membuat soal pertama.
		</div>
	{:else}
		<div class="space-y-3">
			{#each soalList.filter((s) => soalSearch === '' || s.teks_soal.toLowerCase().includes(soalSearch.toLowerCase())) as s, i}
				<div class="card">
					<div class="flex items-start justify-between gap-3">
						<div class="flex-1">
							<div class="flex flex-wrap items-center gap-2 text-sm">
								<span class="badge bg-surface-soft text-ink-heading">#{i + 1}</span>
								<span class="badge bg-surface-soft text-ink-body">{s.jenis_soal}</span>
								{#if s.difficulty}<span class="badge bg-surface-soft text-ink-caption">{s.difficulty}</span>{/if}
								{#if s.kategori_ujian}<span class="badge bg-state-info-bg text-state-info">{s.kategori_ujian}</span>{/if}
								<span class="text-ink-caption">{s.poin} poin</span>
							</div>
							<div class="prose prose-sm mt-2 max-w-none text-ink-body" use:renderMath>
								{@html sanitizeSoal(s.teks_soal)}
							</div>
							{#if s.gambar_url}<img src={s.gambar_url} alt="flowchart" class="mt-2 max-h-32 rounded-lg border" />{/if}
						</div>
						<div class="flex items-center gap-1.5 whitespace-nowrap">
							<button class="inline-flex items-center gap-1 bg-primary/10 hover:bg-primary hover:text-white text-primary px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95" onclick={() => editSoal(s)}>
								<Edit size={12} /> Edit
							</button>
							<button class="inline-flex items-center gap-1 bg-red-50 hover:bg-red-650 hover:text-white text-red-650 px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95 border border-red-100" onclick={() => delSoal(s.id)}>
								<Trash2 size={12} /> Hapus
							</button>
						</div>
				</div>
			</div>
		{/each}
		</div>
	{/if}
	</div>
{/if}

<!-- Modal Tambah/Edit Soal -->
{#if showSoalModal && selectedCourse}
	<div
		class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-black/50 p-4 backdrop-blur-sm sm:items-center"
		role="presentation"
		onclick={(e) => { if (e.target === e.currentTarget) closeSoalModal(); }}
	>
		<div class="my-8 w-full max-w-4xl rounded-2xl bg-white shadow-2xl" role="dialog" aria-modal="true">
			<div class="flex items-center justify-between border-b border-gray-200 px-6 py-4">
				<h3 class="text-lg font-bold text-ink-heading">{editSoalId ? 'Edit' : 'Tambah'} Soal</h3>
				<button class="text-ink-caption hover:text-ink-heading" aria-label="Tutup" onclick={closeSoalModal}><X size={20} /></button>
			</div>
			<div class="max-h-[85vh] overflow-y-auto px-6 py-4">
				<label class="label" for="sj2">Jenis Soal</label>
				<select id="sj2" class="input" bind:value={soalForm.jenis_soal}>
					<option value="essay">Essay</option>
					<option value="coding">Coding</option>
				</select>
				<label class="label mt-2" for="sd2">Difficulty</label>
				<select id="sd2" class="input" bind:value={soalForm.difficulty} onchange={suggestPoin}>
					<option value="">— Tidak ada —</option>
					<option value="easy">Easy</option>
					<option value="medium">Medium</option>
					<option value="hard">Hard</option>
				</select>
				{#if selectedCourse.jenis === 'ujian_praktik'}
					<label class="label mt-2" for="ku">Kategori Ujian</label>
					<select id="ku" class="input" bind:value={soalForm.kategori_ujian}>
						<option value="">— Pilih —</option>
						<option value="modul_1">Modul 1</option>
						<option value="modul_2">Modul 2</option>
						<option value="modul_3">Modul 3</option>
						<option value="modul_4_5">Modul 4 dan 5</option>
						<option value="modul_6">Modul 6</option>
						<option value="flowchart">Flowchart</option>
					</select>
				{/if}
				<label class="label mt-2" for="ts">Teks Soal</label>
				<div class="mt-1">
					<RichTextEditor bind:value={soalForm.teks_soal} placeholder="Tulis soal di sini..." />
				</div>
				{#if soalForm.kategori_ujian === 'flowchart' || soalForm.gambar_url}
					<label class="label mt-2" for="gu">Gambar Flowchart</label>
					{#if soalForm.gambar_url}<img src={soalForm.gambar_url} alt="flowchart" class="mb-2 max-h-40 rounded-lg border" />{/if}
					<input id="gu" type="file" accept="image/*" onchange={uploadGambar} />
				{/if}
				<label class="label mt-2" for="sp">Poin</label>
				<input id="sp" type="number" class="input" bind:value={soalForm.poin} min="0" />
				<p class="mt-1 text-xs text-ink-caption">Saran rubrik: {rubrikPoin(selectedCourse.jenis, soalForm.difficulty)} (otomatis terisi; boleh diubah).</p>
				<label class="label mt-2" for="kj">Kunci Jawaban (opsional)</label>
				<div class="mt-1">
					<RichTextEditor bind:value={soalForm.kunci_jawaban} placeholder="Tulis referensi atau rubrik jawaban..." />
				</div>
			</div>
			<div class="flex justify-end gap-2 border-t border-gray-200 px-6 py-4">
				<button class="btn-outline" onclick={closeSoalModal}>Batal</button>
				<button class="btn-primary" onclick={saveSoal}>Simpan</button>
			</div>
		</div>
	</div>
{/if}

{#if showScrollTop}
	<button
		onclick={scrollToTop}
		class="fixed bottom-6 right-6 z-50 flex h-10 w-10 items-center justify-center rounded-full bg-primary text-white shadow-lg transition-all hover:bg-primary-hover hover:-translate-y-0.5 focus:outline-none"
		aria-label="Kembali ke atas"
	>
		<ArrowUp size={18} />
	</button>
{/if}
