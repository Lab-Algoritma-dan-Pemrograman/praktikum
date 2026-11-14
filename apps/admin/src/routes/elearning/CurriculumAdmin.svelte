<script lang="ts">
  import { ApiError, type Principal } from '@algohub/auth-client';
  import {
    ElearningClient,
    permitted,
    reusableCommand,
    errorMessage,
    type AccessMode,
    type Command,
    type LearningLanguage,
    type LearningLesson,
    type LearningLevel,
    type LearningModule,
    type Scope
  } from '@algohub/elearning-client';
  export let levels: LearningLevel[];
  export let selectedId: string;
  export let principal: Principal | null;
  export let refresh: () => Promise<void>;
  const client = new ElearningClient();

  let newLevelTitle = '';
  let newLevelDesc = '';
  let newLevelSort = 1;
  let newLevelAccess: AccessMode = 'unlocked';
  let newLevelXP = 0;
  let newLevelInst = 'lab';
  let newLevelDiv = 'pendidikan';

  let editLevelTitle = '';
  let editLevelDesc = '';
  let editLevelSort = 1;
  let editLevelAccess: AccessMode = 'unlocked';
  let editLevelXP = 0;
  let editLevelVersion = 1;

  let moduleId = '';
  let moduleTitle = '';
  let moduleDesc = '';
  let moduleSort = 1;
  let moduleVersion = 0;

  let lessonId = '';
  let lessonVersion = 0;
  let lessonTitle = '';
  let lessonSort = 1;
  let lessonLanguage: LearningLanguage = 'c';
  let lessonXP = 60;
  let lessonExplanation = '';
  let lessonCodeExample = '';
  let lessonStarterCode = '';
  let lessonSolution = '';
  let lessonHint = '';
  let quizQuestion = '';
  let quizOptA = '';
  let quizOptB = '';
  let quizCorrect = 0;
  let testDesc = '';
  let testInput = '';
  let testExpected = 'SIMULATED';

  let busy = false;
  let conflict = false;
  let issue = '';
  let notice = '';
  let pending: Command | null = null;

  $: level = levels.find((l) => l.id === selectedId) ?? null;
  $: if (level && editLevelVersion !== level.version && !conflict) {
    syncLevelFields(level);
  }
  $: createScope = {
    domain: 'elearning',
    ...(newLevelInst ? { institution: newLevelInst } : {}),
    ...(newLevelDiv ? { division: newLevelDiv } : {})
  } satisfies Scope;
  $: canCreateLevel = permitted(
    principal,
    'learning.content.manage',
    createScope
  );
  $: canManageSelected =
    !!level && permitted(principal, 'learning.content.manage', level.scope);

  function syncLevelFields(lvl: LearningLevel) {
    editLevelTitle = lvl.title;
    editLevelDesc = lvl.description;
    editLevelSort = lvl.sort_order;
    editLevelAccess = lvl.access_mode;
    editLevelXP = lvl.required_xp;
    editLevelVersion = lvl.version;
  }

  function chooseModule(id: string) {
    const mod = level?.modules?.find((m) => m.id === id);
    moduleId = id;
    moduleTitle = mod?.title ?? '';
    moduleDesc = mod?.description ?? '';
    moduleSort = mod?.sort_order ?? 1;
    moduleVersion = mod?.version ?? 0;
    chooseLesson('');
  }

  function chooseLesson(id: string) {
    const mod = level?.modules?.find((m) => m.id === moduleId);
    const les = mod?.lessons?.find((l) => l.id === id);
    lessonId = id;
    lessonVersion = les?.version ?? 0;
    lessonTitle = les?.title ?? '';
    lessonSort = les?.sort_order ?? 1;
    lessonLanguage = les?.language ?? 'c';
    lessonXP = les?.xp_reward ?? 60;
    lessonExplanation = les?.explanation ?? '';
    lessonCodeExample = les?.code_example ?? '';
    lessonStarterCode = les?.starter_code ?? '';
    lessonSolution = les?.solution ?? '';
    lessonHint = les?.hint ?? '';
    quizQuestion = les?.quiz?.question ?? '';
    quizOptA = les?.quiz?.options?.[0] ?? '';
    quizOptB = les?.quiz?.options?.[1] ?? '';
    quizCorrect = les?.quiz?.correct_option ?? 0;
    testDesc = les?.test_cases?.[0]?.description ?? '';
    testInput = les?.test_cases?.[0]?.input ?? '';
    testExpected = les?.test_cases?.[0]?.expected_output ?? 'SIMULATED';
  }

  async function mutate(
    path: string,
    method: string,
    body: unknown,
    after?: (value: any) => Promise<void>
  ) {
    busy = true;
    issue = '';
    notice = '';
    pending = reusableCommand(pending, path, method, body);
    try {
      const value = await client.execute(pending);
      pending = null;
      await refresh();
      if (after) await after(value);
      notice = 'Perubahan kurikulum berhasil disimpan.';
    } catch (error) {
      issue = errorMessage(error);
      if (error instanceof ApiError && error.status === 409) {
        conflict = true;
        await refresh();
      }
    } finally {
      busy = false;
    }
  }

  function resolveConflict(keepLocal: boolean) {
    if (level) {
      if (!keepLocal) {
        syncLevelFields(level);
        if (moduleId) chooseModule(moduleId);
        if (lessonId) chooseLesson(lessonId);
      } else {
        editLevelVersion = level.version;
        moduleVersion =
          level.modules?.find((m) => m.id === moduleId)?.version ?? 0;
        lessonVersion =
          level.modules
            ?.find((m) => m.id === moduleId)
            ?.lessons?.find((l) => l.id === lessonId)?.version ?? 0;
      }
    }
    conflict = false;
    pending = null;
    issue = '';
    notice = 'Versi terbaru dimuat. Periksa form sebelum menyimpan.';
  }
