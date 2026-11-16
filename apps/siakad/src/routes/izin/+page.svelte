<script lang="ts">
  import { onMount } from 'svelte';
  import {
    SiakadClient,
    leaveCommand,
    leaveDecisionCommand,
    emptyPage,
    loadResourcePage,
    mergeResources,
    errorMessage,
    jakarta,
    permitted,
    type AcademicLeaveRequest,
    type AcademicScheduleView,
    type Command
  } from '@algohub/siakad-client';
  import type { AppIdentityData } from '@algohub/auth-client';
  export let data: AppIdentityData;
  const client = new SiakadClient();
  let leaves: AcademicLeaveRequest[] = [];
  let leavePage = emptyPage<AcademicLeaveRequest>();
  let schedules: AcademicScheduleView[] = [];
  let schedulePage = emptyPage<AcademicScheduleView>();
  let scheduleId = '';
  let kind: AcademicLeaveRequest['kind'] = 'izin';
  let reason = '';
  let originalDate = '';
  let decisions: Record<string, { action: string; rescheduleAt: string }> = {};
  let loading = false;
  let busy = false;
  let issue = '';
  let notice = '';
  let pending: Command | null = null;
  let decisionPending: Record<string, Command> = {};

  $: canDecide =
    permitted(data.principal, 'leave.decide', { domain: 'siakad' }) ||
    permitted(data.principal, 'schedule.write', { domain: 'siakad' });

  async function load(kindArg: 'all' | 'leaves' | 'schedules' = 'all') {
    if (loading) return;
    loading = true;
    issue = '';
    try {
      const [ownLeaves, sched] = await Promise.all([
        kindArg === 'all' || kindArg === 'leaves'
          ? loadResourcePage(
              client,
              '/academic/izin',
              leavePage,
              (row) => row.id,
              kindArg !== 'all'
            )
          : leavePage,
        kindArg === 'all' || kindArg === 'schedules'
          ? loadResourcePage(
              client,
              '/academic/schedules',
              schedulePage,
              (row) => row.schedule.id,
              kindArg !== 'all'
            )
          : schedulePage
      ]);
      leavePage = ownLeaves;
      schedulePage = sched;
      leaves = ownLeaves.rows;
      schedules = sched.rows;
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loading = false;
    }
  }

  async function submitLeave() {
    if (busy || !scheduleId || !reason.trim() || !originalDate) return;
    busy = true;
    issue = '';
    notice = '';
    const draft = {
      schedule_id: scheduleId,
      kind,
      reason: reason.trim(),
      original_at: new Date(`${originalDate}T08:00:00+07:00`).toISOString()
    };
    pending = leaveCommand(pending, draft);
    try {
      const created = await client.execute<AcademicLeaveRequest>(pending);
      pending = null;
      reason = '';
      leaves = mergeResources(leaves, [created], (row) => row.id);
      leavePage = { ...leavePage, rows: leaves };
      notice = `Pengajuan ${created.kind} terkirim, menunggu keputusan staf.`;
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      busy = false;
    }
  }

  async function decide(row: AcademicLeaveRequest) {
    if (busy) return;
    const choice = decisions[row.id];
    if (!choice?.action) return;
    if (choice.action === 'reschedule' && !choice.rescheduleAt) {
      issue = 'Pilih tanggal reschedule terlebih dahulu.';
      return;
    }
    busy = true;
    issue = '';
    notice = '';
    const mutation = leaveDecisionCommand(
      decisionPending[row.id] ?? null,
      row.id,
      {
        version: row.version,
        action: choice.action,
        ...(choice.action === 'reschedule'
          ? {
              reschedule_at: new Date(
                `${choice.rescheduleAt}T08:00:00+07:00`
              ).toISOString()
            }
          : {})
      }
    );
    decisionPending[row.id] = mutation;
    try {
      const updated = await client.execute<AcademicLeaveRequest>(mutation);
      delete decisionPending[row.id];
      leaves = mergeResources(leaves, [updated], (r) => r.id);
      leavePage = { ...leavePage, rows: leaves };
      notice = `Pengajuan ${updated.user_name} diputuskan: ${updated.status}.`;
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

<svelte:head><title>Izin & Reschedule — SIAKAD</title></svelte:head>
<section class="stack" aria-labelledby="izin-title">
  <h2 id="izin-title">Izin dan reschedule</h2>
  {#if loading}<p role="status">Memuat pengajuan…</p>{/if}
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if notice}<p class="notice success" role="status">{notice}</p>{/if}

  <form
    class="panel stack"
    onsubmit={(e) => {
      e.preventDefault();
      void submitLeave();
    }}
  >
    <h3>Ajukan izin / sakit / reschedule</h3>
    <div class="grid">
      <label
        >Jadwal kelas<select bind:value={scheduleId} required disabled={busy}>
          {#each schedules as view (view.schedule.id)}
            <option value={view.schedule.id}
              >{view.schedule.class_code} · {view.schedule.day_of_week} ·
              {view.schedule.starts_at}</option
            >
          {/each}
        </select></label
      >
      <label
        >Jenis<select bind:value={kind} disabled={busy}>
          <option value="izin">Izin</option>
          <option value="sakit">Sakit</option>
          <option value="reschedule">Reschedule</option>
        </select></label
      >
      <label
        >Tanggal yang ditinggalkan<input
          type="date"
          bind:value={originalDate}
          required
          disabled={busy}
        /></label
      >
    </div>
    <label
      >Alasan detail<textarea
        bind:value={reason}
        required
        maxlength="500"
        rows="3"
        placeholder="Contoh: Ada acara keluarga / dirawat di RS…"
      ></textarea></label
    >
    <button disabled={busy || !scheduleId || !reason.trim() || !originalDate}
      >{busy ? 'Mengirim…' : 'Kirim pengajuan'}</button
    >
  </form>

  <section class="stack" aria-label="Riwayat pengajuan saya">
    <h3>Riwayat pengajuan saya</h3>
    {#each leaves as row (row.id)}
      <article class="panel" data-leave-id={row.id}>
        <span class="badge"
          >{row.kind.toUpperCase()} · {row.status.toUpperCase()}</span
        >
        <p>{row.reason}</p>
        <p class="muted">
          Jadwal: {row.schedule_id} · Diajukan untuk {jakarta(row.original_at)}
        </p>
        {#if row.decided_at}
          <p class="muted">Diputuskan {jakarta(row.decided_at)}</p>
        {/if}
        {#if row.reschedule_at}
          <p class="muted">Reschedule ke {jakarta(row.reschedule_at)}</p>
        {/if}
      </article>
    {:else}
      {#if !loading}<p class="notice">Belum ada pengajuan izin.</p>{/if}
    {/each}
    {#if leavePage.hasMore}
      <button
        class="secondary"
        disabled={loading}
        onclick={() => load('leaves')}>Muat pengajuan lainnya</button
      >
    {/if}
  </section>

  {#if canDecide}
    <section class="stack" aria-label="Pengajuan menunggu keputusan">
      <h3>Pengajuan mahasiswa</h3>
      {#each leaves.filter((row) => row.status === 'pending') as row (row.id)}
        <article class="panel" data-decision-leave-id={row.id}>
          <span class="badge">{row.kind.toUpperCase()} · {row.user_name}</span>
          <p>{row.reason}</p>
          <p class="muted">
            Jadwal {row.schedule_id} · {jakarta(row.original_at)}
          </p>
          <div class="grid">
            <label
              >Keputusan<select
                value={decisions[row.id]?.action ?? ''}
                disabled={busy}
                onchange={(e) =>
                  (decisions[row.id] = {
                    action: (e.currentTarget as HTMLSelectElement).value,
                    rescheduleAt: decisions[row.id]?.rescheduleAt ?? ''
                  })}
              >
                <option value="" disabled>Pilih aksi</option>
                <option value="approve">Setujui</option>
                <option value="reject">Tolak</option>
                <option value="reschedule">Setujui + reschedule</option>
              </select></label
            >
            {#if decisions[row.id]?.action === 'reschedule'}
              <label
                >Tanggal pengganti<input
                  type="date"
                  value={decisions[row.id]?.rescheduleAt ?? ''}
                  disabled={busy}
                  oninput={(e) =>
                    (decisions[row.id] = {
                      action: decisions[row.id]?.action ?? 'reschedule',
                      rescheduleAt: (e.currentTarget as HTMLInputElement).value
                    })}
                /></label
              >
            {/if}
            <button
              disabled={busy || !decisions[row.id]?.action}
              onclick={() => decide(row)}>Simpan keputusan</button
            >
          </div>
        </article>
      {:else}
        {#if !loading}<p class="notice">Tidak ada pengajuan pending.</p>{/if}
      {/each}
    </section>
  {/if}
</section>
