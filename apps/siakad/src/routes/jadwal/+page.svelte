<script lang="ts">
  import { onMount } from 'svelte';
  import {
    SiakadClient,
    emptyPage,
    loadResourcePage,
    errorMessage,
    jakarta,
    permitted,
    type AcademicScheduleView
  } from '@algohub/siakad-client';
  import type { AppIdentityData } from '@algohub/auth-client';
  export let data: AppIdentityData;
  const client = new SiakadClient();
  let schedules: AcademicScheduleView[] = [];
  let schedulePage = emptyPage<AcademicScheduleView>();
  let loading = false;
  let issue = '';

  $: canManage = permitted(data.principal, 'schedule.write', {
    domain: 'siakad'
  });

  async function load() {
    if (loading) return;
    loading = true;
    issue = '';
    try {
      schedulePage = await loadResourcePage(
        client,
        '/academic/schedules',
        schedulePage,
        (row) => row.schedule.id
      );
      schedules = schedulePage.rows;
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

<svelte:head><title>Jadwal Saya — SIAKAD</title></svelte:head>
<section class="stack" aria-labelledby="jadwal-title">
  <h2 id="jadwal-title">Jadwal saya</h2>
  {#if loading}<p role="status">Memuat jadwal…</p>{/if}
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if !canManage}<p class="notice muted">
      Menampilkan jadwal kelas tempat Anda terdaftar.
    </p>{/if}
  {#each schedules as view (view.schedule.id)}
    <article class="panel" data-schedule-id={view.schedule.id}>
      {#if view.open_session}
        <span class="badge">Presensi terbuka: {view.open_session.title}</span>
      {/if}
      <h3>{view.schedule.course_name} ({view.schedule.class_code})</h3>
      <p>
        {view.schedule.day_of_week} · Shift {view.schedule.shift} · Ruang
        {view.schedule.room} · {view.schedule.starts_at}–{view.schedule.ends_at}
      </p>
      {#if view.open_session}
        <p class="muted">
          Sesi berakhir {jakarta(view.open_session.expires_at)} — buka halaman Presensi
          QR untuk check-in.
        </p>
      {/if}
    </article>
  {:else}
    {#if !loading}<p class="notice">Belum ada jadwal terdaftar.</p>{/if}
  {/each}
  {#if schedulePage.hasMore}
    <button class="secondary" disabled={loading} onclick={() => load()}
      >Muat jadwal lainnya</button
    >
  {/if}
</section>
