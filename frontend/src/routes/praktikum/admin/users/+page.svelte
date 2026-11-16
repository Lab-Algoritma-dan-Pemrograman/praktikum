<script lang="ts">
	import { onMount } from 'svelte';
	import Papa from 'papaparse';
	import { api } from '$lib/api';
	import { KeyRound, Trash2, Search, UserPlus, X, Edit, FileDown, FileUp } from 'lucide-svelte';
	import { confirmAction } from '$lib/stores/confirm';
	import type { Kelas, User } from '$lib/types';

	let users = $state<User[]>([]);
	let kelas = $state<Kelas[]>([]);
	let err = $state('');
	let msg = $state('');

	// Modal visibility
	let showFormModal = $state(false);

	// Search & Filter
	let userSearch = $state('');
	let filterKelas = $state('');
	let filterShift = $state('');

	let filteredUsers = $derived(
		users.filter((u) => {
			const q = userSearch.toLowerCase().trim();
			const matchSearch = q === '' || u.nim.toLowerCase().includes(q) || u.nama.toLowerCase().includes(q);
			const matchKelas = filterKelas === '' || (u.nama_kelas ?? u.kelas?.nama_kelas ?? '') === filterKelas;
			const matchShift = filterShift === '' || String(u.shift) === filterShift;
			return matchSearch && matchKelas && matchShift;
		})
	);

	let editId = $state<number | null>(null);
	let form = $state({ nim: '', nama: '', kelas_id: null as number | null, shift: 1 as number, gelombang: null as number | null, kelompok: '' });

	let showImport = $state(false);
	let importRows = $state<any[]>([]);
	let importErrors = $state<string[]>([]);
	let isImporting = $state(false);

	// Bulk select
	let selectedIds = $state<Set<number>>(new Set());
	let bulkBusy = $state(false);

	// Bulk set atribut (gelombang/shift/kelas/kelompok)
	let bulkForm = $state<{ gelombang: string; shift: string; kelas_id: string; kelompok: string }>({ gelombang: '', shift: '', kelas_id: '', kelompok: '' });

	function selectedNims(): string[] {
		const idSet = selectedIds;
		return users.filter((u) => idSet.has(u.id)).map((u) => u.nim);
	}

	async function bulkSetAttrs() {
		const nims = selectedNims();
		if (nims.length === 0) return;
		const body: Record<string, unknown> = { nims };
		if (bulkForm.gelombang === '__null__') body.set_gelombang_null = true;
		else if (bulkForm.gelombang) body.gelombang = Number(bulkForm.gelombang);
		if (bulkForm.shift) body.shift = Number(bulkForm.shift);
		if (bulkForm.kelas_id === '__null__') body.set_kelas_null = true;
		else if (bulkForm.kelas_id) body.kelas_id = Number(bulkForm.kelas_id);
		if (bulkForm.kelompok.trim()) body.kelompok = bulkForm.kelompok.trim();
		const count = nims.length;
		if (!await confirmAction({
			title: `Ubah atribut ${count} mahasiswa?`,
			message: `Aksi ini akan mengubah atribut (gelombang/shift/kelas/kelompok) untuk ${count} mahasiswa terpilih. Hanya field yang diisi yang diubah.`
		})) return;
		bulkBusy = true; err = ''; msg = '';
		try {
			const res = await api.put<{ total_processed: number; total_updated: number }>('/api/admin/users/bulk-attrs', body);
			msg = `Berhasil mengubah ${res?.total_updated ?? count} mahasiswa.`;
			selectedIds = new Set();
			bulkForm = { gelombang: '', shift: '', kelas_id: '', kelompok: '' };
			await load();
		} catch (e) { err = (e as Error).message; }
		finally { bulkBusy = false; }
	}

	async function load() {
		try {
			users = (await api.get<User[]>('/api/admin/users')) ?? [];
			kelas = (await api.get<Kelas[]>('/api/admin/kelas')) ?? [];
			if (kelas.length > 0 && form.kelas_id === null) {
				form.kelas_id = kelas[0].id;
			}
		} catch (e) { err = (e as Error).message; }
	}
	onMount(load);

	function toggleSelect(id: number) {
		const s = new Set(selectedIds);
		s.has(id) ? s.delete(id) : s.add(id);
		selectedIds = s;
	}
	function toggleSelectAll() {
		selectedIds = selectedIds.size === filteredUsers.length ? new Set() : new Set(filteredUsers.map((u) => u.id));
	}

	async function bulkAction(action: 'delete' | 'reset_pw') {
		if (selectedIds.size === 0) return;
		const label = action === 'delete' ? 'Hapus' : 'Reset password';
		const extra = action === 'reset_pw' ? ' Mereka harus melakukan `register ulang` untuk mendapatkan akses kembali.' : '';
		if (!await confirmAction({
			title: `${action === 'delete' ? 'Hapus' : 'Reset Password'} ${selectedIds.size} Mahasiswa?`,
			message: `Aksi ini akan ${action === 'delete' ? 'menghapus permanen' : 'mereset sandi'} ${selectedIds.size} mahasiswa terpilih.${extra}`
		})) return;
		bulkBusy = true; err = ''; msg = '';
		let ok = 0, fail = 0;
		for (const id of selectedIds) {
			try {
				if (action === 'delete') await api.del(`/api/admin/users/${id}`);
				else await api.post(`/api/admin/users/${id}/reset-password`);
				ok++;
			} catch { fail++; }
		}
		bulkBusy = false;
		selectedIds = new Set();
		msg = `${label} selesai: ${ok} berhasil${fail ? `, ${fail} gagal` : ''}.`;
		await load();
	}

	function resetForm() {
		editId = null;
		form = { nim: '', nama: '', kelas_id: kelas[0]?.id ?? null, shift: 1, gelombang: null, kelompok: '' };
	}

	function openAddModal() {
		resetForm();
		showFormModal = true;
	}

	function edit(u: User) {
		editId = u.id;
		form = { nim: u.nim, nama: u.nama, kelas_id: u.kelas_id ?? null, shift: u.shift ?? 1, gelombang: u.gelombang ?? null, kelompok: u.kelompok ?? '' };
		showFormModal = true;
	}

	async function save() {
		err = ''; msg = '';
		try {
			const body = { nim: form.nim, nama: form.nama, kelas_id: form.kelas_id, shift: Number(form.shift), gelombang: form.gelombang ? Number(form.gelombang) : null, kelompok: form.kelompok || null };
			if (editId) await api.put(`/api/admin/users/${editId}`, body);
			else await api.post('/api/admin/users', body);
			msg = 'Tersimpan.';
			showFormModal = false;
			resetForm();
			await load();
		} catch (e) { err = (e as Error).message; }
	}

	async function del(id: number) {
		if (!await confirmAction({
			title: 'Hapus Mahasiswa?',
			message: 'Apakah Anda yakin ingin menghapus data mahasiswa ini? Tindakan ini tidak dapat dibatalkan.'
		})) return;
		try { await api.del(`/api/admin/users/${id}`); await load(); }
		catch (e) { err = (e as Error).message; }
	}

	async function resetPw(id: number) {
		if (!await confirmAction({
			title: 'Reset Password Mahasiswa?',
			message: 'Aksi ini akan menghapus sandi saat ini. Mahasiswa yang bersangkutan harus melakukan `register ulang` untuk mendapatkan akses kembali.'
		})) return;
		try { await api.post(`/api/admin/users/${id}/reset-password`); msg = 'Password direset.'; await load(); }
		catch (e) { err = (e as Error).message; }
	}


	function onFileChange(e: Event) {
		const file = (e.target as HTMLInputElement).files?.[0];
		if (!file) return;
		Papa.parse(file, {
			header: true,
			skipEmptyLines: true,
			complete: (results) => {
				importErrors = [];
				let valid = [];
				for (let i = 0; i < results.data.length; i++) {
					const row = results.data[i] as any;
					if (!row.NIM || !row.Nama || !row.Kelas) {
						importErrors.push(`Baris ${i + 1}: Format tidak lengkap. Harus ada header NIM, Nama, Kelas.`);
						continue;
					}
					const k = kelas.find((kl) => kl.nama_kelas === row.Kelas);
					if (!k) {
						importErrors.push(`Baris ${i + 1} (${row.NIM}): Kelas "${row.Kelas}" tidak ditemukan di database.`);
						continue;
					}
					valid.push({
						nim: row.NIM,
						nama: row.Nama,
						kelas_id: k.id,
						shift: Number(row.Shift) || 1,
						gelombang: row.Gelombang ? Number(row.Gelombang) : null,
						kelompok: row.Kelompok || ''
					});
				}
				if (importErrors.length === 0) importRows = valid;
				else importRows = [];
			}
		});
	}

	async function importBulk() {
		if (importRows.length === 0) return;
		isImporting = true;
		err = ''; msg = '';
		try {
			const res = await api.post<any>('/api/admin/users/bulk', { users: importRows });
			msg = `Berhasil memproses ${res?.total_processed ?? importRows.length} mahasiswa.`;
			showImport = false;
			importRows = [];
			await load();
		} catch (e) { err = (e as Error).message; }
		finally { isImporting = false; }
	}

	function downloadTemplate() {
		const csvContent = "NIM,Nama,Kelas,Shift,Gelombang,Kelompok\n12345678,Budi Santoso,4IA01,1,,A\n";
		const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
		const url = URL.createObjectURL(blob);
		const link = document.createElement('a');
		link.setAttribute('href', url);
		link.setAttribute('download', 'template_import_mahasiswa.csv');
		document.body.appendChild(link);
		link.click();
		document.body.removeChild(link);
	}
