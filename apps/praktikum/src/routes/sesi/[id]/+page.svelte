<script lang="ts">
  import { page } from '$app/state';
  import SessionDetail from '../../../lib/SessionDetail.svelte';
  const id = $derived(page.params.id);
</script>

{#if id}{#key id}<SessionDetail {id} />{/key}{/if}
