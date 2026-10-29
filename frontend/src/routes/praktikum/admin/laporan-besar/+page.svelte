<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { Eye, Download, Check, Unlock, Lock, X, Save, Trash2 } from 'lucide-svelte';
	import { confirmAction } from '$lib/stores/confirm';
	import type { Kelas } from '$lib/types';

	interface Target {
		id?: number;
		kelas_id: number;
		buka_at: string | null;
		deadline_at: string | null;
		late_until: string | null;
		kelas?: { id: number; nama_kelas: string };
	}

	interface LaporanBesar {
		id: number;
		judul: string;
		deskripsi: string;
		is_aktif: boolean;
		butuh_laporan: boolean;
		butuh_analisa: boolean;
		targets: Target[];
	}

	interface Pengumpulan {
		mahasiswa_id: number;
		nim: string;
		nama: string;
		kelas_id: number;
		nama_kelas: string;
		status: 'belum' | 'terkumpul';
		sumber: 'web' | 'manual' | null;
		submitted_at: string | null;
		is_late: boolean;
		manual_catatan: string | null;
		override_alasan: string | null;
		override_oleh: string | null;
		laporan_url: string | null;
		analisa_url: string | null;
	}

	let lb = $state<LaporanBesar | null>(null);
	let kelas = $state<Kelas[]>([]);
	let pengumpulan = $state<Pengumpulan[]>([]);
	let err = $state('');
	let msg = $state('');
	let loading = $state(true);
	let savingForm = $state(false);
	let savingTarget = $state(false);

	let form = $state({
		judul: 'Laporan Besar',
		deskripsi: '',
		is_aktif: true,
		butuh_laporan: true,
		butuh_analisa: false
	});

	// Target kelas yang sedang diedit (draft lokal sebelum disimpan ke server).
	let targets = $state<Target[]>([]);
	let addKelasId = $state<number | null>(null);

	// Filter & seleksi tabel pengumpulan.
	let filterKelas = $state<number | 'all'>('all');
	let selected = $state<Record<number, boolean>>({});

	// Modal pratinjau.
	let previewOpen = $state(false);
	let previewTipe = $state<'laporan' | 'analisa'>('laporan');
	let previewUrl = $state('');
	let previewRow = $state<Pengumpulan | null>(null);

	let filtered = $derived(
		filterKelas === 'all'
			? pengumpulan
			: pengumpulan.filter((p) => p.kelas_id === filterKelas)
	);

	let selectedCount = $derived(Object.values(selected).filter(Boolean).length);

	// Format ISO -> nilai untuk <input type="datetime-local"> (tanpa detik & zona).
	function toLocalInput(iso: string | null): string {
		if (!iso) return '';
		const d = new Date(iso);
		if (isNaN(d.getTime())) return '';
		const pad = (n: number) => String(n).padStart(2, '0');
		return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
	}

	// Nilai datetime-local -> ISO (atau null bila kosong).
	function fromLocalInput(v: string): string | null {
		if (!v) return null;
		const d = new Date(v);
		if (isNaN(d.getTime())) return null;
		return d.toISOString();
	}

	function fmtWaktu(iso: string | null): string {
		if (!iso) return '—';
		const d = new Date(iso);
		if (isNaN(d.getTime())) return '—';
		return d.toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' });
	}

	function namaKelas(kelasId: number): string {
		return (
			lb?.targets.find((t) => t.kelas_id === kelasId)?.kelas?.nama_kelas ??
			kelas.find((k) => k.id === kelasId)?.nama_kelas ??
			`Kelas #${kelasId}`
		);
	}

	async function load() {
		loading = true;
		err = '';
		try {
			lb = (await api.get<LaporanBesar>('/api/admin/laporan-besar')) ?? null;
			if (lb) {
				form = {
					judul: lb.judul,
					deskripsi: lb.deskripsi ?? '',
					is_aktif: !!lb.is_aktif,
					butuh_laporan: !!lb.butuh_laporan,
					butuh_analisa: !!lb.butuh_analisa
				};
				targets = (lb.targets ?? []).map((t) => ({ ...t }));
			}
		} catch (e) {
			// Belum ada record = bukan error fatal, biarkan form kosong siap dibuat.
			err = (e as Error).message;
		} finally {
			loading = false;
		}
	}

	async function loadKelas() {
		try {
			kelas = (await api.get<Kelas[]>('/api/admin/kelas')) ?? [];
		} catch {
			/* abaikan */
		}
	}

	async function loadPengumpulan() {
		if (!lb) return;
		try {
			const res = await api.get<{ data: Pengumpulan[] } | Pengumpulan[]>(
				`/api/admin/laporan-besar/${lb.id}/pengumpulan`
			);
			pengumpulan = Array.isArray(res) ? res : (res?.data ?? []);
		} catch (e) {
			err = (e as Error).message;
		}
	}

	onMount(async () => {
		await Promise.all([load(), loadKelas()]);
		await loadPengumpulan();
	});

	async function savePengaturan() {
		err = '';
		msg = '';
		savingForm = true;
		try {
			if (lb) await api.put(`/api/admin/laporan-besar/${lb.id}`, form);
			else await api.post('/api/admin/laporan-besar', form);
			msg = 'Pengaturan laporan besar tersimpan.';
			await load();
			await loadPengumpulan();
		} catch (e) {
			err = (e as Error).message;
		} finally {
			savingForm = false;
		}
	}

	function kelasTersedia(): Kelas[] {
		const dipakai = new Set(targets.map((t) => t.kelas_id));
		return kelas.filter((k) => !dipakai.has(k.id));
	}

	function tambahTarget() {
		if (!addKelasId) return;
		targets = [
			...targets,
			{
				kelas_id: addKelasId,
				buka_at: null,
				deadline_at: null,
				late_until: null,
				kelas: kelas.find((k) => k.id === addKelasId)
			}
		];
		addKelasId = null;
	}

	function hapusTarget(kelasId: number) {
		targets = targets.filter((t) => t.kelas_id !== kelasId);
	}

	async function saveTargets() {
		if (!lb) return;
		err = '';
		msg = '';
		savingTarget = true;
		try {
			await api.put(`/api/admin/laporan-besar/${lb.id}/target`, {
				targets: targets.map((t) => ({
					kelas_id: t.kelas_id,
					buka_at: t.buka_at,
					deadline_at: t.deadline_at,
					late_until: t.late_until
				}))
			});
			msg = 'Target kelas tersimpan.';
			await load();
			await loadPengumpulan();
		} catch (e) {
			err = (e as Error).message;
		} finally {
			savingTarget = false;
		}
	}

	function toggleSelect(id: number) {
		selected = { ...selected, [id]: !selected[id] };
	}

	function toggleSelectAll() {
		const all = filtered.length > 0 && filtered.every((p) => selected[p.mahasiswa_id]);
		const next: Record<number, boolean> = { ...selected };
		for (const p of filtered) next[p.mahasiswa_id] = !all;
		selected = next;
	}

	// Centang manual: tandai terkumpul / belum, dengan catatan opsional.
	async function markManual(row: Pengumpulan, status: 'terkumpul' | 'belum') {
		if (!lb) return;
		err = '';
		msg = '';
		let catatan: string | undefined;
		if (status === 'terkumpul') {
			const input = window.prompt(
				`Catatan manual untuk ${row.nama} (opsional):`,
				row.manual_catatan ?? ''
			);
			if (input === null) return; // dibatalkan
			catatan = input;
		}
		try {
			await api.put(`/api/admin/laporan-besar/${lb.id}/pengumpulan/${row.mahasiswa_id}/mark`, {
				status,
				catatan
			});
			msg = status === 'terkumpul' ? 'Ditandai terkumpul.' : 'Ditandai belum terkumpul.';
			await loadPengumpulan();
		} catch (e) {
			err = (e as Error).message;
		}
	}

	// Buka blokir: alasan WAJIB, tidak dikirim bila kosong.
	async function bukaBlokir(row: Pengumpulan) {
		if (!lb) return;
		err = '';
		msg = '';
		const alasan = window.prompt(
			`Alasan membuka blokir untuk ${row.nama} (wajib diisi):`,
			''
		);
		if (alasan === null) return;
		if (!alasan.trim()) {
			err = 'Alasan buka blokir wajib diisi.';
			return;
		}
		try {
			await api.put(
				`/api/admin/laporan-besar/${lb.id}/pengumpulan/${row.mahasiswa_id}/override`,
				{ alasan: alasan.trim() }
			);
			msg = 'Blokir dibuka.';
			await loadPengumpulan();
		} catch (e) {
			err = (e as Error).message;
		}
	}

	async function cabutBlokir(row: Pengumpulan) {
		if (!lb) return;
		if (
			!(await confirmAction({
				title: 'Cabut Buka Blokir?',
				message: `Cabut izin buka blokir untuk ${row.nama}?`
			}))
		)
			return;
		err = '';
		msg = '';
		try {
			await api.del(
				`/api/admin/laporan-besar/${lb.id}/pengumpulan/${row.mahasiswa_id}/override`
			);
			msg = 'Buka blokir dicabut.';
			await loadPengumpulan();
		} catch (e) {
			err = (e as Error).message;
		}
	}

	// Pratinjau: PDF inline via iframe, DOCX tidak bisa inline.
	// Endpoint minta signed URL lewat api.get (mengirim JWT) lalu iframe pakai URL itu.
	async function openPreview(row: Pengumpulan, jenis: 'laporan' | 'analisa') {
		if (!lb) return;
		previewRow = row;
		previewTipe = jenis;
		previewUrl = '';
		previewOpen = true;
		err = '';
		try {
			const res = await api.get<{ url: string; filename: string }>(
				`/api/admin/laporan-besar/${lb.id}/pengumpulan/${row.mahasiswa_id}/preview?jenis=${jenis}`
			);
			previewUrl = res?.url ?? '';
			if (!previewUrl) err = 'Tidak bisa memuat pratinjau.';
		} catch (e) {
			err = (e as Error).message;
		}
	}

	function closePreview() {
		previewOpen = false;
		previewUrl = '';
		previewRow = null;
	}

	function isDocx(url: string | null): boolean {
		if (!url) return false;
		return url.toLowerCase().includes('.doc');
	}

	// Unduh satu berkas: minta signed URL lewat api.get (JWT) lalu klik tautan itu.
	async function openDownload(row: Pengumpulan, jenis: 'laporan' | 'analisa') {
		if (!lb) return;
		err = '';
		try {
			const res = await api.get<{ url: string; filename: string }>(
				`/api/admin/laporan-besar/${lb.id}/pengumpulan/${row.mahasiswa_id}/download?jenis=${jenis}`
			);
			if (!res?.url) {
				err = 'Berkas belum tersedia.';
				return;
			}
			const a = document.createElement('a');
			a.href = res.url;
			a.download = res.filename || '';
			document.body.appendChild(a);
			a.click();
			a.remove();
		} catch (e) {
			err = (e as Error).message;
		}
	}

	// Unduh massal: minta URL dari backend, lalu unduh tiap file berurutan.
	async function unduhTerpilih() {
		if (!lb) return;
		const picked = filtered.filter((p) => selected[p.mahasiswa_id]);
		if (picked.length === 0) {
			err = 'Pilih minimal satu baris terlebih dahulu.';
			return;
		}
		err = '';
		msg = '';
		const items: { mhs_id: number; jenis: string }[] = [];
		for (const p of picked) {
			if (form.butuh_laporan) items.push({ mhs_id: p.mahasiswa_id, jenis: 'laporan' });
			if (form.butuh_analisa) items.push({ mhs_id: p.mahasiswa_id, jenis: 'analisa' });
		}
		if (items.length === 0) {
			err = 'Tidak ada jenis dokumen yang diminta (laporan/analisa nonaktif).';
			return;
		}
		try {
			const res = await api.post<{ data: { mhs_id: number; jenis: string; filename: string; url: string }[] }>(
				`/api/admin/laporan-besar/${lb.id}/download-batch`,
				{ items }
			);
			const list = res?.data ?? [];
			msg = `Mengunduh ${list.length} berkas…`;
			for (const f of list) {
				const a = document.createElement('a');
				a.href = f.url;
				a.download = f.filename || '';
				document.body.appendChild(a);
				a.click();
				a.remove();
				// jeda kecil agar browser tidak memblokir unduhan beruntun
				await new Promise((r) => setTimeout(r, 350));
			}
		} catch (e) {
			err = (e as Error).message;
		}
	}

	function statusBadge(p: Pengumpulan): { label: string; cls: string } {
		if (p.override_alasan) {
			return { label: 'Buka blokir', cls: 'bg-state-info-bg text-state-info' };
		}
		if (p.status === 'terkumpul') {
			if (p.is_late) return { label: 'Terkumpul (telat)', cls: 'bg-amber-50 text-amber-600' };
			return { label: 'Terkumpul', cls: 'bg-state-success-bg text-state-success' };
		}
		return { label: 'Belum', cls: 'bg-surface-soft text-ink-caption' };
	}
