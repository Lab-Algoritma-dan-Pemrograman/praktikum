<script lang="ts">
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { setAuth } from '$lib/stores/auth';
	import type { AuthResponse } from '$lib/types';
	import { User, Mail, Lock, AlertCircle, ArrowLeft } from 'lucide-svelte';

	// Roster-gated: NIM harus sudah ditanam asisten (bulk import / manual).
	// Nama & kelas selalu dari roster — tidak dikirim dari sini.
	let nim = $state('');
	let email = $state('');
	let password = $state('');
	let passwordConfirm = $state('');
	let err = $state('');
	let loading = $state(false);

	function redirectByRole(role: string) {
		goto(role === 'asisten' || role === 'koordinator' ? '/praktikum/admin' : '/praktikum/dashboard');
	}

	function validateEmail(): string | null {
		const e = email.trim().toLowerCase();
		if (!e.includes('@')) return 'Email tidak valid.';
		const [local, domain] = e.split('@');
		if (!local || !domain) return 'Email tidak valid.';
		if (local.includes('+')) return "Karakter '+' tidak diizinkan pada email.";
		if (domain === 'gmail.com' && local.includes('.')) return "Karakter '.' tidak diizinkan pada email Gmail.";
		const allowed = ['gmail.com', 'yahoo.com', 'outlook.com', 'hotmail.com', 'itpln.ac.id'];
		if (!allowed.includes(domain)) return 'Domain email tidak diizinkan.';
		return null;
	}

	async function doRegister() {
		err = '';
		if (!nim.trim()) { err = 'NIM wajib diisi.'; return; }
		if (password.length < 6) { err = 'Password minimal 6 karakter.'; return; }
		if (password !== passwordConfirm) { err = 'Konfirmasi password tidak cocok.'; return; }
		const v = validateEmail();
		if (v) { err = v; return; }
		loading = true;
		try {
			// Backend langsung balas token — register = langsung masuk (tanpa verifikasi email).
			const res = await api.post<AuthResponse>('/api/auth/register', {
				nim: nim.trim(),
				email: email.trim().toLowerCase(),
				password
			});
			setAuth(res.token, res.user);
			redirectByRole(res.user.role);
		} catch (e) {
			err = (e as Error).message;
		} finally {
			loading = false;
		}
	}
</script>

