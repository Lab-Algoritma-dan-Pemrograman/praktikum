<script lang="ts">
  import { onMount } from 'svelte';
  import type { AppIdentityData } from '@algohub/auth-client';
  import {
    ElearningClient,
    errorMessage,
    levelProgressPercent,
    type LearningLevelsView
  } from '@algohub/elearning-client';
  export let data: AppIdentityData;
  let view: LearningLevelsView | null = null;
  let loading = false;
  let issue = '';
  $: canRead = data.principal?.grants.some(
    (g) => g.permission === 'learning.content.read'
  );
  $: canAdmin = data.principal?.grants.some(
    (g) =>
      g.permission === 'learning.content.manage' ||
      g.permission === 'learning.monitor'
  );
  $: adminApp = data.applications.find((a) => a.name === 'admin');
  $: practicumApp = data.applications.find((a) => a.name === 'praktikum');

  async function load() {
    loading = true;
    issue = '';
    try {
      view = await new ElearningClient().levels();
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loading = false;
    }
  }
  onMount(() => {
    if (canRead) void load();
  });
</script>

<svelte:head><title>Kurikulum E-learning — Algohub</title></svelte:head>
<section aria-labelledby="curriculum-title" class="stack">
  <h2 id="curriculum-title">Kurikulum dan progres belajar</h2>
  {#if !data.principal}
    <p class="notice">
      Anda belum masuk. Gunakan tautan Masuk di atas untuk mengakses materi,
      latihan kode, dan progres XP.
    </p>
  {:else if !canRead}
    <p class="notice">
      Akun ini tidak memiliki izin membaca konten pembelajaran
      (learning.content.read).
    </p>
  {:else}
    <div class="actions">
      <button class="secondary" onclick={load} disabled={loading}
        >Perbarui kurikulum</button
      >
      {#if practicumApp}<a
          class="button secondary"
          href={`${practicumApp.origin}/login`}>Buka Asesmen Praktikum</a
        >{/if}
      {#if canAdmin && adminApp}<a
          class="button secondary"
          href={`${adminApp.origin}/login?return_to=/elearning`}
          >Buka Admin E-learning</a
        >{/if}
    </div>
    {#if loading}<p role="status">Memuat kurikulum…</p>{/if}
    {#if issue}<div role="alert" class="notice error">
        {issue}
        <button onclick={load} disabled={loading}>Coba lagi</button>
      </div>{/if}
    {#if view}
      <section class="panel stack" aria-label="Ringkasan progres">
        <div class="grid">
          <div>
            <span class="muted">Level & XP</span>
            <p data-testid="profile-xp">
              <strong>Level {view.profile.level}</strong> · {view.profile.xp} XP (target
              level berikutnya: {view.profile.next_level_xp} XP)
            </p>
            <div class="progress-bar" aria-hidden="true">
              <div
                class="progress-fill"
                style={`width:${levelProgressPercent(view.profile.xp)}%`}
              ></div>
            </div>
          </div>
          <div>
            <span class="muted">Streak & Penyelesaian</span>
            <p data-testid="profile-streak">
              Streak: <strong>{view.profile.streak} hari</strong> · Pelajaran
              selesai: <strong>{view.profile.completed_lessons}</strong> ·
              Achievement:
              <strong>{view.profile.unlocked_achievements.length}</strong>
            </p>
          </div>
        </div>
      </section>
      <div class="stack">
        {#each view.levels as level}
          <article class="panel stack" data-level-id={level.id}>
            <div>
              <span class="badge"
                >{level.unlocked
                  ? 'Terbuka'
                  : level.access_mode === 'locked'
                    ? 'Dikunci instruktur'
                    : `Butuh ${level.required_xp} XP`}</span
              >
              <h3>{level.title}</h3>
              <p class="muted">
                {level.description} · Progres: {level.completed_lessons} / {level.total_lessons}
                pelajaran
              </p>
            </div>
            {#each level.modules ?? [] as mod}
              <section class="panel stack" data-module-id={mod.id}>
                <h4>{mod.title}</h4>
                <p class="muted">{mod.description}</p>
                <div class="grid">
                  {#each mod.lessons ?? [] as lesson}
                    <div class="panel stack" data-lesson-id={lesson.id}>
                      <p class="muted">
                        {lesson.language.toUpperCase()} · +{lesson.xp_reward} XP ·
                        Status: {lesson.status ?? 'not_started'}
                      </p>
                      <strong>{lesson.title}</strong>
                      {#if level.unlocked}
                        <a
                          class="button"
                          href={`/lesson/${encodeURIComponent(lesson.id)}`}
                          >Buka pelajaran</a
                        >
                      {:else}
                        <p class="muted">
                          Selesaikan target XP untuk membuka level ini.
                        </p>
                      {/if}
                    </div>
                  {/each}
                </div>
              </section>
            {/each}
          </article>
        {:else}
          <p class="notice">Belum ada level kurikulum yang terdaftar.</p>
        {/each}
      </div>
    {/if}
  {/if}
</section>
