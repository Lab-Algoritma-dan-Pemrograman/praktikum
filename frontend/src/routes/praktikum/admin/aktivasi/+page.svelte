<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { labelJenis, labelShift } from '$lib/utils';
	import { KeyRound, Trash2, Search, Zap, ArrowUp, Eye } from 'lucide-svelte';
	import { confirmAction } from '$lib/stores/confirm';
	import type { Sesi, Kelas, User } from '$lib/types';

	interface AktivasiSesi {
		id: number; sesi_praktikum_id: number; kelas_id: number; shift: number;
		gelombang?: number | null;
		is_active: boolean; activated_at: string; token?: string;
		sesi?: { judul_sesi: string };
		kelas?: { nama_kelas: string };
		aktivasi_courses?: AktivasiCourse[];
	}
	interface AktivasiCourse {
		id: number; course_id: number; is_open: boolean; urutan: number;
		course?: { jenis: string; judul: string };
	}
	interface Susulan { id: number; mahasiswa_id: number; alasan: string; mahasiswa?: { nama: string; nim: string } }
	interface Peserta {
		mahasiswa_id: number; nim: string; nama: string;
		course_id: number; judul_course: string; status: string;
		waktu_mulai: string | null; waktu_selesai: string | null;
	}

	let aktivasiList = $state<AktivasiSesi[]>([]);
	let sesiList = $state<Sesi[]>([]);
	let kelasList = $state<Kelas[]>([]);
	let users = $state<User[]>([]);
	let err = $state(''); let msg = $state('');

	// Search & Filter
	let searchSesi = $state('');
	let searchKelas = $state('');

	let filteredAktivasi = $derived(
		aktivasiList.filter((a) => {
			const querySesi = searchSesi.toLowerCase().trim();
			const queryKelas = searchKelas.toLowerCase().trim();
			const matchSesi = querySesi === '' || (a.sesi?.judul_sesi ?? '').toLowerCase().includes(querySesi);
			const matchKelas = queryKelas === '' || (a.kelas?.nama_kelas ?? '').toLowerCase().includes(queryKelas);
			return matchSesi && matchKelas;
		})
	);

	let form = $state({ sesi_praktikum_id: 0, kelas_id: 0, shift: 1, gelombang: null as number | null, gacha_pilihan: 'pretest' });
	// Gelombang hanya relevan untuk sesi ujian praktik.
	let isUjianPraktik = $derived(sesiList.find((s) => s.id === form.sesi_praktikum_id)?.is_ujian_praktik ?? false);

	let selected = $state<AktivasiSesi | null>(null);
	let susulanList = $state<Susulan[]>([]);
	let susulanForm = $state({ mahasiswa_id: 0, alasan: '' });
	let pesertaList = $state<Peserta[]>([]);

	let showScrollTop = $state(false);

	function scrollToTop() {
		window.scrollTo({ top: 0, behavior: 'smooth' });
	}

	async function load() {
		try {
			aktivasiList = (await api.get<AktivasiSesi[]>('/api/admin/aktivasi')) ?? [];
			sesiList = (await api.get<Sesi[]>('/api/admin/sesi')) ?? [];
			kelasList = (await api.get<Kelas[]>('/api/admin/kelas')) ?? [];
			users = (await api.get<User[]>('/api/admin/users')) ?? [];
			
			if (sesiList.length > 0 && form.sesi_praktikum_id === 0) form.sesi_praktikum_id = sesiList[0].id;
			if (kelasList.length > 0 && form.kelas_id === 0) form.kelas_id = kelasList[0].id;
		} catch (e) { err = (e as Error).message; }
	}
	onMount(() => {
		load();
		const handleScroll = () => {
			showScrollTop = window.scrollY > 300;
		};
		window.addEventListener('scroll', handleScroll);
		return () => window.removeEventListener('scroll', handleScroll);
	});

	async function aktivasi() {
		err = ''; msg = '';
		try {
			await api.post('/api/admin/aktivasi', form);
			msg = 'Sesi berhasil diaktifkan.'; await load();
		} catch (e) { err = (e as Error).message; }
	}

	async function selectAktivasi(a: AktivasiSesi) {
		selected = a;
		try {
			const detail = await api.get<AktivasiSesi>(`/api/admin/aktivasi/${a.id}`);
			if (detail) selected = detail;
			susulanList = (await api.get<Susulan[]>(`/api/admin/aktivasi/${a.id}/susulan`)) ?? [];
			await loadPeserta(a.id);
			
			// Smooth scroll to the results page/section
			setTimeout(() => {
				document.getElementById('detail-session')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
			}, 80);
		} catch (e) { err = (e as Error).message; }
	}

	async function loadPeserta(aktivasiId: number) {
		try { pesertaList = (await api.get<Peserta[]>(`/api/admin/aktivasi/${aktivasiId}/peserta`)) ?? []; }
		catch (e) { err = (e as Error).message; }
	}

	function statusBadge(s: string): { label: string; cls: string } {
		if (s === 'sedang_dikerjakan') return { label: 'Sedang mengerjakan', cls: 'bg-state-warning-bg text-state-warning' };
		if (s === 'selesai') return { label: 'Selesai', cls: 'bg-state-success-bg text-state-success' };
		return { label: 'Belum', cls: 'bg-gray-100 text-ink-caption' };
	}

	async function toggleCourse(ac: AktivasiCourse) {
		err = ''; msg = '';
		const action = ac.is_open ? 'MENUTUP (auto-submit massal)' : 'MEMBUKA';
		if (!await confirmAction({
			title: `${ac.is_open ? 'Tutup' : 'Buka'} Course?`,
			message: `Apakah Anda yakin ingin ${action.toLowerCase()} course ini?`
		})) return;
		try {
			await api.post('/api/admin/aktivasi-course/buka-tutup', {
				aktivasi_course_id: ac.id,
				is_open: !ac.is_open
			});
			msg = ac.is_open ? 'Course ditutup. Auto-submit massal dijalankan.' : 'Course dibuka.';
			if (selected) await selectAktivasi(selected);
		} catch (e) { err = (e as Error).message; }
	}

	async function unlockCourse(ac: AktivasiCourse) {
		err = ''; msg = '';
		if (!await confirmAction({
			title: 'Buka Kunci Peserta?',
			message: 'Seluruh praktikan pada course ini direset ke "belum dikerjakan" supaya bisa mengetik dan mengerjakan lagi. Isi jawaban mereka TIDAK dihapus, dan timer dihitung ulang dari praktikan pertama yang mulai.'
		})) return;
		try {
			await api.post('/api/admin/aktivasi-course/buka-tutup', {
				aktivasi_course_id: ac.id,
				unlock_only: true
			});
			msg = 'Kunci peserta dilepas. Praktikan bisa mengerjakan dan mengetik lagi.';
			if (selected) await selectAktivasi(selected);
		} catch (e) { err = (e as Error).message; }
	}

	async function addSusulan() {
		if (!selected) return;
		err = ''; msg = '';
		try {
			await api.post(`/api/admin/aktivasi/${selected.id}/susulan`, susulanForm);
			msg = 'Mahasiswa susulan didaftarkan.';
			susulanForm = { mahasiswa_id: 0, alasan: '' };
			await selectAktivasi(selected);
		} catch (e) { err = (e as Error).message; }
	}

	async function removeSusulan(mhsId: number) {
		if (!selected || !await confirmAction({
			title: 'Hapus Akses Susulan?',
			message: 'Apakah Anda yakin ingin menghapus hak akses susulan mahasiswa ini?'
		})) return;
		try {
			await api.del(`/api/admin/aktivasi/${selected.id}/susulan/${mhsId}`);
			await selectAktivasi(selected);
		} catch (e) { err = (e as Error).message; }
	}

	async function generateToken(a: AktivasiSesi) {
		err = ''; msg = '';
		if (!await confirmAction({
			title: 'Reset PIN Ujian Kelas?',
			message: 'Apakah Anda yakin ingin mereset/generate ulang PIN ujian kelas ini? Praktikan yang belum masuk harus menggunakan PIN baru.'
		})) return;
		try {
			await api.post(`/api/admin/aktivasi/${a.id}/token`);
			msg = 'PIN Ujian berhasil dibuat/direset.';
			await load();
			if (selected && selected.id === a.id) {
				await selectAktivasi(selected);
			}
		} catch (e) { err = (e as Error).message; }
	}

	async function hapusAktivasi(a: AktivasiSesi) {
		err = ''; msg = '';
		const name = `${a.sesi?.judul_sesi ?? a.sesi_praktikum_id} — ${a.kelas?.nama_kelas ?? a.kelas_id} ${labelShift(a.shift)}`;
		if (!await confirmAction({
			title: 'Hapus Aktivasi Sesi Permanen?',
			message: `PERINGATAN: Aksi ini akan \`menghapus permanen\` seluruh jawaban, nilai, susulan, dan log praktikan pada aktivasi \`${name}\`. Data yang dihapus tidak dapat dikembalikan. Yakin melanjutkan?`
		})) return;
		try {
			await api.del(`/api/admin/aktivasi/${a.id}`);
			msg = `Aktivasi "${name}" beserta seluruh data terkait dihapus.`;
			if (selected?.id === a.id) selected = null;
			await load();
		} catch (e) { err = (e as Error).message; }
	}
