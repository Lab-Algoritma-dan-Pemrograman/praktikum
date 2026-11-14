<script lang="ts">
  import { page } from '$app/state';
  import type { ApplicationLink } from '@algohub/auth-client';
  import AttemptWorkspace from '../../../lib/AttemptWorkspace.svelte';
  const id = $derived(page.params.id);
</script>

<svelte:head><title>Pengerjaan — Praktikum</title></svelte:head>
{#if id}{#key id}<AttemptWorkspace
      {id}
      runtimeOrigin={page.data.applications?.find(
        (app: ApplicationLink) => app.name === 'portal'
      )?.origin ?? ''}
    />{/key}{/if}
