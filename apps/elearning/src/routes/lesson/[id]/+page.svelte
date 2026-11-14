<script lang="ts">
  import { page } from '$app/state';
  import type { ApplicationLink } from '@algohub/auth-client';
  import LessonView from '../../../lib/LessonView.svelte';
  const id = $derived(page.params.id);
</script>

<svelte:head><title>Pelajaran — E-learning</title></svelte:head>
{#if id}{#key id}<LessonView
      {id}
      runtimeOrigin={page.data.applications?.find(
        (app: ApplicationLink) => app.name === 'portal'
      )?.origin ?? ''}
    />{/key}{/if}
