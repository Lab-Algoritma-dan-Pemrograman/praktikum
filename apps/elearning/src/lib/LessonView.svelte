<script lang="ts">
  import { onMount } from 'svelte';
  import { ApiError } from '@algohub/auth-client';
  import BrowserPreview from '@algohub/ui/BrowserPreview.svelte';
  import {
    warmupPreview,
    type BrowserVerificationResult,
    type PlaygroundFallbackResult
  } from '@algohub/ui';
  export let runtimeOrigin: string;
  import {
    ElearningClient,
    LessonWorkspace,
    command,
    reusableCommand,
    errorMessage,
    jakarta,
    type BrowserTestAttestation,
    type Command,
    type LearningAchievement,
    type LearningAIHintResponse,
    type LearningHintResponse,
    type LessonWorkspaceState
  } from '@algohub/elearning-client';
  export let id: string;
  const client = new ElearningClient();
  const workspace = new LessonWorkspace(client, id, (next) => (state = next));
  let state: LessonWorkspaceState = workspace.state;
  let stdin = '';
  let browserAttestation: BrowserTestAttestation | null = null;
  let hintPending: Command | null = null;
  let hintBusy = false;
  let hintIssue = '';
  let hintData: LearningHintResponse | null = null;
  let aiPending: Command | null = null;
  let aiBusy = false;
  let aiIssue = '';
  let aiData: LearningAIHintResponse | null = null;
  let completeNotice = '';
  let newlyUnlocked: LearningAchievement[] = [];

  $: language = state.view?.lesson.language ?? 'c';
  $: codeRunRequired = Boolean(
    state.view?.lesson.starter_code || state.view?.lesson.test_cases.length
  );
  $: verificationCases =
    state.view?.lesson.test_cases.map((testCase) => ({
      description: testCase.description,
      input: testCase.input,
      expectedOutput: testCase.expected_output
    })) ?? [];
  $: attestationValid = Boolean(
    state.view &&
    browserAttestation?.lessonId === state.view.lesson.id &&
    browserAttestation.lessonVersion === state.view.lesson.version &&
    browserAttestation.language === state.view.lesson.language &&
    browserAttestation.source === state.draftCode
  );

  function handleVerification(result: BrowserVerificationResult | null) {
    browserAttestation =
      result?.passed && state.view && result.source === state.draftCode
        ? {
            lessonId: state.view.lesson.id,
            lessonVersion: state.view.lesson.version,
            language: result.language,
            source: result.source
          }
        : null;
  }

  async function askHint() {
    if (!state.view) return;
    hintBusy = true;
    hintIssue = '';
    hintPending = reusableCommand(hintPending, '/learning/hints', 'POST', {
      lesson_id: state.view.lesson.id,
      draft_code: state.draftCode
    });
    try {
      hintData = await client.execute<LearningHintResponse>(hintPending);
      hintPending = null;
    } catch (error) {
      hintIssue = errorMessage(error);
    } finally {
      hintBusy = false;
    }
  }

  async function askAIHint() {
    if (!state.view) return;
    aiBusy = true;
    aiIssue = '';
    aiPending = reusableCommand(aiPending, '/learning/hint', 'POST', {
      lesson_id: state.view.lesson.id,
      draft_code: state.draftCode
    });
    try {
      aiData = await client.aiHint(aiPending);
      aiPending = null;
    } catch (error) {
      aiIssue =
        error instanceof ApiError &&
        (error.status === 502 || error.status === 503)
          ? 'Layanan hint AI sedang tidak tersedia. Coba lagi nanti atau minta bantuan koordinator.'
          : errorMessage(error);
    } finally {
      aiBusy = false;
    }
  }

  async function finishLesson() {
    completeNotice = '';
    newlyUnlocked = [];
    const res = await workspace.complete(
      attestationValid && browserAttestation ? browserAttestation : undefined
    );
    if (res) {
      newlyUnlocked = res.newly_unlocked ?? [];
      completeNotice = `Pelajaran selesai dikonfirmasi server (+${res.progress.xp_awarded} XP). Total XP: ${res.profile.xp} (Level ${res.profile.level}).`;
    }
  }

  onMount(() => {
    workspace.setOffline(!navigator.onLine);
    void workspace.refresh();
    const online = () => void workspace.reconnect();
    const offline = () => workspace.setOffline(true);
    window.addEventListener('online', online);
    window.addEventListener('offline', offline);
    return () => {
      workspace.dispose();
      window.removeEventListener('online', online);
      window.removeEventListener('offline', offline);
    };
  });
</script>

