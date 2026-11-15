<script lang="ts">
  import { ApiError, type Principal } from '@algohub/auth-client';
  import {
    PracticumClient,
    permitted,
    reusableCommand,
    errorMessage,
    courseLabels,
    type Command,
    type Course,
    type CourseKind,
    type Question,
    type QuestionType,
    type Rubric,
    type SessionDetail
  } from '@algohub/practicum-client';
  export let detail: SessionDetail;
  export let principal: Principal | null;
  export let refresh: () => Promise<void>;
  const client = new PracticumClient();
  let title = detail.session.title;
  let sequence = detail.session.sequence;
  let active = detail.session.active;
  let sessionVersion = detail.session.version;
  let courseId = '';
  let courseTitle = '';
  let kind: CourseKind = 'pretest';
  let duration = 30;
  let tokenRequired = false;
  let courseVersion = 0;
  let questions: Question[] = [];
  let questionOffset = 0;
  let questionId = '';
  let questionVersion = 0;
  let questionType: QuestionType = 'essay';
  let difficulty: Question['difficulty'] = 'easy';
  let category = '';
  let prompt = '';
  let points = 10;
  let starterCode = '';
  let language = 'c';
  let referenceAnswer = '';
  let rubric: Rubric[] = [
    { id: 'criterion-1', description: 'Ketepatan jawaban', max_score: 10 }
  ];
  let issue = '';
  let notice = '';
  let busy = false;
  let conflict = false;
  let pending: Command | null = null;
  $: scope = { ...detail.session.scope, session_id: detail.session.id };
  $: canDefine = permitted(principal, 'practicum.session.manage', scope);
  $: canQuestions = permitted(principal, 'question.manage', scope);
  $: rubricValid =
    rubric.length > 0 &&
    rubric.every(
      (r) =>
        r.description.trim() &&
        Number.isInteger(r.max_score) &&
        r.max_score >= 0
    ) &&
    rubric.reduce((sum, r) => sum + r.max_score, 0) === points;
  function chooseCourse(id: string) {
    const course = detail.courses.find((c) => c.id === id);
    courseId = id;
    courseTitle = course?.title ?? '';
    kind = course?.kind ?? 'pretest';
    duration = course?.duration_minutes ?? 30;
    tokenRequired = course?.token_required ?? false;
    courseVersion = course?.version ?? 0;
    questions = [];
    questionOffset = 0;
    chooseQuestion('');
    if (id && canQuestions) void loadQuestions();
  }
  function chooseQuestion(id: string) {
    const question = questions.find((q) => q.id === id);
    questionId = id;
    questionVersion = question?.version ?? 0;
    questionType = question?.type ?? 'essay';
    difficulty = question?.difficulty ?? 'easy';
    category = question?.category ?? '';
    prompt = question?.prompt ?? '';
    points = question?.points ?? 10;
    starterCode = question?.starter_code ?? '';
    language = question?.language || 'c';
    referenceAnswer = question?.reference_answer ?? '';
    rubric = question?.rubric?.map((r) => ({ ...r })) ?? [
      { id: 'criterion-1', description: 'Ketepatan jawaban', max_score: 10 }
    ];
  }
  async function loadQuestions(next = questionOffset) {
    try {
      questions = await client.get<Question[]>(
        `/admin/praktikum/questions?course_id=${encodeURIComponent(courseId)}&limit=25&offset=${next}`
      );
      questionOffset = next;
    } catch (error) {
      issue = errorMessage(error);
    }
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
      notice =
        'Perubahan tersimpan. Snapshot activation yang sudah ada tidak berubah.';
    } catch (error) {
      issue = errorMessage(error);
      if (error instanceof ApiError && error.status === 409) {
        conflict = true;
        try {
          await refresh();
          if (courseId && canQuestions) await loadQuestions();
        } catch (refreshError) {
          issue = errorMessage(refreshError);
        }
      }
    } finally {
      busy = false;
    }
  }
  function resolve(keep: boolean) {
    if (!keep) {
      title = detail.session.title;
      sequence = detail.session.sequence;
      active = detail.session.active;
      chooseCourse(courseId);
    }
    sessionVersion = detail.session.version;
    courseVersion = detail.courses.find((c) => c.id === courseId)?.version ?? 0;
    questionVersion = questions.find((q) => q.id === questionId)?.version ?? 0;
    conflict = false;
    pending = null;
    issue = '';
    notice = 'Versi terbaru dimuat. Periksa form sebelum menyimpan.';
  }
  async function saveQuestion() {
    if (!canQuestions || !rubricValid) return;

    await mutate(
      `/admin/praktikum/questions${questionId ? `/${encodeURIComponent(questionId)}` : ''}`,
      questionId ? 'PATCH' : 'POST',
      {
        ...(questionId
          ? { version: questionVersion }
          : { course_id: courseId }),
        type: questionType,
        difficulty,
        category,
        prompt,
        points,
        starter_code: questionType === 'coding' ? starterCode : '',
        language: questionType === 'coding' ? language : '',
        reference_answer: referenceAnswer,
        rubric
      },
      async (value: Question) => {
        await loadQuestions();
        chooseQuestion(value.id);
      }
    );
  }
  async function generateRumpang() {
    if (!questionId || questionType !== 'coding' || starterCode.trim()) return;
    busy = true;
    issue = '';
    notice = '';
    pending = reusableCommand(
      pending,
      `/admin/praktikum/questions/${encodeURIComponent(questionId)}/rumpang`,
      'POST',
      {}
    );
    try {
      const value = await client.execute<Question>(pending);
      pending = null;
      await refresh();
      await loadQuestions();
      chooseQuestion(value.id);
      notice =
        'Kode rumpang dibuat dari kunci jawaban. Periksa sebelum menyimpan soal.';
    } catch (error) {
      issue = errorMessage(error);
      pending = null;
    } finally {
      busy = false;
    }
  }
