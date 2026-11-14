<script lang="ts">
  import { onMount } from 'svelte';
  import {
    ElearningClient,
    errorMessage,
    jakarta,
    type LearningAchievementsView,
    type LearningLeaderboardRow
  } from '@algohub/elearning-client';
  const client = new ElearningClient();
  let rows: LearningLeaderboardRow[] = [];
  let achView: LearningAchievementsView | null = null;
  let offset = 0;
  let loading = true;
  let issue = '';

  async function load(next = offset) {
    loading = true;
    issue = '';
    try {
      const [lb, ach] = await Promise.all([
        client.leaderboard(next),
        client.achievements()
      ]);
      rows = lb;
      achView = ach;
      offset = next;
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

<svelte:head><title>Leaderboard & Achievement — E-learning</title></svelte:head>
<section class="stack" aria-labelledby="leaderboard-title">
  <h1 id="leaderboard-title">Leaderboard & Achievement</h1>
  <div class="actions">
    <button class="secondary" onclick={() => load()} disabled={loading}
      >Perbarui peringkat</button
    >
  </div>
  {#if loading}<p role="status">Memuat peringkat dan achievement…</p>{/if}
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}

  {#if achView}
    <section class="stack" aria-label="Daftar achievement">
      <h2>Achievement saya</h2>
      <div class="grid">
        {#each achView.achievements as ach}
          <article class="panel stack" data-achievement-id={ach.id}>
            <span class="badge"
              >{ach.unlocked ? 'Terbuka' : 'Belum terbuka'}</span
            >
            <h3>{ach.title}</h3>
            <p>{ach.description}</p>
            {#if ach.unlocked_at}<p class="muted">
                Dibuka: {jakarta(ach.unlocked_at)}
              </p>{/if}
          </article>
        {/each}
      </div>
    </section>
  {/if}

  <section class="stack" aria-label="Tabel peringkat XP">
    <h2>Papan peringkat</h2>
    <p class="muted" id="leaderboard-scroll">
      Pada layar kecil, geser tabel secara horizontal untuk melihat semua kolom.
    </p>
    <!-- svelte-ignore a11y_no_noninteractive_tabindex (Horizontal table scrolling must be keyboard reachable.) -->
    <div
      class="table-wrap panel"
      role="region"
      aria-label="Tabel leaderboard"
      aria-describedby="leaderboard-scroll"
      tabindex="0"
    >
      <table>
        <caption>Peringkat peserta E-learning</caption>
        <thead>
          <tr>
            <th>Peringkat</th>
            <th>Peserta</th>
            <th>Level & XP</th>
            <th>Streak</th>
            <th>Pelajaran selesai</th>
            <th>Achievement</th>
          </tr>
        </thead>
        <tbody>
          {#each rows as row}
            <tr data-leaderboard-user={row.user_id}>
              <td>#{row.rank}</td>
              <td>{row.user_name}</td>
              <td>Level {row.level} · {row.xp} XP</td>
              <td>{row.streak} hari</td>
              <td>{row.completed_lessons}</td>
              <td>{row.achievement_count}</td>
            </tr>
          {:else}
            <tr><td colspan="6">Belum ada peserta pada halaman ini.</td></tr>
          {/each}
        </tbody>
      </table>
    </div>
    <div class="actions">
      <button
        class="secondary"
        disabled={offset === 0 || loading}
        onclick={() => load(offset - 25)}>Sebelumnya</button
      >
      <button
        class="secondary"
        disabled={rows.length < 25 || loading}
        onclick={() => load(offset + 25)}>Berikutnya</button
      >
    </div>
  </section>
</section>
