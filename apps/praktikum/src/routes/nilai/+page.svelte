<script lang="ts">
  import { onMount } from 'svelte';
  import {
    PracticumClient,
    errorMessage,
    jakarta,
    type GradeSummary
  } from '@algohub/practicum-client';
  let grades: GradeSummary[] = [];
  let loading = true;
  let issue = '';
  let offset = 0;
  async function load(next = offset) {
    loading = true;
    issue = '';
    try {
      grades = await new PracticumClient().grades(next);
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

<svelte:head><title>Nilai saya — Praktikum</title></svelte:head>
<h1>Nilai saya</h1>
<p>
  Nilai hanya ditampilkan setelah dipublikasikan pengajar. Saran penilaian
  otomatis bukan nilai final.
</p>
<div class="actions">
  <button class="secondary" onclick={() => load()} disabled={loading}
    >Perbarui nilai</button
  >
</div>
{#if loading}<p role="status">Memuat nilai…</p>{/if}
{#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
<div class="stack">
  {#each grades as grade}<article
      class="panel"
      data-attempt-id={grade.attempt_id}
    >
      <h2>{grade.session_title} · {grade.course_title}</h2>
      <p>Status penilaian: {grade.grading_status}</p>
      <p>
        Nilai: {grade.score === null
          ? 'Belum dipublikasikan'
          : `${grade.score} / ${grade.max_score}`}
        {grade.percentage === null ? '' : `(${grade.percentage.toFixed(1)}%)`}
      </p>
      <p class="muted">Dikumpulkan: {jakarta(grade.submitted_at)}</p>
      <a href={`/attempts/${encodeURIComponent(grade.attempt_id)}`}
        >Lihat jawaban dan feedback</a
      >
    </article>{:else}{#if !loading && !issue}<p class="notice">
        Belum ada hasil attempt.
      </p>{/if}{/each}
</div>
<div class="actions">
  <button
    class="secondary"
    disabled={offset === 0 || loading}
    onclick={() => load(offset - 25)}>Sebelumnya</button
  ><button
    class="secondary"
    disabled={grades.length < 25 || loading}
    onclick={() => load(offset + 25)}>Berikutnya</button
  >
</div>
