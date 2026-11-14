<script lang="ts">
  import { onMount } from 'svelte';
  import type { Principal } from '@algohub/auth-client';
  import {
    PracticumClient,
    permitted,
    reusableCommand,
    errorMessage,
    jakarta,
    type Command,
    type MonitoringRow,
    type SessionDetail
  } from '@algohub/practicum-client';
  import Review from './Review.svelte';
  export let detail: SessionDetail;
  export let principal: Principal | null;
  const client = new PracticumClient();
  let rows: MonitoringRow[] = [];
  let activationId = '';
  let offset = 0;
  let loading = false;
  let issue = '';
  let notice = '';
  let failures = 0;
  let selected: string[] = [];
  let reviewId = '';
  let pending: Command | null = null;
  let busy = false;
  function canGrade(row: MonitoringRow) {
    const activation = detail.activations.find(
      (a) => a.id === row.activation_id
    );
    return (
      !!activation &&
      permitted(principal, 'practicum.grade.write', activation.scope)
    );
  }
  async function load(next = offset) {
    if (loading) return;
    loading = true;
    try {
      rows = await client.get<MonitoringRow[]>(
        `/admin/praktikum/monitoring?${new URLSearchParams({ session_id: detail.session.id, ...(activationId ? { activation_id: activationId } : {}), limit: '25', offset: String(next) })}`
      );
      offset = next;
      issue = '';
      failures = 0;
    } catch (error) {
      issue = errorMessage(error);
      failures += 1;
    } finally {
      loading = false;
    }
  }
  async function enqueue() {
    const allowed = selected.filter((id) =>
      rows.some(
        (r) => r.attempt_id === id && canGrade(r) && r.status !== 'in_progress'
      )
    );
    if (!allowed.length) return;
    busy = true;
    issue = '';
    notice = '';
    pending = reusableCommand(
      pending,
      '/admin/praktikum/penilaian/bulk',
      'POST',
      { attempt_ids: allowed }
    );
    try {
      await client.execute(pending);
      pending = null;
      notice =
        'Pekerjaan penilaian diantrikan. Buka review untuk meminta saran; nilai tetap harus diperiksa manual.';
      selected = [];
      await load();
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      busy = false;
    }
  }
  onMount(() => {
    void load();
    const timer = setInterval(() => {
      if (!document.hidden && navigator.onLine && failures < 3 && !reviewId)
        void load();
    }, 10_000);
    return () => clearInterval(timer);
  });
</script>

<div class="stack">
  <h2>Monitoring dan penilaian</h2>
  <p class="muted">
    Hanya attempt dalam scope Anda. Diperbarui setiap 10 detik saat panel
    terlihat; berhenti setelah 3 kegagalan. Monitoring tidak memuat isi jawaban.
  </p>
  <div class="actions">
    <label
      >Filter activation<select
        bind:value={activationId}
        onchange={() => {
          selected = [];
          void load(0);
        }}
        disabled={loading}
        ><option value="">Semua activation yang diizinkan</option
        >{#each detail.activations as activation}<option value={activation.id}
            >{activation.id} · {activation.scope.class_id} / {activation.scope
              .shift}</option
          >{/each}</select
      ></label
    ><button class="secondary" onclick={() => load()} disabled={loading}
      >Perbarui monitoring</button
    >
  </div>
  {#if issue}<p class="notice error" role="alert">
      {issue}{failures >= 3
        ? ' Polling dijeda. Tekan Perbarui monitoring untuk mencoba lagi.'
        : ''}
    </p>{/if}
  {#if notice}<p class="notice" role="status">{notice}</p>{/if}
  {#if loading}<p role="status">Memuat monitoring…</p>{/if}
  <p class="muted" id="monitoring-scroll">
    Pada layar kecil, geser tabel secara horizontal untuk melihat semua kolom.
  </p>
  <!-- svelte-ignore a11y_no_noninteractive_tabindex (Horizontal table scrolling must be keyboard reachable.) -->
  <div
    class="table-wrap"
    role="region"
    aria-label="Tabel monitoring peserta"
    aria-describedby="monitoring-scroll"
    tabindex="0"
  >
    <table>
      <caption>Attempt peserta</caption><thead
        ><tr
          ><th>Peserta</th><th>Status</th><th>Jawaban</th><th>Deadline</th><th
            >Tindakan</th
          ></tr
        ></thead
      ><tbody
        >{#each rows as row}<tr data-attempt-id={row.attempt_id}
            ><td
              >{row.user_name}<br /><span class="muted">ID {row.user_id}</span
              ></td
            ><td>{row.status}<br />{row.grading_status}</td><td
              >{row.answer_count} / {row.question_count}</td
            ><td>{jakarta(row.deadline_at)}</td><td
              >{#if canGrade(row) && row.status !== 'in_progress'}<label
                  class="check"
                  ><input
                    type="checkbox"
                    value={row.attempt_id}
                    bind:group={selected}
                  />Pilih {row.user_name}</label
                ><button
                  class="secondary"
                  onclick={() => (reviewId = row.attempt_id)}
                  >Review {row.user_name}</button
                >{:else}<span class="muted"
                  >{row.status === 'in_progress'
                    ? 'Belum dikumpulkan'
                    : 'Tidak ada izin penilaian'}</span
                >{/if}</td
            ></tr
          >{:else}<tr
            ><td colspan="5">Belum ada attempt dalam filter ini.</td></tr
          >{/each}</tbody
      >
    </table>
  </div>
  <div class="actions">
    {#if selected.length}<button
        onclick={enqueue}
        disabled={busy || selected.length > 25}
        >Antrikan penilaian ({selected.length})</button
      >{/if}<button
      class="secondary"
      disabled={offset === 0 || loading}
      onclick={() => {
        selected = [];
        void load(offset - 25);
      }}>Attempt sebelumnya</button
    ><button
      class="secondary"
      disabled={rows.length < 25 || loading}
      onclick={() => {
        selected = [];
        void load(offset + 25);
      }}>Attempt berikutnya</button
    >
  </div>
  {#if reviewId}<div class="panel">
      <button class="secondary" onclick={() => (reviewId = '')}
        >Tutup review</button
      >{#key reviewId}<Review id={reviewId} />{/key}
    </div>{/if}
</div>