</script>

<h1 class="mb-4 text-2xl">Laporan Besar</h1>
<p class="mb-4 text-sm text-ink-caption">
	Satu tugas laporan berdiri sendiri (terpisah dari modul praktikum). Atur judul, target kelas, dan pantau
	pengumpulan mahasiswa di sini.
</p>

{#if msg}<p class="mb-3 rounded-lg bg-state-success-bg p-3 text-sm text-state-success">{msg}</p>{/if}
{#if err}<p class="mb-3 rounded-lg bg-state-error-bg p-3 text-sm text-state-error">{err}</p>{/if}

{#if loading}
	<p class="text-sm text-ink-caption">Memuat…</p>
{:else}
	<!-- 1. Form pengaturan laporan besar -->
	<div class="card max-w-2xl">
		<h2 class="mb-3 text-lg">{lb ? 'Pengaturan Laporan Besar' : 'Buat Laporan Besar'}</h2>
		<label class="label" for="lb-judul">Judul</label>
		<input id="lb-judul" class="input" bind:value={form.judul} placeholder="mis. Laporan Besar" />
		<label class="label mt-2" for="lb-desk">Deskripsi</label>
		<textarea id="lb-desk" class="input min-h-16" bind:value={form.deskripsi}></textarea>
		<div class="mt-3 space-y-2">
			<label class="flex items-center gap-2 text-sm">
				<input type="checkbox" bind:checked={form.is_aktif} /> Aktifkan laporan besar
			</label>
			<label class="flex items-center gap-2 text-sm">
				<input type="checkbox" bind:checked={form.butuh_laporan} /> Wajib unggah laporan
			</label>
			<label class="flex items-center gap-2 text-sm">
				<input type="checkbox" bind:checked={form.butuh_analisa} /> Wajib unggah analisa
			</label>
		</div>
		<div class="mt-3">
			<button class="btn-primary" onclick={savePengaturan} disabled={savingForm}>
				{savingForm ? 'Menyimpan…' : 'Simpan'}
			</button>
		</div>
	</div>

	{#if lb}
		<!-- 2. Target kelas -->
		<hr class="my-6 border-gray-200" />
		<h2 class="mb-3 text-xl">Target Kelas</h2>
		<p class="mb-3 text-sm text-ink-caption">
			Atur jadwal buka, tenggat, dan batas telat untuk tiap kelas. Kosongkan buka/late untuk membiarkannya
			tanpa batas.
		</p>

		<div class="mb-3 flex flex-wrap items-end gap-2">
			<div>
				<label class="label" for="add-kelas">Tambah Kelas</label>
				<select id="add-kelas" class="input min-w-52" bind:value={addKelasId}>
					<option value={null}>— Pilih Kelas —</option>
					{#each kelasTersedia() as k}
						<option value={k.id}>{k.nama_kelas}</option>
					{/each}
				</select>
			</div>
			<button class="btn-outline" onclick={tambahTarget} disabled={!addKelasId}>+ Tambah</button>
			<button class="btn-primary" onclick={saveTargets} disabled={savingTarget}>
				<Save size={14} /> {savingTarget ? 'Menyimpan…' : 'Simpan Target'}
			</button>
		</div>

		<div class="table-wrap">
			<table class="tbl">
				<thead>
					<tr>
						<th>Kelas</th>
						<th>Buka</th>
						<th>Deadline</th>
						<th>Batas Telat</th>
						<th>Aksi</th>
					</tr>
				</thead>
				<tbody>
					{#each targets as t (t.kelas_id)}
						<tr>
							<td>{t.kelas?.nama_kelas ?? namaKelas(t.kelas_id)}</td>
							<td>
								<input
									type="datetime-local"
									class="input text-sm"
									value={toLocalInput(t.buka_at)}
									oninput={(e) => (t.buka_at = fromLocalInput((e.target as HTMLInputElement).value))}
								/>
							</td>
							<td>
								<input
									type="datetime-local"
									class="input text-sm"
									value={toLocalInput(t.deadline_at)}
									oninput={(e) =>
										(t.deadline_at = fromLocalInput((e.target as HTMLInputElement).value))}
								/>
							</td>
							<td>
								<input
									type="datetime-local"
									class="input text-sm"
									value={toLocalInput(t.late_until)}
									oninput={(e) => (t.late_until = fromLocalInput((e.target as HTMLInputElement).value))}
								/>
							</td>
							<td>
								<button
									class="inline-flex items-center gap-1 bg-red-50 hover:bg-red-650 hover:text-white text-red-650 px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95 border border-red-100"
									onclick={() => hapusTarget(t.kelas_id)}
								>
									<Trash2 size={12} /> Hapus
								</button>
							</td>
						</tr>
					{:else}
						<tr>
							<td colspan="5" class="text-center text-sm text-ink-caption">
								Belum ada target kelas. Tambahkan kelas di atas lalu klik Simpan Target.
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<!-- 3. Pengumpulan -->
		<hr class="my-6 border-gray-200" />
		<div class="mb-3 flex flex-wrap items-center justify-between gap-3">
			<h2 class="text-xl">Pengumpulan ({filtered.length})</h2>
			<div class="flex flex-wrap items-center gap-2">
				<select class="input min-w-44" bind:value={filterKelas}>
					<option value="all">Semua Kelas</option>
					{#each lb.targets as t}
						<option value={t.kelas_id}>{t.kelas?.nama_kelas ?? namaKelas(t.kelas_id)}</option>
					{/each}
				</select>
				<button class="btn-primary" onclick={unduhTerpilih} disabled={selectedCount === 0}>
					<Download size={14} /> Unduh terpilih ({selectedCount})
				</button>
			</div>
		</div>
		<p class="mb-3 text-xs text-ink-caption">
			Unduhan massal mengambil URL dari server lalu mengunduh tiap berkas berurutan. Untuk banyak berkas
			sekaligus, pertimbangkan memakai pembuat ZIP di kemudian hari.
		</p>

		<div class="table-wrap">
			<table class="tbl">
				<thead>
					<tr>
						<th>
							<input type="checkbox" onchange={toggleSelectAll} />
						</th>
						<th>NIM</th>
						<th>Nama</th>
						<th>Kelas</th>
						<th>Status</th>
						<th>Sumber</th>
						<th>Waktu</th>
						<th>Aksi</th>
					</tr>
				</thead>
				<tbody>
					{#each filtered as p (p.mahasiswa_id)}
						{@const b = statusBadge(p)}
						<tr>
							<td><input type="checkbox" checked={!!selected[p.mahasiswa_id]} onchange={() => toggleSelect(p.mahasiswa_id)} /></td>
							<td>{p.nim}</td>
							<td>{p.nama}</td>
							<td>{p.nama_kelas}</td>
							<td>
								<span class="badge {b.cls}">{b.label}</span>
								{#if p.manual_catatan}
									<p class="mt-1 text-xs text-ink-caption">{p.manual_catatan}</p>
								{/if}
								{#if p.override_alasan}
									<p class="mt-1 text-xs text-state-info">Alasan: {p.override_alasan}</p>
								{/if}
							</td>
							<td>{p.sumber ?? '—'}</td>
							<td class="whitespace-nowrap">{fmtWaktu(p.submitted_at)}</td>
							<td>
								<div class="flex flex-wrap items-center gap-1.5">
									{#if p.laporan_url}
										<button
											class="inline-flex items-center gap-1 bg-blue-50 hover:bg-blue-650 hover:text-white text-blue-600 px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95 border border-blue-100"
											onclick={() => openPreview(p, 'laporan')}
										>
											<Eye size={12} /> Laporan
										</button>
										<button
											class="inline-flex items-center gap-1 bg-surface-soft hover:bg-primary hover:text-white text-ink-body px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95"
											onclick={() => openDownload(p, 'laporan')}
										>
											<Download size={12} />
										</button>
									{/if}
									{#if p.analisa_url}
										<button
											class="inline-flex items-center gap-1 bg-blue-50 hover:bg-blue-650 hover:text-white text-blue-600 px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95 border border-blue-100"
											onclick={() => openPreview(p, 'analisa')}
										>
											<Eye size={12} /> Analisa
										</button>
										<button
											class="inline-flex items-center gap-1 bg-surface-soft hover:bg-primary hover:text-white text-ink-body px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95"
											onclick={() => openDownload(p, 'analisa')}
										>
											<Download size={12} />
										</button>
									{/if}
									{#if p.status === 'belum'}
										<button
											class="inline-flex items-center gap-1 bg-primary/10 hover:bg-primary hover:text-white text-primary px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95"
											onclick={() => markManual(p, 'terkumpul')}
										>
											<Check size={12} /> Centang manual
										</button>
									{:else}
										<button
											class="inline-flex items-center gap-1 bg-surface-soft hover:bg-primary hover:text-white text-ink-body px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95"
											onclick={() => markManual(p, 'belum')}
										>
											Batal centang
										</button>
									{/if}
									{#if p.override_alasan}
										<button
											class="inline-flex items-center gap-1 bg-amber-50 hover:bg-amber-600 hover:text-white text-amber-600 px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95 border border-amber-100"
											onclick={() => cabutBlokir(p)}
										>
											<Lock size={12} /> Cabut blokir
										</button>
									{:else}
										<button
											class="inline-flex items-center gap-1 bg-amber-50 hover:bg-amber-600 hover:text-white text-amber-600 px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95 border border-amber-100"
											onclick={() => bukaBlokir(p)}
										>
											<Unlock size={12} /> Buka blokir
										</button>
									{/if}
								</div>
							</td>
						</tr>
					{:else}
						<tr>
							<td colspan="8" class="text-center text-sm text-ink-caption">Belum ada data pengumpulan.</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
{/if}

<!-- 5. Modal pratinjau -->
{#if previewOpen && previewRow}
	<div
		class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-black/50 p-4 backdrop-blur-sm sm:items-center"
		role="presentation"
		onclick={(e) => { if (e.target === e.currentTarget) closePreview(); }}
	>
		<div class="my-8 w-full max-w-4xl rounded-2xl bg-white shadow-2xl" role="dialog" aria-modal="true">
			<div class="flex items-center justify-between border-b border-gray-200 px-6 py-4">
				<h3 class="text-lg font-bold text-ink-heading">
					Pratinjau {previewTipe === 'laporan' ? 'Laporan' : 'Analisa'} — {previewRow.nama}
				</h3>
				<button class="text-ink-caption hover:text-ink-heading" aria-label="Tutup" onclick={closePreview}>
					<X size={20} />
				</button>
			</div>
			<div class="max-h-[80vh] overflow-y-auto px-6 py-4">
				{#if isDocx(previewTipe === 'laporan' ? previewRow.laporan_url : previewRow.analisa_url)}
					<div class="rounded-lg bg-surface-soft p-6 text-center">
						<p class="mb-3 text-sm text-ink-body">
							Pratinjau DOCX tidak tersedia inline — silakan unduh.
						</p>
						<button class="btn-primary" onclick={() => openDownload(previewRow!, previewTipe)}>
							<Download size={14} /> Unduh Berkas
						</button>
					</div>
				{:else}
					<iframe src={previewUrl} title="Pratinjau berkas" class="h-[70vh] w-full rounded-lg border border-gray-200"></iframe>
				{/if}
			</div>
			<div class="flex justify-end gap-2 border-t border-gray-200 px-6 py-4">
				<button class="btn-outline" onclick={closePreview}>Tutup</button>
				<button class="btn-primary" onclick={() => openDownload(previewRow!, previewTipe)}>
					<Download size={14} /> Unduh
				</button>
			</div>
		</div>
	</div>
{/if}