{#if state.offline}<p class="notice" role="status">
    Offline. Perubahan draf tetap di memori tab ini sampai koneksi pulih.
  </p>{/if}
{#if state.issue}<div class="notice error" role="alert">
    {state.issue}
    <div class="actions">
      <button
        class="secondary"
        onclick={() => workspace.reconnect()}
        disabled={state.syncing || state.saving || state.completing}
        >Hubungkan ulang / coba lagi</button
      >
    </div>
  </div>{/if}
{#if !state.view}<p role="status">
    {state.syncing ? 'Memuat pelajaran…' : 'Pelajaran belum dapat dibuka.'}
  </p>{:else}
  <article class="stack">
    <header class="panel stack">
      <p class="muted">
        {state.view.lesson.language.toUpperCase()} · +{state.view.lesson
          .xp_reward} XP · Status:
        <strong>{state.view.progress.status}</strong>
      </p>
      <h1>{state.view.lesson.title}</h1>
      <p role="status" data-testid="lesson-save-status">
        {state.saving
          ? 'Menyimpan draf…'
          : state.conflict
            ? 'Konflik draf: pilih versi'
            : state.dirty
              ? 'Ada perubahan draf belum tersimpan'
              : 'Draf tersimpan'} · Revisi {state.view.progress.revision} · Diperbarui
        {jakarta(state.view.progress.updated_at)}
      </p>
    </header>

    {#if state.conflict}
      <section class="notice" aria-labelledby="lesson-conflict-title">
        <h2 id="lesson-conflict-title">Draf berubah di server</h2>
        <p>
          Perubahan lokal tidak ditimpa otomatis. Pilih draf server atau simpan
          pilihan lokal di atas versi terbaru.
        </p>
        <pre>{state.view.progress.draft_code}</pre>
        <div class="actions">
          <button class="secondary" onclick={() => workspace.resolve(false)}
            >Gunakan draf server</button
          ><button onclick={() => workspace.resolve(true)}
            >Simpan draf lokal di atas versi terbaru</button
          >
        </div>
      </section>
    {/if}

    {#if completeNotice}
      <section class="notice success" role="status">
        <p><strong>{completeNotice}</strong></p>
        {#if newlyUnlocked.length > 0}
          <p>
            Achievement baru terbuka:
            {newlyUnlocked.map((a) => a.title).join(', ')}
          </p>
        {/if}
      </section>
    {/if}

    <section class="panel stack" aria-label="Materi pelajaran">
      <h2>Penjelasan materi</h2>
      <p>{state.view.lesson.explanation}</p>
      {#if state.view.lesson.code_example}
        <h3>Contoh kode</h3>
        <pre>{state.view.lesson.code_example}</pre>
      {/if}
    </section>

    {#if state.view.lesson.quiz}
      <fieldset class="panel stack">
        <legend>Kuis pemahaman — {state.view.lesson.quiz.question}</legend>
        {#each state.view.lesson.quiz.options as option, index}
          <label class="check">
            <input
              type="radio"
              name="lesson-quiz"
              value={index}
              checked={state.quizOption === index}
              onchange={() => workspace.selectQuiz(index)}
            />
            {option}
          </label>
        {/each}
      </fieldset>
    {/if}

    <section class="panel stack" aria-label="Code lab pelajaran">
      <h2>Latihan kode ({state.view.lesson.language.toUpperCase()})</h2>
      <label
        >Kode latihan<textarea
          class="code"
          rows="10"
          spellcheck="false"
          autocomplete="off"
          maxlength="65536"
          value={state.draftCode}
          oninput={(event) => workspace.editCode(event.currentTarget.value)}
          onfocusin={() => warmupPreview(runtimeOrigin, language)}
        ></textarea></label
      >
      <label
        >Input standar latihan<textarea
          class="code"
          rows="2"
          maxlength="8192"
          bind:value={stdin}
        ></textarea></label
      >
      <BrowserPreview
        {runtimeOrigin}
        {language}
        source={state.draftCode}
        {stdin}
        verify={codeRunRequired}
        {verificationCases}
        onVerification={handleVerification}
        serverRun={(input) =>
          client.execute<PlaygroundFallbackResult>(
            command('/playground/run', 'POST', input)
          )}
      />
      {#if codeRunRequired}
        <p class="notice" role="status" data-testid="lesson-run-requirement">
          Testcase dijalankan di browser untuk XP gamifikasi. Hasil ini bukan
          verifikasi nilai akademik dan harus diulang jika kode atau versi
          pelajaran berubah.
        </p>
      {/if}
      <div class="actions">
        <button
          class="secondary"
          onclick={askHint}
          disabled={hintBusy || state.offline}
          >{hintBusy ? 'Meminta hint…' : 'Minta hint'}</button
        >
        <button
          class="secondary"
          onclick={askAIHint}
          disabled={aiBusy || state.offline}
          >{aiBusy ? 'Menulis hint AI…' : 'Minta hint AI'}</button
        >
        <button
          class="secondary"
          onclick={() => workspace.flush()}
          disabled={state.saving || state.offline || state.conflict}
          >Simpan draf</button
        >
        <button
          onclick={finishLesson}
          disabled={state.completing ||
            state.offline ||
            state.conflict ||
            (codeRunRequired && !attestationValid)}
          >{state.completing
            ? 'Menyelesaikan…'
            : state.view.progress.status === 'completed'
              ? 'Perbarui penyelesaian (XP sudah diklaim)'
              : `Selesaikan pelajaran (+${state.view.lesson.xp_reward} XP)`}</button
        >
      </div>
      {#if hintIssue}<p class="notice error" role="alert">{hintIssue}</p>{/if}
      {#if hintData}
        <section class="notice" aria-label="Hint pelajaran">
          <h3>Hint belajar</h3>
          <p>{hintData.hint}</p>
          <p class="muted">
            Sisa kuota hint menit ini: {hintData.remaining_quota}
          </p>
        </section>
      {/if}
      {#if aiIssue}<p class="notice error" role="alert">{aiIssue}</p>{/if}
      {#if aiData}
        <section class="notice" aria-label="Hint AI pelajaran">
          <h3>Hint AI</h3>
          <p data-testid="ai-hint-text">{aiData.hint}</p>
          <p class="muted">
            Sisa kuota hint AI menit ini: {aiData.remaining_quota}
          </p>
        </section>
      {/if}
    </section>
  </article>
{/if}
