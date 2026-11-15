<script lang="ts">
  import { onMount } from 'svelte';
  import type { AppIdentityData } from '@algohub/auth-client';
  import {
    PracticumClient,
    errorMessage,
    jakarta,
    gradingLabels,
    attemptLabels,
    courseLabels,
    type GradeSummary
  } from '@algohub/practicum-client';
  export let data: AppIdentityData;
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
    if (data.principal) void load();
    else loading = false;
  });
</script>

<svelte:head><title>Nilai & riwayat — Praktikum</title></svelte:head>
<header class="page-heading">
  <p class="eyebrow">HASIL PRAKTIKUM</p>
  <h1>Nilai & riwayat</h1>
  <p class="muted">
    Riwayat pengerjaan milik Anda tetap tersedia meski penugasan kelas berubah.
    Nilai hanya ditampilkan setelah dipublikasikan pengajar.
  </p>
</header>
{#if !data.principal}
  <section class="panel empty-state">
    <h2>Masuk untuk melihat riwayat Anda</h2>
    <p>Nilai dan jawaban hanya dapat dibuka oleh pemilik akun.</p>
    <a class="button" href="/login?return_to=/nilai">Masuk ke Praktikum</a>
  </section>
{:else}
  <section aria-label="Riwayat pengerjaan" aria-busy={loading}>
    <div class="section-heading">
      <h2>Pengerjaan Anda</h2>
      <button class="secondary" onclick={() => load()} disabled={loading}
        >Perbarui nilai</button
      >
    </div>
    {#if loading}<p role="status" class="muted">Memuat nilai…</p>{/if}
    {#if issue}<div class="notice error" role="alert">
        {issue}<button
          class="secondary"
          onclick={() => load()}
          disabled={loading}>Coba lagi</button
        >
      </div>{/if}
    <div class="stack">
      {#each grades as grade (grade.attempt_id)}
        <article class="panel grade-card" data-attempt-id={grade.attempt_id}>
          <div class="card-meta">
            <p class="eyebrow">
              {courseLabels[grade.course_kind] ?? 'Praktikum'}
            </p>
            <span class="badge"
              >{(grade.status
                ? attemptLabels[grade.status]
                : gradingLabels[grade.grading_status]) ??
                'Status belum tersedia'}</span
            >
          </div>
          <h2>{grade.session_title}</h2>
          <p class="grade-course">{grade.course_title}</p>
          <div class="grade-result">
            <strong
              >{grade.score === null
                ? 'Belum dipublikasikan'
                : `${grade.score} / ${grade.max_score}`}</strong
            >{#if grade.percentage !== null && Number.isFinite(grade.percentage)}<span
                >({grade.percentage.toFixed(1)}%)</span
              >{/if}
          </div>
          <p class="muted">
            {grade.status === 'in_progress'
              ? 'Pengerjaan belum dikumpulkan.'
              : (gradingLabels[grade.grading_status] ??
                'Status penilaian belum tersedia.')}
          </p>
          {#if grade.submitted_at}<p class="muted">
              Dikumpulkan: {jakarta(grade.submitted_at)}
            </p>{/if}
          {#if grade.can_open !== false}<a
              class="button secondary"
              href={`/attempts/${encodeURIComponent(grade.attempt_id)}`}
              >{grade.status === 'in_progress'
                ? 'Lanjutkan pengerjaan'
                : 'Lihat jawaban dan feedback'}</a
            >{:else}<p class="notice">
              Akses pengerjaan aktif mengikuti kelas dan shift saat ini. Hubungi
              pengajar untuk memeriksa penugasan Anda.
            </p>{/if}
        </article>
      {:else}{#if !loading && !issue}<div class="panel empty-state">
            <h3>Belum ada riwayat pengerjaan</h3>
            <p>
              Pengerjaan akan muncul setelah Anda membuka kegiatan praktikum.
            </p>
            <a class="button" href="/">Lihat sesi praktikum</a>
          </div>{/if}{/each}
    </div>
    {#if grades.length > 0 || offset > 0}<div
        class="pagination"
        aria-label="Halaman riwayat"
      >
        <button
          class="secondary"
          disabled={offset === 0 || loading}
          onclick={() => load(offset - 25)}>Sebelumnya</button
        ><span class="muted">Halaman {Math.floor(offset / 25) + 1}</span><button
          class="secondary"
          disabled={grades.length < 25 || loading}
          onclick={() => load(offset + 25)}>Berikutnya</button
        >
      </div>{/if}
  </section>
{/if}
