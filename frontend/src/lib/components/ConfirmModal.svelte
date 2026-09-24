<script lang="ts">
	import { confirmStore } from '$lib/stores/confirm';
	import { escapeHtml } from '$lib/sanitize';
	import { AlertTriangle, Power } from 'lucide-svelte';

	let state = $derived($confirmStore);

	// CR-H2: pesan konfirmasi adalah teks polos → escape penuh dulu,
	// baru bungkus `backtick` jadi badge. Tanpa escape, pesan berisi HTML
	// (mis. nama dari input user) bisa disuntikkan sebagai markup.
	let formattedMessage = $derived(
		escapeHtml(state.message).replace(
			/`([^`]+)`/g,
			'<span class="px-2 py-0.5 rounded bg-slate-800 border border-slate-700 text-slate-100 text-xs font-mono font-bold">$1</span>'
		)
	);

	function cancel() {
		if (state.resolve) state.resolve(false);
		confirmStore.update(s => ({ ...s, show: false }));
	}

	function confirm() {
		if (state.resolve) state.resolve(true);
		confirmStore.update(s => ({ ...s, show: false }));
	}
</script>

{#if state.show}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 backdrop-blur-sm p-4"
		role="presentation"
		onclick={(e) => { if (e.target === e.currentTarget) cancel(); }}
	>
		<div class="relative w-full max-w-sm bg-[#111827] border border-slate-800 text-white rounded-3xl p-6 sm:p-7 text-center shadow-2xl mt-6" role="dialog" aria-modal="true">
			<!-- Floating Icon on Top -->
			<div class="absolute -top-7 left-1/2 -translate-x-1/2 w-14 h-14 rounded-full bg-[#111827] border-4 border-slate-950 shadow-md flex items-center justify-center">
				<div class="w-9 h-9 rounded-full bg-amber-500/10 border border-amber-500/30 flex items-center justify-center text-amber-500">
					<AlertTriangle size={18} class="fill-amber-500/10" />
				</div>
			</div>

			<!-- Content -->
			<div class="mt-6 space-y-3">
				<h3 class="text-lg font-extrabold text-white tracking-tight">{state.title}</h3>
				<p class="text-sm text-slate-350 leading-relaxed max-w-xs mx-auto">
					{@html formattedMessage}
				</p>
			</div>

			<!-- Actions -->
			<div class="flex gap-3 mt-7">
				<button
					class="flex-1 py-3 text-sm font-semibold rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700/80 transition-all duration-200 active:scale-95"
					onclick={cancel}
				>
					{state.cancelText}
				</button>
				<button
					class="flex-1 py-3 text-sm font-bold rounded-xl text-white shadow-md active:scale-95 transition-all duration-200 flex items-center justify-center gap-1.5
					{state.danger ? 'bg-rose-600 hover:bg-rose-700' : 'bg-primary hover:bg-primary-hover'}"
					onclick={confirm}
				>
					<Power size={14} />
					{state.confirmText}
				</button>
			</div>
		</div>
	</div>
{/if}
