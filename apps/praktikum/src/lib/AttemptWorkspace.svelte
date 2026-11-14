<script lang="ts">
  import { onMount } from 'svelte';
  import { beforeNavigate } from '$app/navigation';
  import {
    PracticumClient,
    errorMessage,
    jakarta,
    type RunnerInfo
  } from '@algohub/practicum-client';
  import {
    Workspace,
    timerLabel,
    type WorkspaceState
  } from '@algohub/practicum-client/workspace';
  import CodeEditor from './CodeEditor.svelte';
  export let id: string;
  export let runtimeOrigin: string;
  const client = new PracticumClient();
  const workspace = new Workspace(client, id, (next) => (state = next));
  let state: WorkspaceState = workspace.state;
  let remaining = 0;
  let runner: RunnerInfo | null = null;
  let runnerIssue = '';
  let confirmSubmit = false;
  let nextDeadlineRetry = 0;
  let mounted = false;
  $: lastSaved =
    state.attempt?.answers
      .map((answer) => answer.updated_at ?? '')
      .filter(Boolean)
      .sort()
      .at(-1) ?? null;
  $: editable =
    state.attempt?.status === 'in_progress' &&
    remaining > 0 &&
    !state.blocked &&
    !state.conflict &&
    !state.submitting;
  beforeNavigate(({ cancel }) => {
    if (
      (state.dirty || state.saving || state.submitting) &&
      !window.confirm('Perubahan belum tersimpan. Tetap tinggalkan halaman?')
    )
      cancel();
  });
  async function loadRunner() {
    runnerIssue = '';
    try {
      runner = await client.get<RunnerInfo>('/practicum/runner');
    } catch (error) {
      runnerIssue = errorMessage(error);
    }
  }
  onMount(() => {
    mounted = true;
    workspace.setOffline(!navigator.onLine);
    void workspace.refresh();
    void loadRunner();
    const tick = setInterval(() => {
      remaining = workspace.clock.milliseconds();
      if (
        state.attempt?.status === 'in_progress' &&
        remaining <= 0 &&
        !state.offline &&
        performance.now() >= nextDeadlineRetry
      ) {
        nextDeadlineRetry = performance.now() + 10_000;
        void workspace.submit();
      }
    }, 250);
    const poll = setInterval(() => {
      if (!document.hidden) void workspace.poll();
    }, 15_000);
    const online = () => void workspace.reconnect();
    const offline = () => workspace.setOffline(true);
    const visible = () => {
      if (!document.hidden) void workspace.reconnect();
    };
    const unload = (event: BeforeUnloadEvent) => {
      if (state.dirty || state.saving || state.submitting) {
        event.preventDefault();
        event.returnValue = '';
      }
    };
    window.addEventListener('online', online);
    window.addEventListener('offline', offline);
    window.addEventListener('beforeunload', unload);
    document.addEventListener('visibilitychange', visible);
    return () => {
      workspace.dispose();
      clearInterval(tick);
      clearInterval(poll);
      window.removeEventListener('online', online);
      window.removeEventListener('offline', offline);
      window.removeEventListener('beforeunload', unload);
      document.removeEventListener('visibilitychange', visible);
    };
  });
</script>

