<script lang="ts">
  import { onMount } from 'svelte';
  import { ApiError } from '@algohub/auth-client';
  import {
    PracticumClient,
    reusableCommand,
    errorMessage,
    type AttemptReview,
    type Command,
    type GradeSuggestion
  } from '@algohub/practicum-client';
  export let id: string;
  const client = new PracticumClient();
  let review: AttemptReview | null = null;
  let grades: GradeSuggestion[] = [];
  let revision = 0;
  let reason = '';
  let publish = false;
  let keaktifan: number | null = null;
  let busy = false;
  let issue = '';
  let notice = '';
  let conflict = false;
  let pending: Command | null = null;
  $: keaktifanValid =
    keaktifan === null ||
    (Number.isInteger(keaktifan) && keaktifan >= 0 && keaktifan <= 100);
  $: valid =
    !!review &&
    grades.length === review.questions.length &&
    grades.every(
      (grade) =>
        Number.isInteger(grade.score) &&
        grade.score >= 0 &&
        grade.score <=
          (review?.questions.find((q) => q.id === grade.question_id)?.points ??
            0)
    ) &&
    keaktifanValid &&
    reason.trim().length > 0;
  function serverGrades() {
    grades =
      review?.questions.map((q) => ({
        question_id: q.id,
        score:
          review?.attempt.answers.find((a) => a.question_id === q.id)?.score ??
          0,
        feedback:
          review?.attempt.answers.find((a) => a.question_id === q.id)
            ?.feedback ?? ''
      })) ?? [];
    revision = review?.attempt.grade_revision ?? 0;
    keaktifan =
      (review as { keaktifan?: number | null } | null)?.keaktifan ?? null;
  }
  async function load(preserve = false) {
    busy = true;
    issue = '';
    try {
      const incoming = await client.get<AttemptReview>(
        `/admin/praktikum/penilaian/${encodeURIComponent(id)}`
      );
      if (
        !review ||
        incoming.attempt.grade_revision >= review.attempt.grade_revision
      )
        review = incoming;
      if (!preserve) serverGrades();
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      busy = false;
    }
  }
  async function suggestions() {
    busy = true;
    issue = '';
    notice = '';
    pending = reusableCommand(
      pending,
      `/admin/praktikum/penilaian/${encodeURIComponent(id)}/grade`,
      'POST',
      {}
    );
    try {
      const incoming = await client.execute<AttemptReview>(pending);
      if (
        !review ||
        incoming.attempt.grade_revision >= review.attempt.grade_revision
      )
        review = incoming;
      pending = null;
      notice =
        'Saran dimuat. Periksa rubric dan jawaban; saran tidak diterapkan atau dipublikasikan otomatis.';
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      busy = false;
    }
  }
  async function save() {
    if (!valid || conflict) return;
    busy = true;
    issue = '';
    notice = '';
    pending = reusableCommand(
      pending,
      `/admin/praktikum/penilaian/${encodeURIComponent(id)}/manual`,
      'PUT',
      {
        grade_revision: revision,
        grades,
        publish,
        reason,
        ...(keaktifan === null ? {} : { keaktifan })
      }
    );
    try {
      review = await client.execute<AttemptReview>(pending);
      pending = null;
      serverGrades();
      notice = publish
        ? 'Nilai dipublikasikan oleh server.'
        : 'Penilaian manual disimpan; belum dipublikasikan.';
    } catch (error) {
      issue = errorMessage(error);
      if (error instanceof ApiError && error.status === 409) {
        conflict = true;
        await load(true);
      }
    } finally {
      busy = false;
    }
  }
  onMount(() => {
    void load();
  });
</script>

