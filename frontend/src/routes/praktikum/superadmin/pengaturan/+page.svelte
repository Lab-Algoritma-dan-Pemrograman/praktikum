<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';

	const RESOURCES: { key: string; label: string; desc: string }[] = [
		{ key: 'users:write', label: 'Users (mahasiswa)', desc: 'Kelola data mahasiswa & bulk import' },
		{ key: 'kelas:write', label: 'Kelas', desc: 'Kelola kelas & buka/tutup register' },
		{ key: 'jadwal:write', label: 'Jadwal', desc: 'Kelola jadwal praktikum' },
		{ key: 'asisten:write', label: 'Asisten', desc: 'Kelola akun asisten' },
		{ key: 'sesi:write', label: 'Sesi / Course / Soal', desc: 'Kelola sesi, course, dan soal' },
		{ key: 'aktivasi:write', label: 'Aktivasi', desc: 'Aktivasi sesi & token' },
		{ key: 'penilaian:write', label: 'Penilaian', desc: 'Nilai, keaktifan, rekap jawaban' },
		{ key: 'konfigurasi:write', label: 'Konfigurasi', desc: 'Konfigurasi global & lobby' }
	];

	let perms = $state<Record<string, Record<string, boolean>>>({});
	let loading = $state(true);
	let saving = $state(false);
	let msg = $state('');

	onMount(async () => {
		try {
			const data = await api.get<Record<string, Record<string, boolean>>>('/api/superadmin/permissions');
			perms = data ?? {};
		} catch { perms = {}; }
		loading = false;
	});

	function isEnabled(key: string): boolean {
		// default true jika belum ada entry (fail-open sebelum toggle)
		const row = perms['asisten'];
		if (!row) return true;
		const v = row[key];
		if (v === undefined) return true;
		return v;
	}

	function toggle(key: string) {
		const cur = isEnabled(key);
		const next = { ...perms };
		if (!next['asisten']) next['asisten'] = {};
		next['asisten'] = { ...next['asisten'], [key]: !cur };
		perms = next;
	}

	async function save() {
		saving = true;
		msg = '';
		try {
			await api.put('/api/superadmin/permissions', perms);
			msg = 'Tersimpan.';
		} catch (e) { msg = (e as Error).message; }
		saving = false;
		setTimeout(() => (msg = ''), 3000);
	}
</script>

<div class="max-w-3xl space-y-6">
	<div>
		<h1 class="text-xl font-black text-slate-900">Kontrol Akses Role</h1>
		<p class="text-sm text-slate-500 mt-1">Superadmin — matikan/nyalakan hak tulis role <code>admin</code> per resource. Nonaktif = request ditolak 403.</p>
		<p class="text-xs text-amber-600 mt-1">Halaman hidden — hanya bisa diakses langsung via URL ini.</p>
	</div>

	{#if loading}
		<p class="text-sm text-slate-500">Memuat…</p>
	{:else}
		<div class="bg-white rounded-2xl border border-slate-200 divide-y">
			{#each RESOURCES as r}
				{@const on = isEnabled(r.key)}
				<label class="flex items-center justify-between p-4 gap-4 cursor-pointer hover:bg-slate-50">
					<div>
						<p class="text-sm font-bold text-slate-800">{r.label}</p>
						<p class="text-xs text-slate-500">{r.desc} — <code class="text-[11px] bg-slate-100 px-1 py-0.5 rounded">{r.key}</code></p>
					</div>
					<button type="button" onclick={() => toggle(r.key)} class="relative w-11 h-6 rounded-full transition-colors {on ? 'bg-emerald-500' : 'bg-slate-300'} shrink-0">
						<span class="absolute top-0.5 left-0.5 w-5 h-5 bg-white rounded-full shadow transition-transform {on ? 'translate-x-5' : 'translate-x-0'}"></span>
					</button>
				</label>
			{/each}
		</div>

		<div class="flex items-center gap-3">
			<button onclick={save} disabled={saving} class="px-5 py-2.5 rounded-xl bg-[#8A1538] text-white text-sm font-bold disabled:opacity-50">
				{saving ? 'Menyimpan…' : 'Simpan'}
			</button>
			{#if msg}<span class="text-sm {msg === 'Tersimpan.' ? 'text-emerald-600' : 'text-red-600'}">{msg}</span>{/if}
		</div>

		<p class="text-xs text-slate-400">Key disimpan di tabel <code>konfigurasi</code> dengan key <code>role_permissions</code> sebagai JSON. Superadmin selalu bypass.</p>
	{/if}
</div>
