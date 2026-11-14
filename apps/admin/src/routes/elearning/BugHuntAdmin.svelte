<script lang="ts">
  import { onMount } from 'svelte';
  import type { Principal } from '@algohub/auth-client';
  import {
    ElearningClient,
    permitted,
    reusableCommand,
    errorMessage,
    type Command,
    type LearningBugHuntQuestion,
    type LearningLanguage,
    type Scope
  } from '@algohub/elearning-client';
  export let principal: Principal | null;
  const client = new ElearningClient();
  let questions: LearningBugHuntQuestion[] = [];
  let selectedId = '';
  let version = 0;
  let title = '';
  let language: LearningLanguage = 'c';
  let difficulty: 'easy' | 'medium' | 'hard' = 'easy';
  let bugLine = 1;
  let xpReward = 50;
  let buggyCode = '';
  let explanation = '';
  let active = true;
  let institution = 'lab';
  let division = 'pendidikan';
  let busy = false;
  let issue = '';
  let notice = '';
  let pending: Command | null = null;

  $: scope = {
    domain: 'elearning',
    ...(institution ? { institution } : {}),
    ...(division ? { division } : {})
  } satisfies Scope;
  $: canSave = permitted(principal, 'learning.content.manage', scope);

  function choose(id: string) {
    const q = questions.find((item) => item.id === id);
    selectedId = id;
    version = q?.version ?? 0;
    title = q?.title ?? '';
    language = q?.language ?? 'c';
    difficulty = q?.difficulty ?? 'easy';
    bugLine = q?.bug_line ?? 1;
    xpReward = q?.xp_reward ?? 50;
    buggyCode = q?.buggy_code ?? '';
    explanation = q?.explanation ?? '';
    active = q?.active ?? true;
    institution = q?.scope.institution ?? 'lab';
    division = q?.scope.division ?? 'pendidikan';
  }

  async function load() {
    issue = '';
    try {
      questions = await client.get<LearningBugHuntQuestion[]>(
        '/admin/elearning/bug-hunt?limit=25&offset=0'
      );
    } catch (error) {
      issue = errorMessage(error);
    }
  }

  async function save() {
    if (!canSave) return;
    busy = true;
    issue = '';
    notice = '';
    const path = `/admin/elearning/bug-hunt${selectedId ? `/${encodeURIComponent(selectedId)}` : ''}`;
    const method = selectedId ? 'PATCH' : 'POST';
    pending = reusableCommand(pending, path, method, {
      ...(selectedId ? { version } : {}),
      title,
      language,
      difficulty,
      bug_line: bugLine,
      xp_reward: xpReward,
      buggy_code: buggyCode,
      explanation,
      active,
      scope
    });
    try {
      const saved = await client.execute<LearningBugHuntQuestion>(pending);
      pending = null;
      await load();
      choose(saved.id);
      notice = 'Tantangan Bug Hunt berhasil disimpan.';
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

<section class="panel stack">
  <h2>Kelola tantangan Bug Hunt</h2>
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if notice}<p class="notice" role="status">{notice}</p>{/if}
  <label
    >Tantangan Bug Hunt untuk diedit<select
      value={selectedId}
      onchange={(e) => choose(e.currentTarget.value)}
    >
      <option value="">Tantangan baru</option>
      {#each questions as q}<option value={q.id}
          >{q.title} ({q.language.toUpperCase()} · baris {q.bug_line})</option
        >{/each}
    </select></label
  >
  <form
    class="stack"
    onsubmit={(e) => {
      e.preventDefault();
      void save();
    }}
  >
    <div class="grid">
      <label
        >Judul tantangan<input
          bind:value={title}
          required
          maxlength="200"
        /></label
      >
      <label
        >Bahasa Bug Hunt<select bind:value={language}>
          <option value="c">C</option>
          <option value="python">Python</option>
        </select></label
      >
      <label
        >Kesulitan Bug Hunt<select bind:value={difficulty}>
          <option value="easy">Mudah</option>
          <option value="medium">Sedang</option>
          <option value="hard">Sulit</option>
        </select></label
      >
      <label
        >Baris bug (privat)<input
          type="number"
          min="1"
          max="500"
          bind:value={bugLine}
          required
        /></label
      >
      <label
        >Hadiah XP Bug Hunt<input
          type="number"
          min="10"
          max="300"
          bind:value={xpReward}
          required
        /></label
      >
    </div>
    <label
      >Kode mengandung bug<textarea
        class="code"
        rows="6"
        bind:value={buggyCode}
        required
        maxlength="32768"
      ></textarea></label
    >
    <label
      >Penjelasan bug (privat)<textarea
        rows="3"
        bind:value={explanation}
        required
        minlength="5"
        maxlength="4000"
      ></textarea></label
    >
    <label class="check"
      ><input type="checkbox" bind:checked={active} />Tantangan aktif</label
    >
    <button disabled={!canSave || busy}
      >{selectedId ? 'Simpan tantangan' : 'Buat tantangan'}</button
    >
  </form>
</section>