<div class="relative flex min-h-screen items-center justify-center p-4 overflow-hidden"
	style="background: url('/bg_login.jpg') no-repeat center center; background-size: cover;">

	<!-- Cinematic overlay to make the background photo subtle and text readable -->
	<div class="absolute inset-0 bg-slate-950/65 backdrop-blur-[2px] z-0"></div>

	<!-- Register Card Container -->
	<div class="relative z-10 w-full max-w-md bg-white border border-rose-100/30 rounded-[2.5rem] p-8 sm:p-10 shadow-[0_25px_60px_-15px_rgba(0,0,0,0.4)] text-center">

		<!-- Top border gradient highlight matching the logo maroon -->
		<div class="absolute top-0 left-12 right-12 h-1 bg-gradient-to-r from-transparent via-[#8A1538] to-transparent rounded-t-[2.5rem]"></div>

		<!-- Logo & Brand -->
		<div class="mb-6">
			<div class="mx-auto mb-4 w-16 h-16 bg-gradient-to-br from-rose-50 to-rose-100 border border-rose-200/40 rounded-2xl flex items-center justify-center shadow-sm group">
				<img src="/logo.png" alt="Logo Lab AP" class="h-10 w-10 object-contain transition-transform duration-500 group-hover:scale-110" />
			</div>
			<h1 class="text-2xl font-black tracking-tight text-slate-900">Daftar Akun</h1>
			<p class="mt-1 text-xs font-semibold text-slate-500 leading-relaxed">Gunakan NIM yang sudah terdaftar di lab — nama dan kelas ditetapkan asisten.</p>
		</div>

		{#if err}
			<div class="mb-5 rounded-2xl border border-red-100 bg-red-50 p-4 text-xs font-semibold text-red-700 shadow-sm flex items-start gap-2.5 text-left animate-shake">
				<AlertCircle class="shrink-0 text-red-500 mt-0.5" size={15} />
				<span>{err}</span>
			</div>
		{/if}

		<form onsubmit={(e) => { e.preventDefault(); doRegister(); }} class="space-y-4 text-left">
			<div>
				<label class="block text-xs font-bold text-slate-600 uppercase tracking-wider mb-2" for="nim">Nomor Induk Mahasiswa</label>
				<div class="relative">
					<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-4 text-slate-400">
						<User size={15} />
					</div>
					<input id="nim" class="w-full h-12 pl-10 pr-4 bg-slate-50/80 border border-slate-200 rounded-2xl text-sm font-semibold text-slate-800 placeholder-slate-400/50 focus:outline-none focus:border-[#8A1538]/50 focus:bg-white transition-all" bind:value={nim} placeholder="Masukkan NIM Anda" required autocomplete="off" />
				</div>
			</div>
			<div>
				<label class="block text-xs font-bold text-slate-600 uppercase tracking-wider mb-2" for="email">Email</label>
				<div class="relative">
					<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-4 text-slate-400">
						<Mail size={15} />
					</div>
					<input id="email" type="email" class="w-full h-12 pl-10 pr-4 bg-slate-50/80 border border-slate-200 rounded-2xl text-sm font-semibold text-slate-800 placeholder-slate-400/50 focus:outline-none focus:border-[#8A1538]/50 focus:bg-white transition-all" bind:value={email} placeholder="nama@gmail.com" required />
				</div>
				<p class="mt-1.5 text-[10px] text-slate-400">Hanya gmail.com, yahoo.com, outlook.com, hotmail.com, itpln.ac.id — tanpa '+' atau '.' di Gmail.</p>
			</div>
			<div>
				<label class="block text-xs font-bold text-slate-600 uppercase tracking-wider mb-2" for="pw1">Buat Password Baru</label>
				<div class="relative">
					<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-4 text-slate-400">
						<Lock size={15} />
					</div>
					<input id="pw1" type="password" class="w-full h-12 pl-10 pr-4 bg-slate-50/80 border border-slate-200 rounded-2xl text-sm font-semibold text-slate-800 placeholder-slate-400/50 focus:outline-none focus:border-[#8A1538]/50 focus:bg-white transition-all" bind:value={password} placeholder="Minimal 6 karakter" required minlength={6} autocomplete="new-password" />
				</div>
			</div>
			<div>
				<label class="block text-xs font-bold text-slate-600 uppercase tracking-wider mb-2" for="pw2">Konfirmasi Password</label>
				<div class="relative">
					<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-4 text-slate-400">
						<Lock size={15} />
					</div>
					<input id="pw2" type="password" class="w-full h-12 pl-10 pr-4 bg-slate-50/80 border border-slate-200 rounded-2xl text-sm font-semibold text-slate-800 placeholder-slate-400/50 focus:outline-none focus:border-[#8A1538]/50 focus:bg-white transition-all" bind:value={passwordConfirm} placeholder="Ketik ulang password baru Anda" required autocomplete="new-password" />
				</div>
				{#if passwordConfirm && password !== passwordConfirm}
					<p class="mt-1.5 text-[10px] font-bold text-red-500" role="alert">Password belum sama</p>
				{/if}
			</div>
			<button class="w-full h-12 bg-[#8A1538] hover:bg-[#730d2d] text-white rounded-2xl text-sm font-black flex items-center justify-center gap-2 shadow-lg shadow-[#8A1538]/10 transition-all active:scale-[0.99]" disabled={loading}>
				{loading ? 'Mendaftarkan Akun…' : 'Daftar & Masuk'}
			</button>
		</form>

		<!-- Login link — selalu terlihat -->
		<p class="mt-5 text-xs font-bold text-slate-500">
			Sudah punya akun?
			<a href="/praktikum/login" class="text-[#8A1538] hover:text-[#610a24] transition-colors">Login di sini</a>
		</p>

		<!-- Bottom Back Link -->
		<div class="pt-6 border-t border-slate-100 mt-6 text-center">
			<a href="/info" class="inline-flex items-center gap-1.5 text-xs font-bold text-slate-400 hover:text-primary transition-colors">
				<ArrowLeft size={12} /> Kembali ke portal utama
			</a>
		</div>

	</div>
</div>