</script>

<div class="stack">
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if notice}<p role="status">{notice}</p>{/if}
  {#if conflict}<section class="notice">
      <h2>Definisi berubah</h2>
      <p>
        Form lokal dipertahankan. Versi server terbaru: sesi {detail.session
          .version}. Pilih data server atau tinjau kembali form lokal sebelum
        mencoba simpan.
      </p>
      <details>
        <summary>Definisi server terbaru</summary>
        <pre>{JSON.stringify(
            {
              session: detail.session,
              course: detail.courses.find((c) => c.id === courseId),
              question: questions.find((q) => q.id === questionId)
            },
            null,
            2
          )}</pre>
      </details>
      <div class="actions">
        <button class="secondary" onclick={() => resolve(false)}
          >Gunakan definisi server</button
        ><button onclick={() => resolve(true)}>Pertahankan form lokal</button>
      </div>
    </section>{/if}
  {#if canDefine}
    <form
      class="panel stack"
      onsubmit={(event) => {
        event.preventDefault();
        void mutate(
          `/admin/praktikum/sesi/${encodeURIComponent(detail.session.id)}`,
          'PATCH',
          { version: sessionVersion, title, sequence, active },
          async () => {
            sessionVersion = detail.session.version;
          }
        );
      }}
    >
      <h2>Definisi sesi</h2>
      <label
        >Judul sesi<input bind:value={title} required maxlength="200" /></label
      ><label
        >Urutan<input
          type="number"
          min="1"
          bind:value={sequence}
          required
        /></label
      ><label class="check"
        ><input type="checkbox" bind:checked={active} />Sesi aktif</label
      ><button disabled={busy || conflict}>Simpan sesi</button>
    </form>
  {/if}
  <section class="panel stack">
    <h2>Course dan bank soal</h2>
    <label
      >Course untuk diedit<select
        value={courseId}
        onchange={(event) => chooseCourse(event.currentTarget.value)}
        disabled={busy || conflict}
        ><option value="">Course baru</option
        >{#each detail.courses as course}<option value={course.id}
            >{course.title} ({courseLabels[course.kind]})</option
          >{/each}</select
      ></label
    >
    {#if canDefine}
      <form
        class="stack"
        onsubmit={(event) => {
          event.preventDefault();
          void mutate(
            `/admin/praktikum/courses${courseId ? `/${encodeURIComponent(courseId)}` : ''}`,
            courseId ? 'PATCH' : 'POST',
            {
              ...(courseId
                ? { version: courseVersion }
                : { session_id: detail.session.id, kind }),
              title: courseTitle,
              duration_minutes: duration,
              token_required: tokenRequired
            },
            async (value: Course) => chooseCourse(value.id)
          );
        }}
      >
        <label
          >Judul course<input
            bind:value={courseTitle}
            required
            maxlength="200"
          /></label
        >
        <div class="grid">
          <label
            >Jenis course<select bind:value={kind} disabled={!!courseId}
              >{#each Object.entries(courseLabels) as [value, label]}<option
                  {value}>{label}</option
                >{/each}</select
            ></label
          ><label
            >Durasi (menit)<input
              type="number"
              min="1"
              max="180"
              bind:value={duration}
              required
            /></label
          >
        </div>
        <label class="check"
          ><input type="checkbox" bind:checked={tokenRequired} />Wajib token
          akses</label
        ><button disabled={busy || conflict}
          >{courseId ? 'Simpan course' : 'Buat course'}</button
        >
      </form>
    {/if}
  </section>
  {#if courseId && canQuestions}
    <section class="panel stack">
      <h2>Bank soal</h2>
      <p>
        Referensi jawaban dan rubric bersifat privat. Total rubric harus sama
        dengan poin soal.
      </p>
      <label
        >Soal untuk diedit<select
          value={questionId}
          onchange={(event) => chooseQuestion(event.currentTarget.value)}
          disabled={busy || conflict}
          ><option value="">Soal baru</option
          >{#each questions as question}<option value={question.id}
              >{question.type} · {question.prompt.slice(0, 70)}</option
            >{/each}</select
        ></label
      >
      <div class="actions">
        <button
          class="secondary"
          disabled={questionOffset === 0 || busy}
          onclick={() => loadQuestions(questionOffset - 25)}
          >Soal sebelumnya</button
        ><button
          class="secondary"
          disabled={questions.length < 25 || busy}
          onclick={() => loadQuestions(questionOffset + 25)}
          >Soal berikutnya</button
        >
      </div>
      <form
        class="stack"
        onsubmit={(event) => {
          event.preventDefault();
          void saveQuestion();
        }}
      >
        <div class="grid">
          <label
            >Tipe soal<select bind:value={questionType}
              ><option value="essay">Essay</option><option value="coding"
                >Coding</option
              ></select
            ></label
          ><label
            >Kesulitan<select bind:value={difficulty}
              ><option value="easy">Mudah</option><option value="medium"
                >Sedang</option
              ><option value="hard">Sulit</option></select
            ></label
          ><label>Kategori<input bind:value={category} maxlength="100" /></label
          ><label
            >Poin soal<input
              type="number"
              min="1"
              max="100"
              bind:value={points}
              required
            /></label
          >
        </div>
        <label
          >Pertanyaan<textarea
            rows="5"
            bind:value={prompt}
            required
            maxlength="8192"
          ></textarea></label
        >
        {#if questionType === 'coding'}<label
            >Bahasa awal<select bind:value={language}
              ><option value="c">C</option><option value="python">Python</option
              ></select
            ></label
          >{#if questionId && !starterCode.trim()}<div class="actions">
              <button
                type="button"
                class="secondary"
                onclick={() => void generateRumpang()}
                disabled={busy || conflict}
                >{busy
                  ? 'Menyusun kode rumpang…'
                  : 'Generate kode rumpang dari kunci'}</button
              >
            </div>{/if}<label
            >Kode awal<textarea
              class="code"
              rows="6"
              bind:value={starterCode}
              maxlength="65536"
            ></textarea></label
          >{/if}
        <label
          >Referensi jawaban (privat)<textarea
            rows="4"
            bind:value={referenceAnswer}
            maxlength="65536"
          ></textarea></label
        >
        <fieldset class="stack">
          <legend>Rubric privat</legend>{#each rubric as criterion, index}<div
              class="grid"
            >
              <label
                >Kriteria {index + 1}<input
                  bind:value={criterion.description}
                  required
                  maxlength="1000"
                /></label
              ><label
                >Poin kriteria {index + 1}<input
                  type="number"
                  min="0"
                  max="100"
                  bind:value={criterion.max_score}
                  required
                /></label
              ><button
                type="button"
                class="secondary"
                onclick={() => (rubric = rubric.filter((_, i) => i !== index))}
                >Hapus kriteria {index + 1}</button
              >
            </div>{/each}<button
            type="button"
            class="secondary"
            onclick={() =>
              (rubric = [
                ...rubric,
                { id: crypto.randomUUID(), description: '', max_score: 0 }
              ])}>Tambah kriteria</button
          >{#if !rubricValid}<p class="notice">
              Jumlah poin rubric harus tepat {points}; isi semua kriteria.
            </p>{/if}
        </fieldset>
        <button disabled={busy || conflict || !rubricValid}
          >{questionId ? 'Simpan soal' : 'Buat soal'}</button
        >
      </form>
    </section>
  {/if}
</div>