</script>

<div class="stack">
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if notice}<p class="notice" role="status">{notice}</p>{/if}
  {#if conflict}
    <section class="notice">
      <h2>Definisi kurikulum berubah di server</h2>
      <p>Pilih menggunakan versi server atau mempertahankan form lokal.</p>
      <div class="actions">
        <button class="secondary" onclick={() => resolveConflict(false)}
          >Gunakan definisi server</button
        ><button onclick={() => resolveConflict(true)}
          >Pertahankan form lokal</button
        >
      </div>
    </section>
  {/if}

  <div class="panel stack">
    <label
      >Pilih level kurikulum<select
        value={selectedId}
        onchange={(e) => {
          selectedId = e.currentTarget.value;
          const next = levels.find((l) => l.id === selectedId);
          if (next) syncLevelFields(next);
          chooseModule('');
        }}
      >
        <option value="" disabled>Pilih level</option>
        {#each levels as lvl}<option value={lvl.id}
            >{lvl.sort_order}. {lvl.title} ({lvl.access_mode})</option
          >{/each}
      </select></label
    >
    <div class="actions">
      <button class="secondary" onclick={refresh} disabled={busy}
        >Perbarui kurikulum</button
      >
    </div>
    {#if levels.length === 0}<p class="notice">
        Tidak ada level kurikulum dalam scope Anda.
      </p>{/if}
  </div>

  {#if principal?.grants.some((g) => g.permission === 'learning.content.manage')}
    <details class="panel">
      <summary>Buat level baru</summary>
      <form
        class="stack"
        onsubmit={(e) => {
          e.preventDefault();
          void mutate(
            '/admin/elearning/levels',
            'POST',
            {
              title: newLevelTitle,
              description: newLevelDesc,
              sort_order: newLevelSort,
              access_mode: newLevelAccess,
              required_xp: newLevelXP,
              scope: createScope
            },
            async (created: LearningLevel) => {
              newLevelTitle = '';
              newLevelDesc = '';
              selectedId = created.id;
              syncLevelFields(created);
            }
          );
        }}
      >
        <label
          >Judul level baru<input
            bind:value={newLevelTitle}
            required
            maxlength="200"
          /></label
        >
        <label
          >Deskripsi level baru<input
            bind:value={newLevelDesc}
            maxlength="2000"
          /></label
        >
        <div class="grid">
          <label
            >Urutan level<input
              type="number"
              min="1"
              max="1000"
              bind:value={newLevelSort}
              required
            /></label
          >
          <label
            >Mode akses<select bind:value={newLevelAccess}>
              <option value="unlocked">Terbuka (unlocked)</option>
              <option value="auto">Otomatis sesuai XP (auto)</option>
              <option value="locked">Terkunci (locked)</option>
            </select></label
          >
          <label
            >XP minimum<input
              type="number"
              min="0"
              max="1000000"
              bind:value={newLevelXP}
              required
            /></label
          >
        </div>
        <div class="grid">
          <label
            >Institusi level<input
              bind:value={newLevelInst}
              maxlength="100"
            /></label
          >
          <label
            >Divisi level<input
              bind:value={newLevelDiv}
              maxlength="100"
            /></label
          >
        </div>
        <button disabled={!canCreateLevel || busy || conflict}
          >Buat level</button
        >
      </form>
    </details>
  {/if}

  {#if level && canManageSelected}
    <form
      class="panel stack"
      onsubmit={(e) => {
        e.preventDefault();
        void mutate(
          `/admin/elearning/levels/${encodeURIComponent(level.id)}`,
          'PATCH',
          {
            version: editLevelVersion,
            title: editLevelTitle,
            description: editLevelDesc,
            sort_order: editLevelSort,
            access_mode: editLevelAccess,
            required_xp: editLevelXP
          }
        );
      }}
    >
      <h2>Edit level: {level.title}</h2>
      <label
        >Judul level<input
          bind:value={editLevelTitle}
          required
          maxlength="200"
        /></label
      >
      <label
        >Deskripsi level<input
          bind:value={editLevelDesc}
          maxlength="2000"
        /></label
      >
      <div class="grid">
        <label
          >Urutan<input
            type="number"
            min="1"
            max="1000"
            bind:value={editLevelSort}
            required
          /></label
        >
        <label
          >Akses level<select bind:value={editLevelAccess}>
            <option value="unlocked">Terbuka (unlocked)</option>
            <option value="auto">Otomatis sesuai XP (auto)</option>
            <option value="locked">Terkunci (locked)</option>
          </select></label
        >
        <label
          >Target XP<input
            type="number"
            min="0"
            max="1000000"
            bind:value={editLevelXP}
            required
          /></label
        >
      </div>
      <button disabled={busy || conflict}>Simpan level</button>
    </form>

    <section class="panel stack">
      <h2>Modul dalam level</h2>
      <label
        >Pilih modul<select
          value={moduleId}
          onchange={(e) => chooseModule(e.currentTarget.value)}
        >
          <option value="">Modul baru</option>
          {#each level.modules ?? [] as mod}<option value={mod.id}
              >{mod.sort_order}. {mod.title}</option
            >{/each}
        </select></label
      >
      <form
        class="stack"
        onsubmit={(e) => {
          e.preventDefault();
          void mutate(
            `/admin/elearning/modules${moduleId ? `/${encodeURIComponent(moduleId)}` : ''}`,
            moduleId ? 'PATCH' : 'POST',
            {
              ...(moduleId
                ? { version: moduleVersion }
                : { level_id: level.id }),
              title: moduleTitle,
              description: moduleDesc,
              sort_order: moduleSort
            },
            async (saved: LearningModule) => chooseModule(saved.id)
          );
        }}
      >
        <label
          >Judul modul<input
            bind:value={moduleTitle}
            required
            maxlength="200"
          /></label
        >
        <label
          >Deskripsi modul<input
            bind:value={moduleDesc}
            maxlength="2000"
          /></label
        >
        <label
          >Urutan modul<input
            type="number"
            min="1"
            max="1000"
            bind:value={moduleSort}
            required
          /></label
        >
        <button disabled={busy || conflict}
          >{moduleId ? 'Simpan modul' : 'Buat modul'}</button
        >
      </form>
    </section>

    {#if moduleId}
      <section class="panel stack">
        <h2>Pelajaran dalam modul</h2>
        <label
          >Pilih pelajaran<select
            value={lessonId}
            onchange={(e) => chooseLesson(e.currentTarget.value)}
          >
            <option value="">Pelajaran baru</option>
            {#each level.modules?.find((m) => m.id === moduleId)?.lessons ?? [] as les}
              <option value={les.id}
                >{les.sort_order}. {les.title} ({les.language.toUpperCase()})</option
              >
            {/each}
          </select></label
        >
        <form
          class="stack"
          onsubmit={(e) => {
            e.preventDefault();
            const quiz =
              quizQuestion.trim() && quizOptA.trim() && quizOptB.trim()
                ? {
                    question: quizQuestion.trim(),
                    options: [quizOptA.trim(), quizOptB.trim()],
                    correct_option: quizCorrect
                  }
                : null;
            const test_cases = testDesc.trim()
              ? [
                  {
                    description: testDesc.trim(),
                    input: testInput,
                    expected_output: testExpected
                  }
                ]
              : [];
            void mutate(
              `/admin/elearning/lessons${lessonId ? `/${encodeURIComponent(lessonId)}` : ''}`,
              lessonId ? 'PATCH' : 'POST',
              {
                ...(lessonId
                  ? { version: lessonVersion, module_id: moduleId }
                  : { module_id: moduleId }),
                title: lessonTitle,
                sort_order: lessonSort,
                language: lessonLanguage,
                explanation: lessonExplanation,
                code_example: lessonCodeExample,
                starter_code: lessonStarterCode,
                solution: lessonSolution,
                hint: lessonHint,
                xp_reward: lessonXP,
                quiz,
                test_cases
              },
              async (saved: LearningLesson) => chooseLesson(saved.id)
            );
          }}
        >
          <div class="grid">
            <label
              >Judul pelajaran<input
                bind:value={lessonTitle}
                required
                maxlength="200"
              /></label
            >
            <label
              >Urutan pelajaran<input
                type="number"
                min="1"
                max="1000"
                bind:value={lessonSort}
                required
              /></label
            >
            <label
              >Bahasa pelajaran<select bind:value={lessonLanguage}>
                <option value="c">C</option>
                <option value="python">Python</option>
              </select></label
            >
            <label
              >Hadiah XP<input
                type="number"
                min="10"
                max="500"
                bind:value={lessonXP}
                required
              /></label
            >
          </div>
          <label
            >Penjelasan materi<textarea
              rows="4"
              bind:value={lessonExplanation}
              required
              maxlength="32768"
            ></textarea></label
          >
          <label
            >Contoh kode<textarea
              class="code"
              rows="3"
              bind:value={lessonCodeExample}
              maxlength="32768"
            ></textarea></label
          >
          <label
            >Kode awal latihan<textarea
              class="code"
              rows="3"
              bind:value={lessonStarterCode}
              maxlength="32768"
            ></textarea></label
          >
          <label
            >Solusi referensi (privat)<textarea
              class="code"
              rows="3"
              bind:value={lessonSolution}
              maxlength="32768"
            ></textarea></label
          >
          <label
            >Hint belajar<input
              bind:value={lessonHint}
              maxlength="4000"
            /></label
          >
          <fieldset class="stack">
            <legend>Kuis pelajaran (opsional)</legend>
            <label
              >Pertanyaan kuis<input
                bind:value={quizQuestion}
                maxlength="2000"
              /></label
            >
            <div class="grid">
              <label
                >Opsi kuis 1<input
                  bind:value={quizOptA}
                  maxlength="500"
                /></label
              >
              <label
                >Opsi kuis 2<input
                  bind:value={quizOptB}
                  maxlength="500"
                /></label
              >
              <label
                >Indeks jawaban benar<select bind:value={quizCorrect}>
                  <option value={0}>Opsi 1 (0)</option>
                  <option value={1}>Opsi 2 (1)</option>
                </select></label
              >
            </div>
          </fieldset>
          <fieldset class="stack">
            <legend>Kasus uji kode (opsional)</legend>
            <div class="grid">
              <label
                >Deskripsi kasus uji<input
                  bind:value={testDesc}
                  maxlength="500"
                /></label
              >
              <label
                >Input kasus uji<input
                  bind:value={testInput}
                  maxlength="4000"
                /></label
              >
              <label
                >Output diharapkan<input
                  bind:value={testExpected}
                  maxlength="4000"
                /></label
              >
            </div>
          </fieldset>
          <button disabled={busy || conflict}
            >{lessonId ? 'Simpan pelajaran' : 'Buat pelajaran'}</button
          >
        </form>
      </section>
    {/if}
  {/if}
</div>