<section class="stack" aria-labelledby="review-title">
  <h2 id="review-title">Review jawaban dan rubric privat</h2>
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if notice}<p class="notice" role="status">{notice}</p>{/if}
  {#if busy}<p role="status">Memproses penilaian…</p>{/if}
  {#if !review}<button class="secondary" onclick={() => load()} disabled={busy}
      >Muat ulang review</button
    >{:else}
    <p>
      Attempt {review.attempt.attempt_id} · {review.attempt.grading_status} · Revisi
      nilai {review.attempt.grade_revision}
    </p>
    {#if review.grading_job?.simulated}<p class="notice">
        Saran simulasi dari fixture. Tidak ada evaluasi AI nyata; pengajar wajib
        menilai jawaban secara manual.
      </p>{/if}
    {#if review.attempt.grading_status === 'needs_manual'}<p class="notice">
        Penilaian manual diperlukan. Provider tidak tersedia atau saran belum
        layak dipublikasikan.
      </p>{/if}
    <button
      class="secondary"
      onclick={suggestions}
      disabled={busy || conflict || review.attempt.status === 'in_progress'}
      >Minta / ulang saran penilaian</button
    >
    {#if conflict}<section class="notice">
        <h3>Nilai telah berubah di server</h3>
        <p>
          Form lokal dipertahankan. Nilai terbaru di server ditampilkan pada
          setiap soal. Pilih sebelum menyimpan ulang.
        </p>
        <div class="actions">
          <button
            class="secondary"
            onclick={() => {
              serverGrades();
              conflict = false;
              pending = null;
            }}>Gunakan nilai server</button
          ><button
            onclick={() => {
              revision = review?.attempt.grade_revision ?? 0;
              conflict = false;
              pending = null;
            }}>Pertahankan koreksi lokal</button
          >
        </div>
      </section>{/if}
    <form
      class="stack"
      onsubmit={(event) => {
        event.preventDefault();
        void save();
      }}
    >
      {#each review.questions as question, index}
        <section class="panel stack">
          <h3>Soal {index + 1} · {question.points} poin</h3>
          <p class="prompt">{question.prompt}</p>
          <h4>Jawaban peserta</h4>
          <pre>{review.attempt.answers.find(
              (a) => a.question_id === question.id
            )?.value || '(tidak dijawab)'}</pre>
          <details>
            <summary>Referensi dan rubric privat</summary>
            <pre>{question.reference_answer || '(tidak ada referensi)'}</pre>
            <ul>
              {#each question.rubric ?? [] as criterion}<li>
                  {criterion.description} · {criterion.max_score} poin
                </li>{/each}
            </ul>
          </details>
          {#each review.suggestions ?? [] as suggestion}{#if suggestion.question_id === question.id}<p
                class="notice"
              >
                Saran (bukan nilai final): {suggestion.score} / {question.points}<br
                />{suggestion.feedback}
              </p>{/if}{/each}
          {#if conflict}<p>
              Nilai server: {review.attempt.answers.find(
                (a) => a.question_id === question.id
              )?.score ?? 'Belum dinilai'} · {review.attempt.answers.find(
                (a) => a.question_id === question.id
              )?.feedback ?? ''}
            </p>{/if}
          {#if grades[index]}<label
              >Nilai soal {index + 1}<input
                type="number"
                min="0"
                max={question.points}
                step="1"
                bind:value={grades[index].score}
                required
                disabled={busy || conflict}
              /></label
            ><label
              >Feedback soal {index + 1}<textarea
                rows="3"
                bind:value={grades[index].feedback}
                maxlength="4000"
                disabled={busy || conflict}
              ></textarea></label
            >{/if}
        </section>
      {/each}
      <label
        >Keaktifan (0–100, opsional)<input
          type="number"
          min="0"
          max="100"
          step="1"
          bind:value={keaktifan}
          disabled={busy || conflict}
        /></label
      >
      <label
        >Alasan penilaian / perubahan (audit)<textarea
          bind:value={reason}
          required
          maxlength="1000"
          disabled={busy || conflict}
        ></textarea></label
      >
      <label class="check"
        ><input
          type="checkbox"
          bind:checked={publish}
          disabled={busy || conflict}
        />Saya telah meninjau jawaban dan rubric; publikasikan nilai untuk
        mahasiswa</label
      >
      <button
        disabled={busy ||
          conflict ||
          !valid ||
          review.attempt.status === 'in_progress'}
        >{publish
          ? 'Publikasikan nilai manual'
          : 'Simpan penilaian manual'}</button
      >
    </form>
  {/if}
</section>