</script>

<div class="space-y-6 text-left">
	<div class="flex items-center gap-3">
		<div class="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10 text-primary">
			<Zap size={20} />
		</div>
		<div>
			<h1 class="text-2xl font-bold text-slate-900 leading-none">Aktivasi Sesi</h1>
			<p class="mt-2 text-sm text-ink-caption">Aktifkan modul praktikum atau ujian untuk kelas dan shift tertentu.</p>
		</div>
	</div>

	{#if msg}<p class="rounded-lg bg-state-success-bg p-3 text-sm text-state-success">{msg}</p>{/if}
	{#if err}<p class="rounded-lg bg-state-error-bg p-3 text-sm text-state-error">{err}</p>{/if}

	<div class="grid gap-6 lg:grid-cols-3">
		<!-- Form Aktifkan -->
		<div class="card">
			<h2 class="mb-4 text-lg font-bold text-slate-800">Aktifkan Sesi</h2>
			<div class="space-y-3">
				<div>
					<label class="label font-semibold text-slate-650" for="as">Sesi</label>
					<select id="as" class="input mt-1 w-full bg-white border border-slate-200" bind:value={form.sesi_praktikum_id}>
						<option value={0}>— Pilih Sesi —</option>
						{#each sesiList as s}<option value={s.id}>{s.judul_sesi}</option>{/each}
					</select>
				</div>
				<div>
					<label class="label font-semibold text-slate-650" for="ak">Kelas</label>
					<select id="ak" class="input mt-1 w-full bg-white border border-slate-200" bind:value={form.kelas_id}>
						<option value={0}>— Pilih Kelas —</option>
						{#each kelasList as k}<option value={k.id}>{k.nama_kelas}</option>{/each}
					</select>
				</div>
				<div>
					<label class="label font-semibold text-slate-650" for="ash">Shift</label>
					<select id="ash" class="input mt-1 w-full bg-white border border-slate-200" bind:value={form.shift}>
						<option value={1}>Shift 1</option>
						<option value={2}>Shift 2</option>
					</select>
				</div>
				{#if isUjianPraktik}
					<div>
						<label class="label font-semibold text-slate-650" for="agel">Gelombang</label>
						<select id="agel" class="input mt-1 w-full bg-white border border-slate-200" bind:value={form.gelombang}>
							<option value={null}>— Tanpa gelombang —</option>
							<option value={1}>Gelombang 1</option>
							<option value={2}>Gelombang 2</option>
						</select>
						<p class="mt-1 text-[10px] text-ink-caption leading-relaxed">Aktifkan terpisah per gelombang. Mahasiswa hanya bisa akses gelombangnya sendiri.</p>
					</div>
				{/if}
				<div>
					<label class="label font-semibold text-slate-650" for="ag">Gacha (Pre/Post-test)</label>
					<select id="ag" class="input mt-1 w-full bg-white border border-slate-200" bind:value={form.gacha_pilihan}>
						<option value="pretest">Pre-test</option>
						<option value="posttest">Post-test</option>
					</select>
					<p class="mt-1 text-[10px] text-ink-caption">Diabaikan untuk sesi ujian praktik.</p>
				</div>
				<button class="btn-primary mt-4 w-full" onclick={aktivasi}>Aktifkan Sesi</button>
			</div>
		</div>

		<!-- Table List with Filters -->
		<div class="lg:col-span-2 space-y-4">
			<div class="card space-y-3.5">
				<h2 class="text-lg font-bold text-slate-800">Daftar Aktivasi Berjalan</h2>
				
				<!-- Filter Inputs -->
				<div class="flex flex-col gap-3 sm:flex-row sm:items-center">
					<div class="relative flex-1">
						<Search size={14} class="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
						<input type="text" placeholder="Filter judul sesi..." bind:value={searchSesi} class="input pl-9 w-full text-xs" />
					</div>
					<div class="relative flex-1">
						<Search size={14} class="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
						<input type="text" placeholder="Filter nama kelas..." bind:value={searchKelas} class="input pl-9 w-full text-xs" />
					</div>
				</div>

				<div class="table-wrap">
					<table class="tbl w-full">
						<thead>
							<tr><th>Sesi</th><th>Kelas</th><th>Shift</th><th>Gel.</th><th>Aksi</th></tr>
						</thead>
						<tbody>
							{#each filteredAktivasi as a}
								<tr class={selected?.id === a.id ? 'ring-2 ring-primary' : ''}>
									<td class="font-bold text-slate-850">{a.sesi?.judul_sesi ?? a.sesi_praktikum_id}</td>
									<td><span class="badge bg-slate-50 border border-slate-200 font-bold">{a.kelas?.nama_kelas ?? a.kelas_id}</span></td>
									<td><span class="badge bg-slate-100 border border-slate-200">{a.shift === 0 ? 'Arsip' : `Shift ${a.shift}`}</span></td>
									<td><span class="badge bg-slate-50 border border-slate-150">{a.gelombang ?? '-'}</span></td>
									<td class="flex items-center gap-1.5 whitespace-nowrap">
										<button class="inline-flex items-center gap-1 bg-primary/10 hover:bg-primary hover:text-white text-primary px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95" onclick={() => selectAktivasi(a)}>
											<Eye size={12} /> Detail
										</button>
										<button class="inline-flex items-center gap-1 bg-red-50 hover:bg-red-650 hover:text-white text-red-650 px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95 border border-red-100" onclick={() => hapusAktivasi(a)}>
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
	</div>

	<!-- Detail Section -->
	{#if selected}
		<div id="detail-session" class="card w-full border-t-4 border-t-primary scroll-mt-20 space-y-6">
			<div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-100 pb-4">
				<div>
					<h2 class="text-xl font-bold text-slate-900">
						{selected.sesi?.judul_sesi}
					</h2>
					<p class="mt-1 text-sm text-slate-500 font-semibold">
						Kelas: {selected.kelas?.nama_kelas} · {labelShift(selected.shift)}
					</p>
				</div>
				<div class="flex items-center gap-3">
					{#if selected.token}
						<div class="bg-primary/10 border-2 border-primary/20 text-primary px-4 py-1.5 rounded-lg font-mono text-xl font-bold tracking-widest shadow-sm">
							{selected.token}
						</div>
					{/if}
					<button class="btn-primary" onclick={() => generateToken(selected!)}>
						<KeyRound size={16} /> {selected.token ? 'Reset PIN' : 'Generate PIN'}
					</button>
				</div>
			</div>

			<!-- Buka/Tutup Courses -->
			<div>
				<h3 class="text-base font-bold text-slate-800 mb-1.5">Buka / Tutup Ujian/Course</h3>
				<p class="text-xs text-slate-500 mb-4 leading-relaxed">Membuka atau menutup course praktikan. Menutup course akan men-submit otomatis seluruh praktikan yang belum submit. Pakai <strong>Buka Kunci Peserta</strong> kalau praktikan sudah terlanjur berstatus selesai padahal masih mau mengerjakan (course tetap terbuka, isi jawaban tidak dihapus).</p>
				<div class="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
					{#each selected.aktivasi_courses ?? [] as ac}
						<div class="card bg-slate-50/60 border border-slate-200/80 flex items-center justify-between gap-3 p-4 shadow-sm">
							<div>
								<p class="font-bold text-slate-800 text-sm">{ac.course?.judul ?? labelJenis(ac.course?.jenis ?? '')}</p>
								<span class="badge mt-1.5 inline-block {ac.is_open ? 'bg-state-success-bg text-state-success' : 'bg-slate-100 border-slate-200 text-ink-caption'}">
									{ac.is_open ? 'Terbuka' : 'Tertutup'}
								</span>
							</div>
							<div class="flex flex-col items-end gap-2">
								<button
									class="btn-outline py-1.5 text-xs font-bold {ac.is_open ? 'text-state-error hover:bg-state-error-bg border-state-error/30' : 'text-state-success hover:bg-state-success-bg border-state-success/30'}"
									onclick={() => toggleCourse(ac)}
								>{ac.is_open ? 'Tutup' : 'Buka'}</button>
								<button
									class="btn-outline py-1.5 text-xs font-bold text-primary border-primary/30 hover:bg-primary hover:text-white"
									onclick={() => unlockCourse(ac)}
								>Buka Kunci Peserta</button>
							</div>
						</div>
					{/each}
				</div>
			</div>

			<!-- Live Students Monitor -->
			<div class="border-t border-slate-150 pt-5">
				<div class="mb-3 flex items-center justify-between">
					<div>
						<h3 class="text-base font-bold text-slate-800 leading-none">Status & Progress Praktikan</h3>
						<p class="mt-1.5 text-xs text-slate-500">
							Daftar mahasiswa yang sedang atau sudah menyelesaikan praktikum pada shift ini.
							{#if pesertaList.length > 0}
								(Sedang: <strong class="text-state-warning">{pesertaList.filter(p => p.status === 'sedang_dikerjakan').length}</strong> · Selesai: <strong class="text-state-success">{pesertaList.filter(p => p.status === 'selesai').length}</strong>)
							{/if}
						</p>
					</div>
					<button class="btn-outline py-1.5 text-xs font-bold" onclick={() => selected && loadPeserta(selected.id)}>Refresh Monitor</button>
				</div>
				
				{#if pesertaList.length === 0}
					<p class="text-sm text-slate-400 py-4 font-semibold text-center bg-slate-50 border border-slate-150 rounded-xl">Belum ada praktikan yang bergabung ke ruang pengerjaan.</p>
				{:else}
					<div class="overflow-x-auto border border-slate-200 rounded-xl bg-white">
						<table class="w-full text-sm">
							<thead>
								<tr class="text-left text-slate-400 bg-slate-50/70 border-b border-slate-200"><th class="py-2.5 px-4 font-bold text-xs uppercase tracking-wider">Nama</th><th class="py-2.5 px-4 font-bold text-xs uppercase tracking-wider">NIM</th><th class="py-2.5 px-4 font-bold text-xs uppercase tracking-wider">Course</th><th class="py-2.5 px-4 font-bold text-xs uppercase tracking-wider">Status</th><th class="py-2.5 px-4 font-bold text-xs uppercase tracking-wider">Mulai</th></tr>
							</thead>
							<tbody>
								{#each pesertaList as p}
									<tr class="border-t border-slate-100 hover:bg-slate-50/50">
										<td class="py-2 px-4 font-semibold text-slate-700">{p.nama}</td>
										<td class="font-mono text-xs px-4 font-bold text-slate-500">{p.nim}</td>
										<td class="px-4 font-medium text-slate-700">{p.judul_course}</td>
										<td class="px-4"><span class="badge {statusBadge(p.status).cls} font-semibold">{statusBadge(p.status).label}</span></td>
										<td class="text-xs text-ink-caption px-4 font-medium">{p.waktu_mulai ? new Date(p.waktu_mulai).toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' }) : '—'}</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
			</div>

			<!-- Susulan Section -->
			<div class="border-t border-slate-150 pt-5 space-y-4">
				<div>
					<h3 class="text-base font-bold text-slate-900 leading-none">Peserta Susulan</h3>
					<p class="mt-1.5 text-xs text-slate-500">Berikan akses masuk ujian susulan untuk mahasiswa yang tidak mengikuti jadwal aslinya.</p>
				</div>
				
				<div class="grid gap-6 lg:grid-cols-3">
					<div class="card bg-slate-50/50 border border-slate-200">
						<h4 class="font-bold text-slate-800 text-sm mb-3">Daftarkan Susulan</h4>
						<div class="space-y-3">
							<div>
								<label class="label font-semibold text-slate-600 text-xs" for="sm">Mahasiswa</label>
								<select id="sm" class="input mt-1 w-full bg-white border border-slate-250 text-xs" bind:value={susulanForm.mahasiswa_id}>
									<option value={0}>— Pilih Mahasiswa —</option>
									{#each users as u}<option value={u.id}>{u.nama} ({u.nim})</option>{/each}
								</select>
							</div>
							<div>
								<label class="label font-semibold text-slate-600 text-xs" for="sa">Alasan</label>
								<input id="sa" class="input mt-1 w-full text-xs" bind:value={susulanForm.alasan} placeholder="Ketik alasan susulan..." />
							</div>
							<button class="btn-primary w-full py-2 text-xs font-bold" onclick={addSusulan}>Tambah Susulan</button>
						</div>
					</div>
					
					<div class="lg:col-span-2">
						{#if susulanList.length === 0}
							<div class="h-full flex items-center justify-center text-sm text-slate-400 font-semibold py-8 border border-dashed border-slate-350 rounded-2xl bg-slate-50/50">
								Tidak ada peserta susulan terdaftar.
							</div>
						{:else}
							<div class="table-wrap">
								<table class="tbl w-full">
									<thead><tr><th>Nama</th><th>NIM</th><th>Alasan</th><th>Aksi</th></tr></thead>
									<tbody>
										{#each susulanList as s}
											<tr>
												<td class="font-bold text-slate-800">{s.mahasiswa?.nama ?? s.mahasiswa_id}</td>
												<td class="font-mono text-xs font-semibold text-slate-500">{s.mahasiswa?.nim ?? '-'}</td>
												<td class="text-slate-600 font-medium">{s.alasan}</td>
												<td>
													<button class="text-state-error font-bold hover:underline flex items-center gap-1" onclick={() => removeSusulan(s.mahasiswa_id)}>
														Hapus
													</button>
												</td>
											</tr>
										{/each}
									</tbody>
								</table>
							</div>
						{/if}
					</div>
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
</div>
