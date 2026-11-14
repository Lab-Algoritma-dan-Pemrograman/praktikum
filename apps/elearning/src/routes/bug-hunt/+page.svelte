<script lang="ts">
  import { onMount } from 'svelte';
  import {
    ElearningClient,
    reusableCommand,
    errorMessage,
    jakarta,
    type Command,
    type LearningBugHuntSubmitView,
    type LearningBugHuntView
  } from '@algohub/elearning-client';
  const client = new ElearningClient();
  let view: LearningBugHuntView | null = null;
  let selectedId = '';
  let selectedLine = 1;
  let explanation = '';
  let loading = false;
  let busy = false;
  let issue = '';
  let lastResult: LearningBugHuntSubmitView | null = null;
  let pending: Command | null = null;

  $: activeQuestion =
    view?.questions.find((q) => q.id === selectedId) ??
    view?.questions[0] ??
    null;
  $: numberedCode = activeQuestion
    ? activeQuestion.buggy_code
        .split('\n')
        .map((line, i) => `${i + 1}: ${line}`)
        .join('\n')
    : '';

  async function load() {
    loading = true;
    issue = '';
    try {
      view = await client.bugHunt();
      if (!selectedId && view.questions.length > 0) {
        selectedId = view.questions[0].id;
      }
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loading = false;
    }
  }

  async function submitAttempt() {
    if (!activeQuestion) return;
    busy = true;
    issue = '';
    lastResult = null;
    pending = reusableCommand(pending, '/learning/bug-hunt/attempts', 'POST', {
      question_id: activeQuestion.id,
      selected_line: selectedLine,
      explanation
    });
    try {
      lastResult = await client.execute<LearningBugHuntSubmitView>(pending);
      pending = null;
      await load();
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      busy = false;
    }
  }

  onMount(() => {
    void load();
  });
</script>

<svelte:head><title>Bug Hunt — E-learning</title></svelte:head>
<section class="stack" aria-labelledby="bughunt-title">
  <h1 id="bughunt-title">Tantangan Bug Hunt</h1>
  <p>
    Temukan baris yang mengandung bug pada kode C atau Python dan jelaskan
    alasannya. XP diberikan satu kali untuk setiap tantangan yang belum pernah
    diselesaikan.
  </p>
  {#if loading}<p role="status">Memuat tantangan Bug Hunt…</p>{/if}
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if lastResult}
    <section
      class="notice"
      class:success={lastResult.attempt.correct}
      role="status"
    >
      <h2>
        {lastResult.attempt.correct
          ? `Tepat! +${lastResult.attempt.xp_earned} XP`
          : 'Belum tepat'}
      </h2>
      <p>{lastResult.attempt.feedback}</p>
      {#if lastResult.newly_unlocked.length > 0}
        <p>
          Achievement terbuka:
          {lastResult.newly_unlocked.map((a) => a.title).join(', ')}
        </p>
      {/if}
    </section>
  {/if}
  {#if view}
    <p class="panel" data-testid="bughunt-quota">
      Kuota mingguan terpakai: <strong
        >{view.weekly_used} / {view.weekly_limit}</strong
      >
      · Total XP Anda: <strong>{view.profile.xp} XP</strong>
    </p>
    {#if activeQuestion}
      <form
        class="panel stack"
        onsubmit={(e) => {
          e.preventDefault();
          void submitAttempt();
        }}
      >
        <label
          >Pilih tantangan Bug Hunt<select bind:value={selectedId}>
            {#each view.questions as q}<option value={q.id}
                >{q.title} ({q.language.toUpperCase()} · +{q.xp_reward} XP{q.solved
                  ? ' · Selesai'
                  : ''})</option
              >{/each}
          </select></label
        >
        <p class="muted">
          Kesulitan: {activeQuestion.difficulty} · Hadiah: +{activeQuestion.xp_reward}
          XP {activeQuestion.solved ? '· Sudah pernah diselesaikan' : ''}
        </p>
        <!-- svelte-ignore a11y_no_noninteractive_tabindex (Scrollable code block must be keyboard reachable.) -->
        <pre
          class="runner-output"
          tabindex="0"
          aria-label="Kode tantangan Bug Hunt">{numberedCode}</pre>
        <label
          >Baris bug<input
            type="number"
            min="1"
            max="500"
            bind:value={selectedLine}
            required
          /></label
        >
        <label
          >Penjelasan bug<textarea
            rows="3"
            minlength="5"
            maxlength="2000"
            bind:value={explanation}
            required
          ></textarea></label
        >
        <button
          disabled={busy ||
            view.weekly_used >= view.weekly_limit ||
            explanation.trim().length < 5}
          >{busy ? 'Memeriksa…' : 'Kirim analisis Bug Hunt'}</button
        >
      </form>
    {:else}
      <p class="notice">Belum ada tantangan Bug Hunt yang aktif.</p>
    {/if}
    <section class="stack" aria-label="Riwayat Bug Hunt">
      <h2>Riwayat percobaan saya</h2>
      {#each view.attempts as attempt}
        <article class="panel" data-bughunt-attempt={attempt.attempt_id}>
          <p>
            <strong>{attempt.question_id}</strong> · Baris {attempt.selected_line}
            · {attempt.correct
              ? `Benar (+${attempt.xp_earned} XP)`
              : 'Belum tepat'}
          </p>
          <p>{attempt.feedback}</p>
          <p class="muted">{jakarta(attempt.played_at)}</p>
        </article>
      {:else}
        <p class="notice">Belum ada riwayat percobaan Bug Hunt.</p>
      {/each}
    </section>
  {/if}
</section>
