<script lang="ts">
  import { onMount } from 'svelte';
  import { beforeNavigate } from '$app/navigation';
  import {
    PracticumClient,
    errorMessage,
    jakarta,
    attemptLabels,
    questionTypeLabels,
    difficultyLabels,
    submissionLabel,
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
  // Questions the student has typed into; once edited, the draft is shown
  // verbatim so clearing the editor does not snap back to the starter code.
  let edited = new Set<string>();
  function editorValue(questionId: string, starter: string): string {
    const draft = state.drafts[questionId]?.value;
    if (edited.has(questionId)) return draft ?? '';
    return draft || starter || '';
  }
  function onEdit(questionId: string, value: string, language?: string) {
    edited.add(questionId);
    workspace.edit(questionId, value, language);
  }
  let submitDialog: HTMLDialogElement | undefined;
  let submitTrigger: HTMLButtonElement | undefined;
  let cancelSubmit: HTMLButtonElement | undefined;
  function openConfirmation() {
    confirmSubmit = true;
    if (submitDialog?.isConnected && !submitDialog.open) {
      submitDialog.showModal();
      cancelSubmit?.focus();
    }
  }
  function closeConfirmation() {
    submitDialog?.close();
    confirmSubmit = false;
    if (submitTrigger?.isConnected) submitTrigger.focus();
    else document.getElementById('praktikum-content')?.focus();
  }
  $: if (confirmSubmit && state.attempt?.status !== 'in_progress')
    closeConfirmation();
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
  beforeNavigate(({ cancel, from, to, willUnload }) => {
    // In-page question links retain this workspace and all drafts.
    if (
      !willUnload &&
      from &&
      to &&
      from.url.origin === to.url.origin &&
      from.url.pathname === to.url.pathname &&
      from.url.search === to.url.search
    )
      return;
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
    workspace.refresh().then(() => {
      if (
        state.attempt?.status === 'in_progress' &&
        state.attempt.questions.some((question) => question.type === 'coding')
      )
        void loadRunner();
    });
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

<div class="workspace-heading">
  <div>
    <a class="back-link" href="/nilai">← Nilai & riwayat</a>
    <p class="eyebrow">PENGERJAAN PRAKTIKUM</p>
    <h1>
      {state.attempt && state.attempt.status !== 'in_progress'
        ? 'Jawaban & hasil praktikum'
        : 'Ruang pengerjaan'}
    </h1>
  </div>
  {#if state.attempt}<a
      class="back-link"
      href={`/sesi/${encodeURIComponent(state.attempt.session_id)}`}
      >Lihat sesi</a
    >{/if}
</div>
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
    {state.syncing ? 'Memuat pengerjaan…' : 'Pengerjaan belum dapat dibuka.'}
  </p>{:else}
  <section class="panel workspace-status" aria-label="Status attempt">
    <div>
      <strong
        >{attemptLabels[state.attempt.status] ??
          'Status pengerjaan belum tersedia'}</strong
      >
      <p class="muted">
        Deadline: {jakarta(state.attempt.deadline_at)}<br />Waktu mengikuti
        server, bukan jam perangkat.
      </p>
    </div>
    {#if state.attempt.status === 'in_progress'}<div>
        <span class="muted">Sisa waktu</span>
        <p class="countdown" role="timer" aria-label="Sisa waktu">
          {timerLabel(remaining)}
        </p>
      </div>{/if}
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
      Pengumpulan dikonfirmasi server: {submissionLabel(
        state.attempt.submission_reason
      )}. Jawaban terkunci. <a href="/nilai">Lihat status nilai</a>
    </p>{/if}
  {#if runnerIssue}<p class="notice">
      Runner: {runnerIssue}
      <button class="secondary" onclick={loadRunner}>Muat ulang runner</button>
    </p>{/if}
  <nav class="question-nav" aria-label="Navigasi soal">
    {#each state.attempt.questions as question, index}<a
        href={`#question-${question.id}`}>Soal {index + 1}</a
      >{/each}
  </nav>
  <div class="stack">
    {#each state.attempt.questions as question, index (question.id)}
      <section class="panel stack" aria-labelledby={`question-${question.id}`}>
        <h2 id={`question-${question.id}`}>
          Soal {index + 1} · {question.points} poin
        </h2>
        <p class="muted">
          {questionTypeLabels[question.type] ?? 'Soal'} · {difficultyLabels[
            question.difficulty
          ] ?? 'Tingkat tidak tersedia'}
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
            value={editorValue(question.id, question.starter_code)}
            language={state.drafts[question.id]?.language ?? question.language}
            disabled={!editable}
            offline={state.offline}
            {runner}
            update={(value, language) => onEdit(question.id, value, language)}
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
        bind:this={submitTrigger}
        disabled={!editable || state.offline}
        onclick={openConfirmation}>Kumpulkan jawaban</button
      >
    </div>
  {/if}
  <dialog
    class="submit-dialog"
    bind:this={submitDialog}
    aria-labelledby="submit-title"
    aria-describedby="submit-description"
    oncancel={(event) => {
      if (state.submitting) event.preventDefault();
    }}
    onclose={() => {
      // Native close events are queued; an earlier Escape must not close a
      // confirmation that the user has already opened again.
      if (submitDialog?.open) return;
      confirmSubmit = false;
      if (submitTrigger?.isConnected) submitTrigger.focus();
    }}
  >
    <h2 id="submit-title">Kumpulkan sekarang?</h2>
    <p id="submit-description">
      Semua perubahan disimpan terlebih dahulu. Setelah server mengonfirmasi,
      jawaban tidak dapat diubah.
    </p>
    {#if state.issue}<p class="notice error" role="alert">
        {state.issue}
      </p>{/if}
    {#if state.offline}<p class="notice" role="status">
        Anda sedang offline. Sambungkan kembali sebelum mengumpulkan.
      </p>{/if}
    {#if state.conflict}<p class="notice" role="status">
        Batalkan konfirmasi dan selesaikan konflik jawaban terlebih dahulu.
      </p>{/if}
    <div class="actions">
      <button
        class="secondary"
        bind:this={cancelSubmit}
        disabled={state.submitting}
        onclick={closeConfirmation}>Batal</button
      >
      <button
        disabled={state.submitting ||
          state.offline ||
          state.conflict ||
          state.blocked}
        onclick={async () => {
          if (await workspace.submit()) closeConfirmation();
        }}
        >{state.submitting
          ? 'Menyimpan dan mengumpulkan…'
          : 'Ya, kumpulkan'}</button
      >
    </div>
  </dialog>
{/if}

<style>
  .workspace-status {
    display: flex;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 1rem;
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
