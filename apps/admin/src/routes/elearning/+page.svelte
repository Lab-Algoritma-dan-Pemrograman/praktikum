<script lang="ts">
  import { onMount } from 'svelte';
  import type { AppIdentityData } from '@algohub/auth-client';
  import '@algohub/elearning-client/styles.css';
  import {
    ElearningClient,
    permitted,
    errorMessage,
    type LearningLevel
  } from '@algohub/elearning-client';
  import CurriculumAdmin from './CurriculumAdmin.svelte';
  import BugHuntAdmin from './BugHuntAdmin.svelte';
  import MonitoringAdmin from './MonitoringAdmin.svelte';
  export let data: AppIdentityData;
  const client = new ElearningClient();
  let levels: LearningLevel[] = [];
  let selectedId = '';
  let tab = 'curriculum';
  let loading = false;
  let issue = '';

  $: selectedLevel =
    levels.find((l) => l.id === selectedId) ?? levels[0] ?? null;
  $: canManageAny = !!data.principal?.grants.some(
    (g) => g.permission === 'learning.content.manage'
  );
  $: canMonitorAny = !!data.principal?.grants.some(
    (g) =>
      g.permission === 'learning.monitor' ||
      g.permission === 'learning.content.manage'
  );
  $: canAudit =
    !!selectedLevel &&
    permitted(data.principal, 'audit.read', selectedLevel.scope);

  async function load() {
    loading = true;
    issue = '';
    try {
      levels = await client.get<LearningLevel[]>('/admin/elearning/kurikulum');
      if (levels.length && !levels.some((l) => l.id === selectedId)) {
        selectedId = levels[0].id;
      }
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    void load();
  });
</script>

<svelte:head><title>Admin E-learning — Algohub</title></svelte:head>
<section class="elearning stack">
  <h1>Konfigurasi e-learning</h1>
  <p>
    Pengelolaan kurikulum, tantangan Bug Hunt, dan pemantauan progres mengikuti
    permission serta scope akun. Asesmen nilai tetap terpusat pada domain
    Praktikum.
  </p>
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if loading}<p role="status">Memuat Admin E-learning…</p>{/if}

  <nav aria-label="Panel Admin E-learning">
    <button
      class:secondary={tab !== 'curriculum'}
      aria-pressed={tab === 'curriculum'}
      onclick={() => (tab = 'curriculum')}>Kurikulum</button
    >
    {#if canManageAny}<button
        class:secondary={tab !== 'bughunt'}
        aria-pressed={tab === 'bughunt'}
        onclick={() => (tab = 'bughunt')}>Bug Hunt</button
      >{/if}
    {#if canMonitorAny}<button
        class:secondary={tab !== 'monitoring'}
        aria-pressed={tab === 'monitoring'}
        onclick={() => (tab = 'monitoring')}>Monitoring peserta</button
      >{/if}
    {#if canAudit}<a href="/auditlog">Audit sesuai scope</a>{/if}
  </nav>

  {#if tab === 'curriculum'}
    <CurriculumAdmin
      {levels}
      bind:selectedId
      principal={data.principal}
      refresh={load}
    />
  {:else if tab === 'bughunt' && canManageAny}
    <BugHuntAdmin principal={data.principal} />
  {:else if tab === 'monitoring' && canMonitorAny}
    <MonitoringAdmin />
  {/if}
</section>
