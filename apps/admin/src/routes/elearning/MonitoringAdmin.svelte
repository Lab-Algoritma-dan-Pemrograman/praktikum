<script lang="ts">
  import { onMount } from 'svelte';
  import {
    ElearningClient,
    errorMessage,
    jakarta,
    type LearningMonitoringRow
  } from '@algohub/elearning-client';
  const client = new ElearningClient();
  let rows: LearningMonitoringRow[] = [];
  let offset = 0;
  let loading = false;
  let issue = '';
  let failures = 0;

  async function load(next = offset) {
    if (loading) return;
    loading = true;
    try {
      rows = await client.get<LearningMonitoringRow[]>(
        `/admin/elearning/monitoring?limit=25&offset=${next}`
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

  onMount(() => {
    void load();
    const timer = setInterval(() => {
      if (!document.hidden && navigator.onLine && failures < 3) void load();
    }, 10_000);
    return () => clearInterval(timer);
  });
</script>

<div class="stack">
  <h2>Monitoring progres peserta</h2>
  <p class="muted">
    Hanya menampilkan peserta dalam scope Anda. Isi draf kode mahasiswa bersifat
    privat dan tidak dimuat pada tabel monitoring.
  </p>
  <div class="actions">
    <button class="secondary" onclick={() => load()} disabled={loading}
      >Perbarui monitoring</button
    >
  </div>
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if loading}<p role="status">Memuat monitoring E-learning…</p>{/if}
  <p class="muted" id="elearning-monitoring-scroll">
    Pada layar kecil, geser tabel secara horizontal untuk melihat semua kolom.
  </p>
  <!-- svelte-ignore a11y_no_noninteractive_tabindex (Horizontal table scrolling must be keyboard reachable.) -->
  <div
    class="table-wrap panel"
    role="region"
    aria-label="Tabel monitoring e-learning"
    aria-describedby="elearning-monitoring-scroll"
    tabindex="0"
  >
    <table>
      <caption>Progres peserta E-learning</caption>
      <thead>
        <tr>
          <th>Peserta</th>
          <th>Level & XP</th>
          <th>Streak</th>
          <th>Pelajaran</th>
          <th>Bug Hunt</th>
          <th>Diperbarui</th>
        </tr>
      </thead>
      <tbody>
        {#each rows as row}
          <tr data-monitoring-user={row.user_id}>
            <td
              >{row.user_name}<br /><span class="muted">ID {row.user_id}</span
              ></td
            >
            <td>Level {row.level} · {row.xp} XP</td>
            <td>{row.streak} hari ({row.last_active_date || '—'})</td>
            <td
              >{row.completed_lessons} selesai · {row.in_progress_lessons} berjalan</td
            >
            <td>{row.games_played} percobaan</td>
            <td>{jakarta(row.updated_at)}</td>
          </tr>
        {:else}
          <tr
            ><td colspan="6">Belum ada progres peserta dalam scope ini.</td></tr
          >
        {/each}
      </tbody>
    </table>
  </div>
  <div class="actions">
    <button
      class="secondary"
      disabled={offset === 0 || loading}
      onclick={() => load(offset - 25)}>Peserta sebelumnya</button
    >
    <button
      class="secondary"
      disabled={rows.length < 25 || loading}
      onclick={() => load(offset + 25)}>Peserta berikutnya</button
    >
  </div>
</div>
