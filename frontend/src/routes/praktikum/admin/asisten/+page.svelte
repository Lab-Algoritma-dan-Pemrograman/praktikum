<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { UserPlus, Plus, X, Trash2, Edit, Search, ArrowUp } from 'lucide-svelte';
	import { confirmAction } from '$lib/stores/confirm';
	import { user } from '$lib/stores/auth';
	import type { User, Kelas, AmpuanKelompok } from '$lib/types';

	// Hapus akun asisten hanya untuk superadmin (backend juga menolak role lain).
	let canDeleteAsisten = $derived($user?.role === 'koordinator');

	let list = $state<User[]>([]);
	let kelasList = $state<Kelas[]>([]);
	let ampuanList = $state<AmpuanKelompok[]>([]);
	let err = $state(''); let msg = $state('');

	// Filter state
	let searchAsisten = $state('');

	let filteredAsisten = $derived(
		list.filter((a) => {
			const q = searchAsisten.toLowerCase().trim();
			return q === '' || a.nama.toLowerCase().includes(q) || a.nim.toLowerCase().includes(q);
		})
	);

	let showScrollTop = $state(false);

	function scrollToTop() {
		window.scrollTo({ top: 0, behavior: 'smooth' });
	}

	let showAsistenModal = $state(false);
	let showAmpuanModal = $state(false);

	let editId = $state<number | null>(null);
	let form = $state({ nim: '', nama: '', nomor_hp: '', medsos_link: '', foto_url: '', password: '' });
	let ampuanForm = $state({ asisten_id: 0, kelas_id: 0, kelompok: '' });

	async function load() {
		try {
			list = (await api.get<User[]>('/api/admin/asisten')) ?? [];
			kelasList = (await api.get<Kelas[]>('/api/admin/kelas')) ?? [];
			ampuanList = (await api.get<AmpuanKelompok[]>('/api/admin/ampuan')) ?? [];
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

	function reset() { editId = null; form = { nim: '', nama: '', nomor_hp: '', medsos_link: '', foto_url: '', password: '' }; }
	
	function openAddAsisten() {
		reset();
		showAsistenModal = true;
	}

	function edit(a: User) {
		editId = a.id;
		form = { nim: a.nim, nama: a.nama, nomor_hp: a.nomor_hp ?? '', medsos_link: a.medsos_link ?? '', foto_url: a.foto_url ?? '', password: '' };
		showAsistenModal = true;
	}

	async function uploadFoto(ev: Event) {
		const input = ev.target as HTMLInputElement;
		if (!input.files?.[0]) return;
		const fd = new FormData();
		fd.append('file', input.files[0]); fd.append('folder', 'asisten');
		try { const res = await api.upload<{ url: string }>('/api/admin/upload', fd); form.foto_url = res.url; }
		catch (e) { err = (e as Error).message; }
	}

	async function save() {
		err = ''; msg = '';
		const body: Record<string, unknown> = {
			nim: form.nim, nama: form.nama,
			nomor_hp: form.nomor_hp || null, medsos_link: form.medsos_link || null, foto_url: form.foto_url || null
		};
		if (form.password) body.password = form.password;
		try {
			if (editId) await api.put(`/api/admin/asisten/${editId}`, body);
			else await api.post('/api/admin/asisten', body);
			msg = 'Tersimpan.';
			showAsistenModal = false;
			reset();
			await load();
		} catch (e) { err = (e as Error).message; }
	}

	function openAddAmpuan() {
		ampuanForm = { asisten_id: list[0]?.id ?? 0, kelas_id: kelasList[0]?.id ?? 0, kelompok: '' };
		showAmpuanModal = true;
	}

	async function addAmpuan() {
		err = ''; msg = '';
		try {
			await api.post('/api/admin/ampuan', ampuanForm);
			msg = 'Ampuan ditambahkan.';
			showAmpuanModal = false;
			await load();
		} catch (e) { err = (e as Error).message; }
	}

	async function delAmpuan(id: number) {
		if (!await confirmAction({
			title: 'Hapus Ampuan Kelompok?',
			message: 'Apakah Anda yakin ingin menghapus pembagian kelompok ampuan asisten ini?'
		})) return;
		try { await api.del(`/api/admin/ampuan/${id}`); await load(); }
		catch (e) { err = (e as Error).message; }
	}

	async function delAsisten(a: User) {
		if (!await confirmAction({
			title: 'Hapus Akun Asisten?',
			message: `Akun asisten ${a.nama} (${a.nim}) akan dihapus permanen, beserta seluruh data Ampuan Kelompok miliknya. Tindakan ini tidak bisa dibatalkan.`
		})) return;
		err = ''; msg = '';
		try { await api.del(`/api/admin/users/${a.id}`); msg = `Akun asisten ${a.nama} dihapus.`; await load(); }
		catch (e) { err = (e as Error).message; }
	}
</script>

<div class="space-y-6 text-left">
	<div class="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 pb-4">
		<div>
			<h1 class="text-2xl font-bold text-slate-900 leading-none">Manajemen Asisten Lab</h1>
			<p class="mt-2 text-sm text-ink-caption">Data asisten ini akan tampil di halaman publik /info/asisten.</p>
		</div>
		<button class="btn-primary text-xs flex items-center gap-1.5" onclick={openAddAsisten}>
			<UserPlus size={14} /> Tambah Asisten
		</button>
	</div>

	{#if msg}<p class="rounded-lg bg-state-success-bg p-3 text-sm text-state-success">{msg}</p>{/if}
	{#if err}<p class="rounded-lg bg-state-error-bg p-3 text-sm text-state-error">{err}</p>{/if}

	<!-- Full Width Assistant List Card -->
	<div class="card w-full">
		<div class="flex flex-wrap items-center justify-between gap-4 mb-4">
			<h2 class="text-lg font-bold text-slate-800">Daftar Profil Asisten</h2>
			<div class="relative w-full sm:w-64">
				<Search size={14} class="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
				<input type="text" placeholder="Cari Nama atau NIM..." bind:value={searchAsisten} class="input pl-9 w-full text-xs border border-slate-200" />
			</div>
		</div>
		<div class="table-wrap">
			<table class="tbl w-full">
				<thead>
					<tr><th>Foto</th><th>Nama</th><th>NIM</th><th>HP &amp; WA</th><th>Media Sosial / LinkedIn</th><th>Aksi</th></tr>
				</thead>
				<tbody>
					{#each filteredAsisten as a}
						<tr>
							<td>
								{#if a.foto_url}
									<img src={a.foto_url} alt="" class="h-10 w-10 rounded-full object-cover border border-slate-200 shadow-sm" />
								{:else}
									<div class="h-10 w-10 rounded-full bg-slate-100 border border-slate-200 flex items-center justify-center text-xs font-bold text-slate-400">AP</div>
								{/if}
							</td>
							<td class="font-bold text-slate-800">{a.nama}</td>
							<td class="font-mono text-xs font-semibold text-slate-500">{a.nim}</td>
							<td class="font-semibold text-slate-700">{a.nomor_hp ?? '-'}</td>
							<td>
								{#if a.medsos_link}
									<a href={a.medsos_link} target="_blank" rel="noopener" class="text-xs text-primary hover:underline font-semibold break-all">{a.medsos_link}</a>
								{:else}
									<span class="text-xs text-slate-400 font-semibold">—</span>
								{/if}
							</td>
							<td>
								<div class="flex flex-wrap items-center gap-1.5">
									<button class="inline-flex items-center gap-1 bg-primary/10 hover:bg-primary hover:text-white text-primary px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95" onclick={() => edit(a)}>
										<Edit size={12} /> Edit
									</button>
									{#if canDeleteAsisten}
										<button class="inline-flex items-center gap-1 bg-red-50 hover:bg-red-650 hover:text-white text-red-650 px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95 border border-red-100" onclick={() => delAsisten(a)}>
											<Trash2 size={12} /> Hapus Akun Asisten
										</button>
									{/if}
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>

	<!-- Ampuan Section with Add Button -->
	<div class="border-t border-slate-200 pt-6 space-y-4">
		<div class="flex flex-wrap items-center justify-between gap-3">
			<div>
				<h2 class="text-xl font-bold text-slate-900 leading-none">Ampuan Kelompok</h2>
				<p class="mt-1.5 text-xs text-ink-caption">Assign asisten sebagai pengampu kelompok praktikan di setiap kelas.</p>
			</div>
			<button class="btn-outline text-xs flex items-center gap-1.5 hover:bg-slate-50" onclick={openAddAmpuan}>
				<Plus size={14} /> Tambah Ampuan
			</button>
		</div>

		<div class="card w-full">
			<div class="table-wrap">
				<table class="tbl w-full">
					<thead>
						<tr><th>Asisten</th><th>Kelas</th><th>Kelompok</th><th>Aksi</th></tr>
					</thead>
					<tbody>
						{#each ampuanList as a}
							<tr>
								<td class="font-bold text-slate-800">{a.asisten?.nama ?? a.asisten_id}</td>
								<td><span class="badge bg-slate-50 border border-slate-200 font-bold">{a.kelas?.nama_kelas ?? a.kelas_id}</span></td>
								<td><span class="badge bg-primary/5 text-primary border border-primary/20 font-extrabold">Kelompok {a.kelompok}</span></td>
								<td>
									<button class="inline-flex items-center gap-1 bg-red-50 hover:bg-red-650 hover:text-white text-red-650 px-2.5 py-1.5 rounded-lg text-xs font-bold transition-all shadow-sm active:scale-95 border border-red-100" onclick={() => delAmpuan(a.id)}>
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

<!-- Modal Form Asisten -->
{#if showAsistenModal}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/50 p-4 backdrop-blur-sm"
		role="presentation"
		onclick={(e) => { if (e.target === e.currentTarget) showAsistenModal = false; }}
	>
		<div class="w-full max-w-lg rounded-2xl bg-white shadow-2xl text-left" role="dialog" aria-modal="true">
			<div class="flex items-center justify-between border-b border-gray-200 px-6 py-4">
				<h3 class="text-lg font-bold text-slate-900">{editId ? 'Edit' : 'Tambah'} Asisten</h3>
				<button class="text-slate-400 hover:text-slate-700" aria-label="Tutup" onclick={() => (showAsistenModal = false)}><X size={20} /></button>
			</div>
			
			<div class="px-6 py-4 space-y-3.5">
				<div>
					<label class="label font-semibold text-slate-650" for="nim">NIM</label>
					<input id="nim" class="input mt-1 w-full" bind:value={form.nim} placeholder="Ketik NIM..." />
				</div>
				<div>
					<label class="label font-semibold text-slate-650" for="nama">Nama</label>
					<input id="nama" class="input mt-1 w-full" bind:value={form.nama} placeholder="Ketik Nama Lengkap..." />
				</div>
				<div>
					<label class="label font-semibold text-slate-650" for="hp">Nomor HP / WhatsApp</label>
					<input id="hp" class="input mt-1 w-full" bind:value={form.nomor_hp} placeholder="mis. 08123456789" />
				</div>
				<div>
					<label class="label font-semibold text-slate-650" for="ms">Link Media Sosial / LinkedIn</label>
					<input id="ms" class="input mt-1 w-full" bind:value={form.medsos_link} placeholder="https://..." />
				</div>
				<div>
					<label class="label font-semibold text-slate-650" for="foto">Foto Profil</label>
					{#if form.foto_url}
						<img src={form.foto_url} alt="foto" class="mb-2 h-16 w-16 rounded-full object-cover border" />
					{/if}
					<input id="foto" type="file" accept="image/*" onchange={uploadFoto} class="block w-full text-sm file:mr-4 file:rounded-lg file:border-0 file:bg-primary file:px-4 file:py-2 file:text-sm file:font-semibold file:text-white hover:file:bg-primary/90" />
				</div>
				<div>
					<label class="label font-semibold text-slate-650" for="pw">Password {editId ? '(kosongkan jika tidak diubah)' : ''}</label>
					<input id="pw" type="password" class="input mt-1 w-full" bind:value={form.password} placeholder="Ketik Password..." />
				</div>
			</div>
			
			<div class="flex justify-end gap-2 border-t border-gray-200 px-6 py-4">
				<button class="btn-outline px-4 py-2 font-bold" onclick={() => (showAsistenModal = false)}>Batal</button>
				<button class="btn-primary px-5 py-2" onclick={save}>Simpan</button>
			</div>
		</div>
	</div>
{/if}

<!-- Modal Form Ampuan Kelompok -->
{#if showAmpuanModal}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/50 p-4 backdrop-blur-sm"
		role="presentation"
		onclick={(e) => { if (e.target === e.currentTarget) showAmpuanModal = false; }}
	>
		<div class="w-full max-w-lg rounded-2xl bg-white shadow-2xl text-left" role="dialog" aria-modal="true">
			<div class="flex items-center justify-between border-b border-gray-200 px-6 py-4">
				<h3 class="text-lg font-bold text-slate-900">Tambah Ampuan Kelompok</h3>
				<button class="text-slate-400 hover:text-slate-700" aria-label="Tutup" onclick={() => (showAmpuanModal = false)}><X size={20} /></button>
			</div>
			
			<div class="px-6 py-4 space-y-3.5">
				<div>
					<label class="label font-semibold text-slate-650" for="amp-as">Asisten Lab</label>
					<select id="amp-as" class="input mt-1 w-full bg-white border border-slate-200" bind:value={ampuanForm.asisten_id}>
						<option value={0}>— Pilih Asisten —</option>
						{#each list as a}<option value={a.id}>{a.nama}</option>{/each}
					</select>
				</div>
				<div>
					<label class="label font-semibold text-slate-650" for="amp-kl">Kelas</label>
					<select id="amp-kl" class="input mt-1 w-full bg-white border border-slate-200" bind:value={ampuanForm.kelas_id}>
						<option value={0}>— Pilih Kelas —</option>
						{#each kelasList as k}<option value={k.id}>{k.nama_kelas}</option>{/each}
					</select>
				</div>
				<div>
					<label class="label font-semibold text-slate-650" for="amp-kel">Kelompok</label>
					<input id="amp-kel" class="input mt-1 w-full" bind:value={ampuanForm.kelompok} placeholder="mis. A, B, 1" />
				</div>
			</div>
			
			<div class="flex justify-end gap-2 border-t border-gray-200 px-6 py-4">
				<button class="btn-outline px-4 py-2 font-bold" onclick={() => (showAmpuanModal = false)}>Batal</button>
				<button class="btn-primary px-5 py-2" onclick={addAmpuan}>Tambah</button>
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
