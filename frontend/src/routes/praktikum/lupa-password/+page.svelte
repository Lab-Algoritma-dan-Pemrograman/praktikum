<script lang="ts">
	import { api } from '$lib/api';
	import { Mail, AlertCircle, ArrowLeft, ArrowRight } from 'lucide-svelte';

	let email = $state('');
	let err = $state('');
	let loading = $state(false);
	let sent = $state(false);
	// Akun ada tapi belum punya email/kanal OTP: jangan tampilkan "kode sudah
	// dikirim". Sebelumnya kondisi ini tak terlihat dan pengguna menunggu kode
	// yang tak akan pernah datang.
	let pesanKirim = $state('');

	async function kirim() {
		if (!email.trim() || !/^\S+@\S+\.\S+$/.test(email.trim())) {
			err = 'Masukkan email yang valid';
			return;
		}
		err = '';
		loading = true;
		try {
			// Backend selalu 200 (anti-enumeration) tetapi membawa status pengiriman.
			const hasil = await api.post<{ sent: boolean; reason?: string }>('/api/auth/forgot-password', {
				email: email.trim()
			});
			if (hasil && !hasil.sent) {
				err =
					hasil.reason === 'no_email_channel'
						? 'Akun ini belum punya email untuk menerima kode OTP. Isi email dulu di halaman Profil, lalu coba lagi.'
						: 'Kode OTP gagal dikirim. Coba lagi beberapa saat lagi.';
				return;
			}
			pesanKirim =
				'Jika email tersebut terdaftar, kami sudah mengirim kode OTP 6 digit. Masukkan kode itu di halaman berikutnya. Cek juga folder spam.';
			// Simpan email utk halaman OTP.
			sessionStorage.setItem('reset_email', email.trim());
			sent = true;
		} catch (e) {
			err = (e as Error).message;
		} finally {
			loading = false;
		}
	}
</script>

<div class="relative flex min-h-screen items-center justify-center p-4 overflow-hidden"
	style="background: url('/bg_login.jpg') no-repeat center center; background-size: cover;">
	<div class="absolute inset-0 bg-slate-950/65 backdrop-blur-[2px] z-0"></div>

	<div class="relative z-10 w-full max-w-md bg-white border border-rose-100/30 rounded-[2.5rem] p-8 sm:p-10 shadow-[0_25px_60px_-15px_rgba(0,0,0,0.4)] text-center">
		<div class="absolute top-0 left-12 right-12 h-1 bg-gradient-to-r from-transparent via-[#8A1538] to-transparent rounded-t-[2.5rem]"></div>

		<div class="mb-8">
			<div class="mx-auto mb-4 w-16 h-16 bg-gradient-to-br from-rose-50 to-rose-100 border border-rose-200/40 rounded-2xl flex items-center justify-center shadow-sm">
				<img src="/logo.png" alt="Logo Lab AP" class="h-10 w-10 object-contain" />
			</div>
			<h1 class="text-2xl font-black tracking-tight text-slate-900">Lupa Password</h1>
			<p class="mt-1 text-xs font-bold text-[#8A1538] uppercase tracking-wider">Lab Algoritma &amp; Pemrograman</p>
		</div>

		{#if sent}
			<div class="rounded-2xl border border-emerald-100 bg-emerald-50 p-5 text-left mb-5">
				<h2 class="text-sm font-black text-emerald-800 mb-1.5">Cek Email Anda</h2>
				<p class="text-xs text-emerald-700 leading-relaxed">
					{pesanKirim}
				</p>
			</div>
			<a href="/praktikum/reset-password" class="w-full h-12 bg-[#8A1538] hover:bg-[#730d2d] text-white rounded-2xl text-sm font-black flex items-center justify-center gap-2 shadow-lg shadow-[#8A1538]/10 transition-all">
				Masukkan Kode OTP <ArrowRight size={14} />
			</a>
		{:else}
			{#if err}
				<div class="mb-5 rounded-2xl border border-red-100 bg-red-50 p-4 text-xs font-semibold text-red-700 shadow-sm flex items-start gap-2.5 text-left">
					<AlertCircle class="shrink-0 text-red-500 mt-0.5" size={15} />
					<span>{err}</span>
				</div>
			{/if}

			<p class="mb-5 text-xs text-slate-500 leading-relaxed">Masukkan email akunmu, kami kirim kode OTP untuk reset password.</p>

			<form onsubmit={(e) => { e.preventDefault(); kirim(); }} class="space-y-5 text-left">
				<div>
					<label class="block text-xs font-bold text-slate-600 uppercase tracking-wider mb-2" for="email">Email</label>
					<div class="relative">
						<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-4 text-slate-400">
							<Mail size={15} />
						</div>
						<input id="email" type="email" class="w-full h-12 pl-10 pr-4 bg-slate-50/80 border border-slate-200 rounded-2xl text-sm font-semibold text-slate-800 placeholder-slate-400/50 focus:outline-none focus:border-[#8A1538]/50 focus:bg-white transition-all" bind:value={email} placeholder="nama@email.com" autocomplete="username" required />
					</div>
				</div>
				<button class="w-full h-12 bg-[#8A1538] hover:bg-[#730d2d] text-white rounded-2xl text-sm font-black flex items-center justify-center gap-2 shadow-lg shadow-[#8A1538]/10 transition-all active:scale-[0.99]" disabled={loading}>
					{loading ? 'Mengirim…' : 'Kirim Kode OTP'}
				</button>
			</form>
		{/if}

		<div class="pt-6 border-t border-slate-100 mt-6 text-center">
			<a href="/praktikum/login" class="inline-flex items-center gap-1.5 text-xs font-bold text-slate-400 hover:text-[#8A1538] transition-colors">
				<ArrowLeft size={12} /> Kembali ke Login
			</a>
		</div>
	</div>
</div>
