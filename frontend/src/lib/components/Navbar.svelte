<script lang="ts">
	import { page } from '$app/stores';
	import { LogIn, ExternalLink } from 'lucide-svelte';

	let open = $state(false);

	const links = [
		{ href: '/info', label: 'Beranda' },
		{ href: '/info/jadwal', label: 'Jadwal' },
		{ href: '/info/asisten', label: 'Asisten Lab' },
		{ href: '/info/laporan', label: 'Pedoman' },
		{ href: '/info/modul', label: 'Modul' }
	];

	function active(href: string): boolean {
		return $page.url.pathname === href;
	}
</script>

<header class="fixed top-4 left-0 right-0 z-50 px-4 flex justify-center">
	<nav class="bg-white/90 border border-slate-200/80 shadow-md rounded-2xl px-5 py-2 flex items-center justify-between w-full max-w-6xl transition-all duration-300 backdrop-blur-md">
		
		<!-- Logo — wider, taller -->
		<a href="/info" class="flex items-center gap-3 group text-primary shrink-0">
			<img src="/logo_new.png" alt="Logo Lab AP" class="h-11 w-auto object-contain" style="max-width: 160px;" />
		</a>

		<!-- Mobile Menu Button -->
		<button class="md:hidden w-9 h-9 bg-slate-50 rounded-lg border border-slate-200 flex items-center justify-center text-slate-700 hover:bg-slate-100 hover:text-slate-900 transition-colors focus:outline-none" onclick={() => (open = !open)} aria-label="Menu">
			<svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
			</svg>
		</button>

		<!-- Desktop Links -->
		<div class="hidden md:flex items-center gap-1">
			{#each links as l}
				{#if active(l.href)}
					<a href={l.href} class="px-4 py-2 rounded-xl text-sm font-bold text-primary bg-primary/10 border border-primary/20 shadow-sm transition-all">{l.label}</a>
				{:else}
					<a href={l.href} class="px-4 py-2 rounded-xl text-sm font-semibold text-slate-700 hover:text-slate-900 hover:bg-slate-100 transition-all">{l.label}</a>
				{/if}
			{/each}
			
			<div class="w-px h-5 bg-slate-200 mx-2"></div>
			
			<div class="flex items-center gap-1.5">
				<a href="https://siakad.algohub.web.id" target="_blank" rel="noopener" class="inline-flex items-center gap-1.5 bg-white text-primary border border-primary/25 hover:bg-primary hover:text-white px-4 py-2.5 rounded-xl text-sm font-bold transition-all">
					SIAKAD <ExternalLink size={14} />
				</a>
				<a href="https://elearning.algohub.web.id" target="_blank" rel="noopener" class="inline-flex items-center gap-1.5 bg-white text-primary border border-primary/25 hover:bg-primary hover:text-white px-4 py-2.5 rounded-xl text-sm font-bold transition-all">
					E-Learning <ExternalLink size={14} />
				</a>
				<a href="/praktikum/login" class="inline-flex items-center gap-1.5 bg-primary hover:bg-primary-hover text-white px-5 py-2.5 rounded-xl text-sm font-bold shadow-sm transition-all">
					Login Praktikum <LogIn size={14} />
				</a>
			</div>
		</div>
	</nav>
</header>

{#if open}
	<div class="fixed top-20 left-4 right-4 z-50 rounded-2xl border border-slate-200 bg-white p-4 shadow-lg md:hidden">
		<div class="flex flex-col gap-1.5 text-left">
			{#each links as l}
				{#if active(l.href)}
					<a href={l.href} class="block rounded-lg px-4 py-2 text-sm font-bold text-primary bg-primary/10 transition-colors" onclick={() => (open = false)}>{l.label}</a>
				{:else}
					<a href={l.href} class="block rounded-lg px-4 py-2 text-sm font-semibold text-slate-800 hover:bg-slate-50 hover:text-slate-900 transition-colors" onclick={() => (open = false)}>{l.label}</a>
				{/if}
			{/each}
			<hr class="border-slate-100 my-1.5" />
			<a href="https://siakad.algohub.web.id" target="_blank" rel="noopener" class="block text-center rounded-lg bg-white text-primary border border-primary/25 px-4 py-2.5 text-sm font-bold" onclick={() => (open = false)}>SIAKAD Lab</a>
			<a href="https://elearning.algohub.web.id" target="_blank" rel="noopener" class="block text-center rounded-lg bg-white text-primary border border-primary/25 px-4 py-2.5 text-sm font-bold" onclick={() => (open = false)}>E-Learning Lab</a>
			<a href="/praktikum/login" class="block text-center rounded-lg bg-primary hover:bg-primary-hover text-white px-4 py-2.5 text-sm font-bold shadow-sm" onclick={() => (open = false)}>Login Praktikum</a>
		</div>
	</div>
{/if}
