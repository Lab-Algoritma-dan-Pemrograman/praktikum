<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { fly } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';
	import { api } from '$lib/api';
	import { user, clearAuth } from '$lib/stores/auth';
	import { onMount } from 'svelte';
	import ConfirmModal from '$lib/components/ConfirmModal.svelte';

	import {
		Home, BookOpen, User, BarChart2, Users, GraduationCap, School,
		Calendar, FileText, Book, Puzzle, Zap, CheckCircle, ClipboardList, Trophy,
		LogOut, Menu, X, Settings, Terminal, Key, Globe
	} from 'lucide-svelte';

	let { children } = $props();

	interface NavLink { href: string; label: string; icon: any; }

	const userLinks: NavLink[] = [
		{ href: '/praktikum/dashboard', label: 'Dashboard', icon: Home },
		{ href: '/praktikum/sesi', label: 'Daftar Sesi', icon: BookOpen },
		{ href: '/praktikum/profil', label: 'Profil', icon: User }
	];
	const adminLinks: NavLink[] = [
		{ href: '/praktikum/admin', label: 'Dashboard', icon: BarChart2 },
		{ href: '/praktikum/admin/users', label: 'Data User', icon: Users },
		{ href: '/praktikum/admin/asisten', label: 'Asisten', icon: GraduationCap },
		{ href: '/praktikum/admin/kelas', label: 'Kelas', icon: School },
		{ href: '/praktikum/admin/jadwal', label: 'Jadwal', icon: Calendar },
		{ href: '/praktikum/admin/pedoman', label: 'Pedoman', icon: FileText },
		{ href: '/praktikum/admin/modul', label: 'Modul', icon: Book },
		{ href: '/praktikum/admin/sesi', label: 'Sesi & Soal', icon: Puzzle },
		{ href: '/praktikum/admin/aktivasi', label: 'Aktivasi Sesi', icon: Zap },
		{ href: '/praktikum/admin/penilaian', label: 'Penilaian', icon: CheckCircle },
		{ href: '/praktikum/admin/rekap-jawaban', label: 'Rekap Jawaban', icon: ClipboardList },
		{ href: '/praktikum/admin/rekap-nilai', label: 'Rekap Nilai', icon: Trophy },
		{ href: '/praktikum/admin/log', label: 'Log Aktivitas', icon: Terminal },
		{ href: '/praktikum/admin/pengaturan', label: 'Pengaturan Lobby', icon: Settings }
	];

	let isAdmin = $derived($user?.role === 'asisten' || $user?.role === 'koordinator');
	let links = $derived(isAdmin ? adminLinks : userLinks);

	let open = $state(false);              // mobile off-canvas
	let collapsed = $state(false);         // desktop hide/show (persisted)

	onMount(() => {
		collapsed = localStorage.getItem('sidebar_collapsed') === '1';
	});

	// Satu tombol hamburger: di mobile buka/tutup off-canvas, di desktop (>=768)
	// sembunyikan/tampilkan sidebar + simpan preferensi.
	function toggleSidebar() {
		if (typeof window !== 'undefined' && window.innerWidth >= 768) {
			collapsed = !collapsed;
			localStorage.setItem('sidebar_collapsed', collapsed ? '1' : '0');
		} else {
			open = !open;
		}
	}

	// Saat user di halaman pengerjaan soal, sembunyikan sidebar agar fokus.
	let inSession = $derived(
		/\/praktikum\/sesi\/\d+\/(pretest|posttest|keterampilan|ujian)/.test($page.url.pathname)
	);

	function active(href: string): boolean {
		if (href === '/praktikum/admin' || href === '/praktikum/dashboard') {
			return $page.url.pathname === href;
		}
		return $page.url.pathname === href || $page.url.pathname.startsWith(href + '/');
	}

	function initials(nama?: string): string {
		if (!nama) return '?';
		return nama.trim().split(/\s+/).map((n) => n[0]).join('').substring(0, 2).toUpperCase();
	}

	async function logout() {
		try { await api.post('/api/auth/logout'); } catch { /* ignore */ }
		clearAuth();
		goto('/praktikum/login');
	}
</script>

