<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { user } from '$lib/stores/auth';
	import { User, Lock, Camera, Save, AtSign, Phone, Link2 } from 'lucide-svelte';

	let isAdmin = $derived($user?.role === 'asisten');
	let nama = $state('');
	let nomorHp = $state('');
	let medsos = $state('');
	let fotoUrl = $state('');
	let passwordLama = $state('');
	let passwordBaru = $state('');
	let msg = $state('');
	let err = $state('');

	onMount(async () => {
		try {
			const u = await api.get<any>('/api/profile');
			nama = u.nama ?? '';
			nomorHp = u.nomor_hp ?? '';
			medsos = u.medsos_link ?? '';
			fotoUrl = u.foto_url ?? '';
		} catch (e) {
			err = (e as Error).message;
		}
	});

	async function uploadFoto(ev: Event) {
		const input = ev.target as HTMLInputElement;
		if (!input.files?.[0]) return;
		const fd = new FormData();
		fd.append('file', input.files[0]);
		fd.append('folder', 'asisten');
		try {
			const res = await api.upload<{ url: string }>('/api/admin/upload', fd);
			fotoUrl = res.url;
			msg = 'Foto terunggah.';
		} catch (e) {
			err = (e as Error).message;
		}
	}

	async function save() {
		msg = ''; err = '';
		const body: Record<string, unknown> = { nama };
		if (isAdmin) {
			body.nomor_hp = nomorHp;
			body.medsos_link = medsos;
			body.foto_url = fotoUrl;
		}
		if (passwordBaru) {
			body.password_lama = passwordLama;
			body.password_baru = passwordBaru;
		}
		try {
			await api.put('/api/profile', body);
			msg = 'Profil diperbarui.';
			passwordLama = ''; passwordBaru = '';
		} catch (e) {
			err = (e as Error).message;
		}
	}
</script>

<div class="space-y-6 max-w-2xl">
	<!-- Header -->
	<div class="flex items-center gap-3">
		<div class="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10 text-primary">
			<User size={20} />
		</div>
		<div>
			<h1 class="text-2xl font-bold text-ink-heading">Profil Saya</h1>
			<p class="text-sm text-ink-caption">Kelola informasi akun dan keamanan Anda.</p>
		</div>
	</div>

	{#if msg}<p class="rounded-xl bg-state-success-bg p-3 text-sm font-semibold text-state-success">{msg}</p>{/if}
	{#if err}<p class="rounded-xl bg-state-error-bg p-3 text-sm font-semibold text-state-error">{err}</p>{/if}

	<!-- Info Dasar -->
	<div class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm space-y-4">
		<h2 class="text-base font-bold text-ink-heading flex items-center gap-2">
			<User size={16} class="text-primary" /> Informasi Dasar
		</h2>

		<div>
			<label class="label flex items-center gap-1.5" for="nama"><User size={12} class="text-ink-caption" /> Nama</label>
			<input id="nama" class="input" bind:value={nama} />
		</div>

		{#if isAdmin}
			<div>
				<label class="label flex items-center gap-1.5" for="hp"><Phone size={12} class="text-ink-caption" /> Nomor HP/WhatsApp</label>
				<input id="hp" class="input" bind:value={nomorHp} placeholder="08xxx" />
			</div>
			<div>
				<label class="label flex items-center gap-1.5" for="ms"><Link2 size={12} class="text-ink-caption" /> Link Media Sosial / LinkedIn</label>
				<input id="ms" class="input" bind:value={medsos} />
			</div>
			<div>
				<label class="label flex items-center gap-1.5" for="foto"><Camera size={12} class="text-ink-caption" /> Foto Profil</label>
				{#if fotoUrl}
					<img src={fotoUrl} alt="foto" class="mb-2 h-20 w-20 rounded-full object-cover border-2 border-gray-200 shadow-sm" />
				{/if}
				<input id="foto" type="file" accept="image/*" onchange={uploadFoto} class="block w-full text-sm file:mr-4 file:rounded-lg file:border-0 file:bg-primary file:px-4 file:py-2 file:text-sm file:font-semibold file:text-white hover:file:bg-primary/90" />
			</div>
		{/if}
	</div>

	<!-- Ganti Password -->
	<div class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm space-y-4">
		<h2 class="text-base font-bold text-ink-heading flex items-center gap-2">
			<Lock size={16} class="text-primary" /> Ganti Password
		</h2>
		<p class="text-xs text-ink-caption -mt-2">Opsional. Isi jika ingin mengubah password.</p>

		<div>
			<label class="label flex items-center gap-1.5" for="pl"><Lock size={12} class="text-ink-caption" /> Password Lama</label>
			<input id="pl" type="password" class="input" bind:value={passwordLama} />
		</div>
		<div>
			<label class="label flex items-center gap-1.5" for="pb"><Lock size={12} class="text-ink-caption" /> Password Baru</label>
			<input id="pb" type="password" class="input" bind:value={passwordBaru} minlength="6" />
		</div>
	</div>

	<!-- Save Button -->
	<div class="flex justify-end">
		<button class="btn-primary inline-flex items-center gap-2 px-6" onclick={save}>
			<Save size={16} /> Simpan Perubahan
		</button>
	</div>
</div>