<h1>Ruang pengerjaan</h1>
{#if state.offline}<p class="notice" role="status">
    Offline. Perubahan tetap di memori tab ini. Jangan tutup atau muat ulang
    halaman sampai tersimpan.
  </p>{/if}
{#if state.issue}<div class="notice error" role="alert">
    {state.issue}
    <div class="actions">
      <button
        onclick={() => workspace.reconnect()}
        disabled={state.syncing ||
          state.saving ||
          state.submitting ||
          state.offline}>Hubungkan ulang / coba lagi</button
      >{#if state.blocked}<a href="/login" target="_blank" rel="noopener"
          >Masuk di tab baru</a
        >{/if}
    </div>
  </div>{/if}
{#if !state.attempt}<p role="status">
    {state.syncing ? 'Memuat attempt…' : 'Attempt belum dapat dibuka.'}
  </p>{:else}
  <section class="panel workspace-status" aria-label="Status attempt">
    <div>
      <strong
        >{state.attempt.status === 'in_progress'
          ? 'Sedang dikerjakan'
          : state.attempt.status === 'graded'
            ? 'Nilai dipublikasikan'
            : 'Terkumpul'}</strong
      >
      <p class="muted">
        Deadline: {jakarta(state.attempt.deadline_at)}<br />Waktu mengikuti
        server, bukan jam perangkat.
      </p>
    </div>
    <div>
      <span class="muted">Sisa waktu</span>
      <p class="countdown" role="timer" aria-label="Sisa waktu">
        {timerLabel(remaining)}
      </p>
    </div>
  </section>
  <p role="status" data-testid="save-status">
    {state.saving
      ? 'Menyimpan…'
      : state.conflict
        ? 'Konflik: pilihan Anda diperlukan'
        : state.dirty
          ? 'Ada perubahan belum tersimpan'
          : 'Semua jawaban tersimpan'} · Revisi {state.attempt
      .revision}{#if lastSaved}
      · Tersimpan {jakarta(lastSaved)}{/if}
  </p>
  {#if state.conflict}
    <section class="notice" aria-labelledby="conflict-title">
      <h2 id="conflict-title">Jawaban berubah di server</h2>
      <p>
        Perubahan lokal tidak ditimpa. Bandingkan jawaban terbaru sebelum
        memilih; penyimpanan otomatis dijeda.
      </p>
      {#each state.attempt.questions as question, index}<details>
          <summary>Jawaban server soal {index + 1}</summary>
          <pre>{state.attempt.answers.find((a) => a.question_id === question.id)
              ?.value || '(kosong)'}</pre>
        </details>{/each}
      <div class="actions">
        <button class="secondary" onclick={() => workspace.resolve(false)}
          >Gunakan jawaban server</button
        ><button
          disabled={remaining <= 0 || state.attempt.status !== 'in_progress'}
          onclick={() => workspace.resolve(true)}
          >Simpan pilihan lokal di atas versi terbaru</button
        >
      </div>
    </section>
  {/if}
  {#if remaining <= 0 && state.attempt.status === 'in_progress'}<p
      class="notice"
      role="status"
    >
      Waktu habis. Editor dikunci; menunggu konfirmasi pengumpulan dari server.
      Jawaban yang terlambat tidak dapat disimpan.
    </p>{/if}
  {#if state.attempt.status !== 'in_progress'}<p class="notice">
      Pengumpulan dikonfirmasi server ({state.attempt.submission_reason ??
        'selesai'}). Jawaban terkunci. <a href="/nilai">Lihat status nilai</a>
    </p>{/if}
  {#if runnerIssue}<p class="notice">
      Runner: {runnerIssue}
      <button class="secondary" onclick={loadRunner}>Muat ulang runner</button>
    </p>{/if}
  <div class="stack">
    {#each state.attempt.questions as question, index}
      <section class="panel stack" aria-labelledby={`question-${question.id}`}>
        <h2 id={`question-${question.id}`}>
          Soal {index + 1} · {question.points} poin
        </h2>
        <p class="muted">
          {question.type} · {question.difficulty}
          {question.category ? `· ${question.category}` : ''}
        </p>
        <p class="prompt">{question.prompt}</p>
        {#if question.type === 'coding'}
          {#if question.starter_code}<details>
              <summary>Kode awal</summary>
              <pre>{question.starter_code}</pre>
              <button
                class="secondary"
                disabled={!editable || !!state.drafts[question.id]?.value}
                onclick={() =>
                  workspace.edit(
                    question.id,
                    question.starter_code,
                    question.language
                  )}>Gunakan kode awal</button
              >
            </details>{/if}
          <CodeEditor
            {runtimeOrigin}
            attemptId={id}
            questionId={question.id}
            number={index + 1}
            value={state.drafts[question.id]?.value ?? ''}
            language={state.drafts[question.id]?.language ?? question.language}
            disabled={!editable}
            offline={state.offline}
            {runner}
            update={(value, language) =>
              workspace.edit(question.id, value, language)}
          />
        {:else}
          <label
            >Jawaban soal {index + 1}<textarea
              rows="8"
              maxlength="65536"
              autocomplete="off"
              readonly={!editable}
              value={state.drafts[question.id]?.value ?? ''}
              oninput={(event) =>
                workspace.edit(question.id, event.currentTarget.value)}
            ></textarea></label
          >
        {/if}
        {#if state.attempt.status === 'graded'}<p>
            Nilai: {state.attempt.answers.find(
              (a) => a.question_id === question.id
            )?.score ?? '—'} / {question.points}
          </p>
          <p class="prompt">
            {state.attempt.answers.find((a) => a.question_id === question.id)
              ?.feedback ?? ''}
          </p>{/if}
      </section>
    {/each}
  </div>
  {#if state.attempt.status === 'in_progress'}
    <div class="actions submit-actions">
      <button
        class="secondary"
        disabled={!mounted ||
          state.saving ||
          state.submitting ||
          state.offline ||
          state.conflict ||
          remaining <= 0}
        onclick={() => workspace.flush()}>Simpan sekarang</button
      ><button
        disabled={!editable || state.offline}
        onclick={() => (confirmSubmit = true)}>Kumpulkan jawaban</button
      >
    </div>
    {#if confirmSubmit}<section
        class="notice"
        aria-label="Konfirmasi pengumpulan"
      >
        <h2>Kumpulkan sekarang?</h2>
        <p>
          Semua perubahan disimpan terlebih dahulu. Setelah server
          mengonfirmasi, jawaban tidak dapat diubah.
        </p>
        <div class="actions">
          <button
            disabled={state.submitting || state.offline || state.conflict}
            onclick={async () => {
              if (await workspace.submit()) confirmSubmit = false;
            }}
            >{state.submitting
              ? 'Menyimpan dan mengumpulkan…'
              : 'Ya, kumpulkan'}</button
          ><button
            class="secondary"
            disabled={state.submitting}
            onclick={() => (confirmSubmit = false)}>Batal</button
          >
        </div>
      </section>{/if}
  {/if}
{/if}

<style>
  .workspace-status {
    display: flex;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 1rem;
    position: sticky;
    top: 0;
    z-index: 1;
    border-color: var(--ah-console-line-strong);
  }
  .countdown {
    font:
      700 1.8rem ui-monospace,
      monospace;
    margin: 0.25rem 0;
  }
  .submit-actions {
    margin: 1.5rem 0;
  }
</style>
