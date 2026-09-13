<script lang="ts">
	import { onMount } from 'svelte';
	import { get } from 'svelte/store';
	import { goto } from '$app/navigation';
	import { user } from '$lib/stores/auth';

	let { children } = $props();
	let ok = $state(false);

	onMount(() => {
		const u = get(user);
		if (!u || u.role !== 'koordinator') {
			goto('/praktikum/dashboard');
			return;
		}
		ok = true;
	});
</script>

{#if ok}{@render children()}{/if}
