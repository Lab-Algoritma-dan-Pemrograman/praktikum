<script lang="ts">
  import { onMount } from 'svelte';
  import {
    SiakadClient,
    reusableCommand,
    emptyPage,
    loadResourcePage,
    mergeResources,
    errorMessage,
    jakarta,
    type AcademicAttendanceRecord,
    type AcademicScheduleView,
    type Command
  } from '@algohub/siakad-client';
  const client = new SiakadClient();
  let schedules: AcademicScheduleView[] = [];
  let records: AcademicAttendanceRecord[] = [];
  let schedulePage = emptyPage<AcademicScheduleView>();
  let recordPage = emptyPage<AcademicAttendanceRecord>();
  let sessionId = '';
  let qrToken = '';
  let loading = false;
  let busy = false;
  let issue = '';
  let notice = '';
  let pending: Command | null = null;

  $: openSessions = schedules
    .map((s) => s.open_session)
    .filter((s): s is NonNullable<typeof s> => !!s);

  async function load(kind: 'all' | 'schedules' | 'records' = 'all') {
    if (loading) return;
    loading = true;
    issue = '';
    const selected = schedules.find(
      (row) => row.open_session?.id === sessionId
    );
    try {
      const [sched, att] = await Promise.all([
        kind === 'all' || kind === 'schedules'
          ? loadResourcePage(
              client,
              '/academic/schedules',
              schedulePage,
              (row) => row.schedule.id,
              kind !== 'all',
              selected ? [selected] : []
            )
          : schedulePage,
        kind === 'all' || kind === 'records'
          ? loadResourcePage(
              client,
              '/academic/attendance',
              recordPage,
              (row) => row.id,
              kind !== 'all'
            )
          : recordPage
      ]);
      schedulePage = sched;
      recordPage = att;
      schedules = sched.rows;
      records = att.rows;
      if (!sessionId && !pending) {
        sessionId =
          schedules.find((row) => row.open_session)?.open_session?.id ?? '';
      }
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loading = false;
    }
  }

  async function checkIn() {
    if (busy || loading || !sessionId || !qrToken.trim()) return;
    busy = true;
    issue = '';
    notice = '';
    pending = reusableCommand(pending, '/academic/attendance', 'POST', {
      session_id: sessionId.trim(),
      qr_token: qrToken.trim()
    });
    try {
      const rec = await client.execute<AcademicAttendanceRecord>(pending);
      pending = null;
      qrToken = '';
      records = mergeResources(records, [rec], (row) => row.id);
      recordPage = { ...recordPage, rows: records };
      notice = `Presensi berhasil dicatat (${rec.status}). Menunggu validasi staf.`;
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      busy = false;
    }
  }

  onMount(() => {
    void load();
  });
</script>

<svelte:head><title>Presensi QR — SIAKAD</title></svelte:head>
<section class="stack" aria-labelledby="attendance-title">
  <h2 id="attendance-title">Presensi QR laboratorium</h2>
  {#if loading}<p role="status">Memuat sesi presensi…</p>{/if}
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if notice}<p class="notice success" role="status">{notice}</p>{/if}

  <form
    class="panel stack"
    onsubmit={(e) => {
      e.preventDefault();
      void checkIn();
    }}
  >
    <h3>Check-in presensi</h3>
    {#if openSessions.length > 0}
      <label
        >Pilih sesi presensi terbuka<select
          bind:value={sessionId}
          required
          disabled={busy || loading}
        >
          {#if sessionId && !openSessions.some((s) => s.id === sessionId)}
            <option value={sessionId}>Sesi dipilih: {sessionId}</option>
          {/if}
          {#each openSessions as s}<option value={s.id}
              >{s.title} (berakhir {jakarta(s.expires_at)})</option
            >{/each}
        </select></label
      >
    {:else}
      <label
        >ID sesi presensi<input
          bind:value={sessionId}
          required
          maxlength="128"
          placeholder="att-session-..."
        /></label
      >
    {/if}
    {#if schedulePage.hasMore}
      <button
        type="button"
        class="secondary"
        disabled={busy || loading}
        onclick={() => load('schedules')}>Muat sesi presensi lainnya</button
      >
    {/if}
    <label
      >Token QR presensi<input
        bind:value={qrToken}
        required
        maxlength="256"
        autocomplete="off"
      /></label
    >
    <button disabled={busy || loading || !sessionId.trim() || !qrToken.trim()}
      >{busy ? 'Memproses…' : 'Kirim presensi QR'}</button
    >
  </form>

  <section class="stack" aria-label="Riwayat presensi saya">
    <h3>Riwayat presensi saya</h3>
    {#each records as rec}
      <article class="panel" data-attendance-id={rec.id}>
        <span class="badge"
          >{rec.status.toUpperCase()} · {rec.validated
            ? 'Tervalidasi'
            : 'Menunggu validasi'}</span
        >
        <p>Sesi: {rec.session_id} · Kelas: {rec.class_id}</p>
        {#if rec.notes}<p>Catatan staf: {rec.notes}</p>{/if}
        <p class="muted">Waktu check-in: {jakarta(rec.checked_in_at)}</p>
      </article>
    {:else}
      {#if !loading}<p class="notice">Belum ada riwayat presensi.</p>{/if}
    {/each}
    {#if recordPage.hasMore}
      <button
        class="secondary"
        disabled={busy || loading}
        onclick={() => load('records')}>Muat riwayat presensi lainnya</button
      >
    {/if}
  </section>
</section>