<div class="dashboard-layout" class:sidebar-collapsed={collapsed}>
	<!-- Sidebar Overlay (mobile) -->
	{#if open && !inSession}
		<button class="sidebar-overlay" onclick={() => (open = false)} aria-label="Tutup menu"></button>
	{/if}

	<!-- Sidebar (hidden saat sesi pengerjaan) -->
	{#if !inSession}
	<aside class="sidebar" class:sidebar-open={open}>
		<div class="sidebar-header">
			<a href="/praktikum/dashboard" class="sidebar-brand">
				<img src="/logo_new.png" alt="Logo Lab AP" class="sidebar-logo" />
				<span>Lab Algoritma</span>
			</a>
			<button class="sidebar-close" onclick={() => (open = false)} aria-label="Tutup Menu">
				<X size={18} />
			</button>
		</div>

		<div class="sidebar-role">
			{#if isAdmin}<Key size={13} /> Admin Panel{:else}<GraduationCap size={13} /> Panel Mahasiswa{/if}
		</div>

		<nav class="sidebar-nav custom-scrollbar">
			{#each links as l}
				{@const Icon = l.icon}
				{@const isActive = active(l.href)}
				<a
					href={l.href}
					onclick={() => (open = false)}
					class="sidebar-link"
					class:active={isActive}
				>
					<span class="link-icon"><Icon size={18} /></span>
					<span class="link-label">{l.label}</span>
				</a>
			{/each}
		</nav>

		<div class="sidebar-footer">
			<div class="sb-user">
				<div class="sb-avatar">{initials($user?.nama)}</div>
				<div class="sb-user-info">
					<span class="sb-user-name">{$user?.nama || 'Pengguna'}</span>
					<span class="sb-user-sub">{$user?.nim || '—'}</span>
				</div>
			</div>
			<a href="/info" class="sidebar-link">
				<span class="link-icon"><Globe size={18} /></span>
				<span class="link-label">Portal Publik</span>
			</a>
			<button class="logout-btn" onclick={logout}>
				<span class="link-icon"><LogOut size={18} /></span>
				<span class="link-label">Logout</span>
			</button>
		</div>
	</aside>
	{/if}

	<!-- Main Area -->
	<div class="main-area">
		<header class="topbar">
			{#if !inSession}
			<button class="hamburger" onclick={toggleSidebar} aria-label="Buka/tutup menu">
				<Menu size={22} />
			</button>
			{/if}
			<a href="/praktikum/profil" class="topbar-user">
				<div class="topbar-avatar">{initials($user?.nama)}</div>
				<div class="topbar-id">
					<span class="topbar-name">{$user?.nama || 'Pengguna'}</span>
					<span class="topbar-role">{$user?.role || ''}</span>
				</div>
			</a>
		</header>
		<main class="content-area">
			{#key $page.url.pathname}
				<div in:fly={{ y: 8, duration: 240, easing: cubicOut }}>
					{@render children()}
				</div>
			{/key}
		</main>
	</div>
</div>
<ConfirmModal />

<style>
	/* Palette Lab-AP maroon dipertahankan; layout/anatomi adaptasi dari Mikon.
	   sidebar #5C0E25 (neo-maroon), primary #8A1538, hover #B21F47. */
	.dashboard-layout { display: flex; min-height: 100vh; background: #f8fafc; }

	/* Sidebar — maroon gelap, teks putih */
	.sidebar {
		width: 250px;
		background: linear-gradient(180deg, #5C0E25 0%, #4A0B1E 100%);
		color: #fff;
		display: flex;
		flex-direction: column;
		position: fixed;
		top: 0; left: 0; bottom: 0;
		z-index: 40;
		transform: translateX(-100%);
		transition: transform 0.25s ease;
	}
	.sidebar-open { transform: translateX(0); }
	@media (min-width: 768px) {
		.sidebar { transform: translateX(0); }
		.sidebar-collapsed .sidebar { transform: translateX(-100%); }
	}

	.sidebar-overlay {
		position: fixed; inset: 0;
		background: rgba(0,0,0,0.45);
		border: none; z-index: 35; cursor: pointer;
	}
	@media (min-width: 768px) { .sidebar-overlay { display: none; } }

	.sidebar-header {
		display: flex; align-items: center; justify-content: space-between;
		padding: 1.1rem 1.25rem 0.85rem;
	}
	.sidebar-brand { display: flex; align-items: center; gap: 0.65rem; color: #fff; text-decoration: none; font-weight: 700; font-size: 0.95rem; }
	.sidebar-brand:visited { color: #fff; }
	.sidebar-logo { width: 2.1rem; height: 2.1rem; flex-shrink: 0; object-fit: contain; background: #fff; border-radius: 0.5rem; padding: 2px; }
	.sidebar-close { background: none; border: none; color: rgba(255,255,255,0.7); cursor: pointer; padding: 0.2rem; }
	.sidebar-close:hover { color: #fff; }
	@media (min-width: 768px) { .sidebar-close { display: none; } }

	.sidebar-role {
		display: flex; align-items: center; gap: 0.35rem;
		padding: 0.3rem 1.25rem 0.9rem;
		font-size: 0.72rem; opacity: 0.75;
		text-transform: uppercase; letter-spacing: 0.06em;
		border-bottom: 1px solid rgba(255,255,255,0.1);
	}

	.sidebar-nav {
		flex: 1; padding: 0.75rem 0.7rem;
		display: flex; flex-direction: column; gap: 0.15rem;
		overflow-y: auto;
	}
	.sidebar-link {
		display: flex; align-items: center; gap: 0.65rem;
		padding: 0.62rem 0.8rem;
		color: rgba(255,255,255,0.78);
		text-decoration: none; border-radius: 0.6rem;
		font-size: 0.875rem; font-weight: 500;
		transition: background 0.15s, color 0.15s;
	}
	.sidebar-link:hover { background: rgba(255,255,255,0.1); color: #fff; }
	.sidebar-link:visited { color: rgba(255,255,255,0.78); }
	.sidebar-link.active {
		background: #8A1538; color: #fff; font-weight: 600;
		box-shadow: 0 2px 8px rgba(138,21,56,0.4);
	}
	.sidebar-link.active:visited { color: #fff; }
	.link-icon { display: flex; align-items: center; justify-content: center; width: 1.25rem; flex-shrink: 0; }
	.link-label { white-space: nowrap; }

	.sidebar-footer {
		padding: 0.7rem;
		border-top: 1px solid rgba(255,255,255,0.1);
		display: flex; flex-direction: column; gap: 0.15rem;
	}
	.sb-user {
		display: flex; align-items: center; gap: 0.6rem;
		padding: 0.5rem 0.6rem; margin-bottom: 0.35rem;
		border-radius: 0.6rem; background: rgba(255,255,255,0.07);
	}
	.sb-avatar {
		width: 2.25rem; height: 2.25rem; flex-shrink: 0;
		border-radius: 50%; background: rgba(255,255,255,0.16);
		color: #fff; display: flex; align-items: center; justify-content: center;
		font-weight: 700; font-size: 0.8rem;
	}
	.sb-user-info { display: flex; flex-direction: column; min-width: 0; }
	.sb-user-name { font-size: 0.85rem; font-weight: 600; color: #fff; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
	.sb-user-sub { font-size: 0.72rem; color: rgba(255,255,255,0.7); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

	.logout-btn {
		display: flex; align-items: center; gap: 0.65rem; width: 100%;
		padding: 0.62rem 0.8rem;
		color: rgba(255,255,255,0.78); background: none; border: none;
		border-radius: 0.6rem; font-size: 0.875rem; font-weight: 500;
		cursor: pointer; text-align: left;
		transition: background 0.15s, color 0.15s;
	}
	.logout-btn:hover { background: rgba(239,68,68,0.22); color: #FCA5A5; }

	/* Main Area */
	.main-area { flex: 1; display: flex; flex-direction: column; min-width: 0; }
	@media (min-width: 768px) {
		.main-area { margin-left: 250px; transition: margin-left 0.25s ease; }
		.sidebar-collapsed .main-area { margin-left: 0; }
	}

	.topbar {
		display: flex; align-items: center; gap: 0.7rem;
		padding: 0.6rem 1rem;
		background: rgba(255,255,255,0.95); backdrop-filter: blur(8px);
		border-bottom: 1px solid #e2e8f0;
		position: sticky; top: 0; z-index: 20; height: 3.75rem;
	}
	.hamburger {
		background: none; border: none; cursor: pointer; padding: 0.3rem;
		color: #475569; border-radius: 0.45rem; transition: background 0.15s, color 0.15s;
		flex-shrink: 0; display: flex;
	}
	.hamburger:hover { background: #f1f5f9; color: #8A1538; }

	.topbar-user { display: flex; align-items: center; gap: 0.6rem; margin-left: auto; min-width: 0; text-decoration: none; padding: 0.25rem 0.5rem; border-radius: 999px; transition: background 0.15s; }
	.topbar-user:hover { background: #FDF2F4; }
	.topbar-avatar {
		width: 2.1rem; height: 2.1rem; flex-shrink: 0; border-radius: 50%;
		background: #8A1538; color: #fff; display: flex; align-items: center; justify-content: center;
		font-weight: 700; font-size: 0.78rem;
	}
	.topbar-id { min-width: 0; display: flex; flex-direction: column; line-height: 1.2; text-align: right; }
	.topbar-name { font-size: 0.85rem; font-weight: 700; color: #0f172a; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
	.topbar-role { font-size: 0.65rem; color: #8A1538; font-weight: 800; text-transform: uppercase; letter-spacing: 0.08em; }
	@media (min-width: 768px) { .topbar { padding: 0.7rem 1.5rem; } }

	.content-area { flex: 1; padding: 1.25rem; overflow-x: hidden; }
	@media (min-width: 768px) { .content-area { padding: 2rem; } }

	@media (prefers-reduced-motion: reduce) {
		.sidebar, .main-area { transition: none; }
	}
</style>