</script>

<div class="mb-4 flex flex-wrap items-center justify-between gap-3 text-left">
	<h1 class="text-2xl font-bold text-slate-900">Manajemen Data User</h1>
	<div class="space-x-2 flex">
		<button class="btn-outline text-xs flex items-center gap-1.5 hover:bg-slate-50 transition-colors" onclick={downloadTemplate}>
			<FileDown size={14} class="text-slate-500" /> Unduh Template
		</button>
		<button class="btn-outline text-xs flex items-center gap-1.5 transition-colors {showImport ? 'border-primary text-primary bg-primary/5 hover:bg-primary/10' : 'hover:bg-slate-50'}" onclick={() => (showImport = !showImport)}>
			<FileUp size={14} class={showImport ? 'text-primary' : 'text-slate-500'} /> {showImport ? 'Tutup Import' : 'Import CSV'}
		</button>
		<button class="btn-primary text-xs flex items-center gap-1.5" onclick={openAddModal}>
			<UserPlus size={14} /> Tambah Mahasiswa
		</button>
	</div>
</div>

{#if msg}<p class="mb-3 rounded-lg bg-state-success-bg p-3 text-sm text-state-success text-left">{msg}</p>{/if}
{#if err}<p class="mb-3 rounded-lg bg-state-error-bg p-3 text-sm text-state-error text-left">{err}</p>{/if}

<!-- Import Section -->
{#if showImport}
	<div class="card border-primary border-dashed border-2 mb-6 text-left">
		<h2 class="mb-3 text-lg font-bold">Import Mahasiswa via CSV</h2>
		<p class="mb-3 text-sm text-ink-caption leading-relaxed">
			Pastikan file CSV memiliki header (baris pertama) persis: <strong>NIM, Nama, Kelas, Shift, Gelombang, Kelompok</strong>.<br/>
			Kolom Gelombang opsional (hanya untuk ujian praktik), boleh dikosongkan. Jika NIM sudah ada di sistem, data namanya, kelas, dan shift akan di-_update_ tanpa mereset password.
		</p>
		<input type="file" accept=".csv" class="mb-3 block w-full text-sm file:mr-4 file:rounded-lg file:border-0 file:bg-primary file:px-4 file:py-2 file:text-sm file:font-semibold file:text-white hover:file:bg-primary/90" onchange={onFileChange} />
		
		{#if importErrors.length > 0}
			<div class="mb-3 max-h-48 overflow-y-auto rounded-lg bg-state-error-bg p-3 text-sm text-state-error">
				<p class="font-bold">Terdapat Error pada file CSV:</p>
				<ul class="list-inside list-disc">
					{#each importErrors as ie}<li>{ie}</li>{/each}
				</ul>
				<p class="mt-2 font-bold">Harap perbaiki file CSV dan unggah ulang.</p>
			</div>
		{/if}

		{#if importRows.length > 0}
			<p class="mb-2 text-sm font-bold text-state-success">Valid! {importRows.length} data siap diimpor.</p>
			<button class="btn-primary w-full max-w-sm" onclick={importBulk} disabled={isImporting}>
				{isImporting ? 'Memproses...' : 'Mulai Import'}
			</button>
		{/if}
	</div>
{/if}

<!-- Main Table Card - Full Width -->
<div class="card w-full text-left space-y-4">
	{#if selectedIds.size > 0}
		<div class="space-y-3 rounded-xl border border-primary/20 bg-primary/5 p-3 shadow-sm">
			<div class="flex flex-wrap items-center gap-3">
				<span class="text-sm font-semibold text-primary"><strong>{selectedIds.size}</strong> terpilih</span>
				<div class="ml-auto flex flex-wrap gap-2">
					<button class="btn-outline inline-flex items-center gap-1 border-state-warning py-1.5 text-state-warning hover:bg-state-warning hover:text-white" disabled={bulkBusy} onclick={() => bulkAction('reset_pw')}>
						<KeyRound size={14} /> Reset PW
					</button>
					<button class="btn-outline inline-flex items-center gap-1 border-state-error py-1.5 text-state-error hover:bg-state-error hover:text-white" disabled={bulkBusy} onclick={() => bulkAction('delete')}>
						<Trash2 size={14} /> Hapus
					</button>
					<button class="btn-outline py-1.5" disabled={bulkBusy} onclick={() => (selectedIds = new Set())}>Batal</button>
				</div>
			</div>
			<!-- Bulk Set Atribut -->
			<div class="flex flex-wrap items-end gap-2 border-t border-primary/20 pt-3">
				<span class="text-xs font-semibold text-ink-caption self-center">Set Atribut:</span>
				<div>
					<label class="label text-[10px]" for="b_gel">Gelombang</label>
					<select id="b_gel" class="input w-auto text-xs" bind:value={bulkForm.gelombang}>
						<option value="">(tidak diubah)</option>
						<option value="1">Gelombang 1</option>
						<option value="2">Gelombang 2</option>
						<option value="__null__">(kosongkan)</option>
					</select>
				</div>
				<div>
					<label class="label text-[10px]" for="b_shift">Shift</label>
					<select id="b_shift" class="input w-auto text-xs" bind:value={bulkForm.shift}>
						<option value="">(tidak diubah)</option>
						<option value="1">Shift 1</option>
						<option value="2">Shift 2</option>
					</select>
				</div>
				<div>
					<label class="label text-[10px]" for="b_kelas">Kelas</label>
					<select id="b_kelas" class="input w-auto text-xs" bind:value={bulkForm.kelas_id}>
						<option value="">(tidak diubah)</option>
						{#each kelas as k}<option value={String(k.id)}>{k.nama_kelas}</option>{/each}
					</select>
				</div>
				<div>
					<label class="label text-[10px]" for="b_kel">Kelompok</label>
					<input id="b_kel" type="text" placeholder="(tidak diubah)" class="input w-28 text-xs" bind:value={bulkForm.kelompok} />
				</div>
				<button class="btn-primary py-1.5 text-xs" disabled={bulkBusy} onclick={bulkSetAttrs}>Terapkan</button>
			</div>
		</div>
	{/if}

	<!-- Search & Filter Controls -->
	<div class="flex flex-col gap-3 sm:flex-row sm:items-center">
		<div class="relative flex-1 max-w-xs">
			<Search size={14} class="absolute left-3 top-1/2 -translate-y-1/2 text-ink-caption" />
			<input type="text" placeholder="Cari NIM atau Nama..." bind:value={userSearch} class="input pl-9 w-full text-sm" />
		</div>
		<select bind:value={filterKelas} class="input w-auto text-sm bg-white border border-slate-200">
			<option value="">Semua Kelas</option>
			{#each kelas as k}<option value={k.nama_kelas}>{k.nama_kelas}</option>{/each}
		</select>
		<select bind:value={filterShift} class="input w-auto text-sm bg-white border border-slate-200">
			<option value="">Semua Shift</option>
			<option value="1">Shift 1</option>
			<option value="2">Shift 2</option>
		</select>
	</div>

	<div class="table-wrap">
		<table class="tbl w-full">
			<thead>
				<tr>
					<th class="w-10 text-center">
						<input type="checkbox" class="rounded border-gray-300 text-primary focus:ring-primary"
							checked={filteredUsers.length > 0 && selectedIds.size === filteredUsers.length}
							onchange={toggleSelectAll} aria-label="Pilih semua" />
					</th>
					<th>NIM</th><th>Nama</th><th>Kelas</th><th>Shift</th><th>Gel.</th><th>Kelompok</th><th>Status</th><th>Aksi</th>
				</tr>
			</thead>
			<tbody>
				{#each filteredUsers as u}
					<tr class={selectedIds.has(u.id) ? 'bg-primary/5' : ''}>
						<td class="text-center">
							<input type="checkbox" class="rounded border-gray-300 text-primary focus:ring-primary"
								checked={selectedIds.has(u.id)} onchange={() => toggleSelect(u.id)} aria-label={`Pilih ${u.nim}`} />
						</td>
						<td class="font-mono text-xs font-semibold text-slate-800">{u.nim}</td>
						<td class="font-semibold text-slate-700">{u.nama}</td>
						<td><span class="badge bg-slate-50 border border-slate-100 font-semibold">{u.nama_kelas ?? u.kelas?.nama_kelas ?? '-'}</span></td>
						<td><span class="badge bg-slate-100 border border-slate-200">Shift {u.shift ?? '-'}</span></td>
						<td><span class="badge bg-slate-50 border border-slate-150">{u.gelombang ?? '-'}</span></td>
						<td class="font-bold text-primary">{u.kelompok ?? '-'}</td>
						<td>
							{#if u.is_registered}
								<span class="badge bg-state-success-bg text-state-success">Terdaftar</span>
							{:else}
								<span class="badge bg-gray-100 text-ink-caption">Belum</span>
							{/if}
						</td>
						<td class="flex items-center gap-1.5 whitespace-nowrap">
							<button class="inline-flex items-center gap-1 bg-primary/10 hover:bg-primary hover:text-white text-primary px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95" onclick={() => edit(u)}>
								<Edit size={12} /> Edit
							</button>
							<button class="inline-flex items-center gap-1 bg-amber-50 hover:bg-amber-500 hover:text-white text-amber-600 px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95 border border-amber-100" onclick={() => resetPw(u.id)}>
								<KeyRound size={12} /> Reset PW
							</button>
							<button class="inline-flex items-center gap-1 bg-red-50 hover:bg-red-650 hover:text-white text-red-650 px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95 border border-red-100" onclick={() => del(u.id)}>
								<Trash2 size={12} /> Hapus
							</button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
</div>

<!-- Modal Popup Form Add/Edit -->
{#if showFormModal}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/50 p-4 backdrop-blur-sm"
		role="presentation"
		onclick={(e) => { if (e.target === e.currentTarget) showFormModal = false; }}
	>
		<div class="w-full max-w-lg rounded-2xl bg-white shadow-2xl text-left" role="dialog" aria-modal="true">
			<div class="flex items-center justify-between border-b border-gray-200 px-6 py-4">
				<h3 class="text-lg font-bold text-slate-900">{editId ? 'Edit' : 'Tambah'} Mahasiswa</h3>
				<button class="text-slate-400 hover:text-slate-700" aria-label="Tutup" onclick={() => (showFormModal = false)}><X size={20} /></button>
			</div>
			
			<div class="px-6 py-4 space-y-3.5">
				<div>
					<label class="label font-semibold text-slate-600" for="nim">NIM</label>
					<input id="nim" class="input mt-1 w-full" bind:value={form.nim} placeholder="Ketik NIM..." />
				</div>
				<div>
					<label class="label font-semibold text-slate-600" for="nama">Nama</label>
					<input id="nama" class="input mt-1 w-full" bind:value={form.nama} placeholder="Ketik Nama Lengkap..." />
				</div>
				<div>
					<label class="label font-semibold text-slate-600" for="kelas">Kelas</label>
					<select id="kelas" class="input mt-1 w-full bg-white border border-slate-200" bind:value={form.kelas_id}>
						{#each kelas as k}<option value={k.id}>{k.nama_kelas}</option>{/each}
					</select>
				</div>
				<div>
					<label class="label font-semibold text-slate-600" for="shift">Shift</label>
					<select id="shift" class="input mt-1 w-full bg-white border border-slate-200" bind:value={form.shift}>
						<option value={1}>Shift 1</option>
						<option value={2}>Shift 2</option>
					</select>
				</div>
				<div>
					<label class="label font-semibold text-slate-600" for="gelombang">Gelombang <span class="text-ink-caption font-normal">(ujian praktik)</span></label>
					<select id="gelombang" class="input mt-1 w-full bg-white border border-slate-200" bind:value={form.gelombang}>
						<option value={null}>— Tidak ada —</option>
						<option value={1}>Gelombang 1</option>
						<option value={2}>Gelombang 2</option>
					</select>
				</div>
				<div>
					<label class="label font-semibold text-slate-600" for="kelompok">Kelompok</label>
					<input id="kelompok" class="input mt-1 w-full" bind:value={form.kelompok} placeholder="mis. A, B, 1" />
				</div>
			</div>
			
			<div class="flex justify-end gap-2 border-t border-gray-250 px-6 py-4">
				<button class="btn-outline px-4 py-2 font-bold" onclick={() => (showFormModal = false)}>Batal</button>
				<button class="btn-primary px-5 py-2" onclick={save}>Simpan</button>
			</div>
		</div>
	</div>
{/if}
