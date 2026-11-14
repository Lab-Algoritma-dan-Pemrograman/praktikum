<script lang="ts">
  import { onMount } from 'svelte';
  import { scopeLabel, type Principal } from '@algohub/auth-client';
  import {
    PracticumClient,
    permitted,
    reusableCommand,
    errorMessage,
    jakarta,
    type Activation,
    type Command,
    type Question,
    type Scope,
    type SessionDetail
  } from '@algohub/practicum-client';
  export let detail: SessionDetail;
  export let principal: Principal | null;
  export let refresh: () => Promise<void>;
  const client = new PracticumClient();
  const grant = principal?.grants.find(
    (g) =>
      g.permission === 'practicum.session.manage' &&
      (!g.scope.session_id || g.scope.session_id === detail.session.id)
  );
  let courseId = detail.courses[0]?.id ?? '';
  let classId = grant?.scope.class_id ?? '';
  let shift = grant?.scope.shift || 1;
  let timerMode: 'shared' | 'individual' = 'shared';
  let roster = '';
  let approved = '';
  let closesAt = '';
  let questions: Question[] = [];
  let selected: string[] = [];
  let questionOffset = 0;
  let questionIssue = '';
  let loadingQuestions = false;
  let busy = false;
  let issue = '';
  let notice = '';
  let token = '';
  let pending: Command | null = null;
  let closeId = '';
  $: scope = {
    ...detail.session.scope,
    domain: 'praktikum',
    session_id: detail.session.id,
    class_id: classId,
    shift
  } satisfies Scope;
  $: canActivate = permitted(principal, 'practicum.session.manage', scope);
  $: canBrowseQuestions = permitted(principal, 'question.manage', {
    ...detail.session.scope,
    session_id: detail.session.id
  });
  $: course = detail.courses.find((c) => c.id === courseId);
  function ids(value: string): number[] {
    return value.trim() ? [...new Set(value.split(/[\s,]+/).map(Number))] : [];
  }
  function validIds(values: number[]) {
    return (
      values.length > 0 &&
      values.every((id) => Number.isSafeInteger(id) && id > 0)
    );
  }
  async function loadQuestions(next = 0) {
    if (!courseId || !canBrowseQuestions) return;
    loadingQuestions = true;
    questionIssue = '';
    try {
      questions = await client.get<Question[]>(
        `/admin/praktikum/questions?course_id=${encodeURIComponent(courseId)}&limit=25&offset=${next}`
      );
      questionOffset = next;
    } catch (error) {
      questionIssue = errorMessage(error);
    } finally {
      loadingQuestions = false;
    }
  }
  async function activate() {
    issue = '';
    notice = '';
    const studentIds = ids(roster),
      approvedIds = ids(approved);
    if (
      !validIds(studentIds) ||
      (approvedIds.length > 0 && !validIds(approvedIds))
    ) {
      issue = 'Gunakan ID mahasiswa positif, dipisahkan koma atau spasi.';
      return;
    }
    if (approvedIds.some((id) => !studentIds.includes(id))) {
      issue = 'Persetujuan masuk harus merupakan anggota roster.';
      return;
    }
    if (course?.kind === 'ujian_praktik' && approvedIds.length === 0) {
      issue = 'Ujian praktik memerlukan daftar persetujuan masuk eksplisit.';
      return;
    }
    if (
      !canActivate ||
      !classId.trim() ||
      selected.length < 1 ||
      selected.length > 20
    ) {
      issue = 'Pilih kelas/shift yang diizinkan dan 1–20 soal.';
      return;
    }
    busy = true;
    const body = {
      course_id: courseId,
      scope,
      timer_mode: timerMode,
      student_ids: studentIds,
      entry_approved_ids: approvedIds,
      question_ids: selected,
      ...(closesAt
        ? { closes_at: new Date(`${closesAt}:00+07:00`).toISOString() }
        : {})
    };
    pending = reusableCommand(
      pending,
      `/admin/praktikum/sesi/${encodeURIComponent(detail.session.id)}/activate`,
      'POST',
      body
    );
    try {
      const activation = await client.execute<Activation>(pending);
      pending = null;
      token = activation.access_token ?? '';
      notice =
        'Activation dibuat. Roster, soal, dan timer telah dibekukan; retake memerlukan activation baru.';
      await refresh();
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      busy = false;
    }
  }
  async function close(activation: Activation) {
    if (!permitted(principal, 'practicum.session.manage', activation.scope))
      return;
    busy = true;
    issue = '';
    notice = '';
    pending = reusableCommand(
      pending,
      `/admin/praktikum/sesi/${encodeURIComponent(detail.session.id)}/courses/${encodeURIComponent(activation.course_id)}/close`,
      'POST',
      { activation_id: activation.id }
    );
    try {
      await client.execute(pending);
      pending = null;
      closeId = '';
      notice = 'Activation ditutup. Attempt aktif dikumpulkan oleh server.';
      await refresh();
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      busy = false;
    }
  }
  onMount(() => {
    if (grant) void loadQuestions();
  });
</script>

<div class="stack">
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if notice}<p class="notice" role="status">{notice}</p>{/if}
  {#if token}<section class="notice" aria-label="Token activation">
      <h2>Token akses — hanya ditampilkan sekali</h2>
      <p>
        Sampaikan melalui kanal kelas yang aman. Token tidak disimpan di browser
        dan tidak dapat ditampilkan kembali.
      </p>
      <output>{token}</output>
      <div class="actions">
        <button class="secondary" onclick={() => (token = '')}
          >Sembunyikan token</button
        >
      </div>
    </section>{/if}
  {#if grant}
    <form
      class="panel stack"
      onsubmit={(event) => {
        event.preventDefault();
        void activate();
      }}
    >
      <h2>Buka activation</h2>
      <p>
        Roster dan soal menjadi snapshot tetap. Timer bersama dimulai saat
        peserta pertama mulai; timer individual saat masing-masing mulai.
      </p>
      <label
        >Course activation<select
          bind:value={courseId}
          onchange={() => {
            selected = [];
            void loadQuestions();
          }}
          required
          disabled={busy}
          >{#each detail.courses as course}<option value={course.id}
              >{course.title}</option
            >{/each}</select
        ></label
      >
      <div class="grid">
        <label
          >Kelas<input bind:value={classId} required maxlength="100" /></label
        ><label
          >Shift<select bind:value={shift}
            ><option value={1}>Shift 1</option><option value={2}>Shift 2</option
            ></select
          ></label
        ><label
          >Mode timer<select bind:value={timerMode}
            ><option value="shared">Bersama</option><option value="individual"
              >Individual</option
            ></select
          ></label
        >
      </div>
      <label
        >ID mahasiswa roster<textarea
          rows="3"
          bind:value={roster}
          required
          placeholder="1, 2, 3"
        ></textarea></label
      >
      <label
        >ID mahasiswa disetujui masuk {course?.kind === 'ujian_praktik'
          ? '(wajib untuk ujian)'
          : '(opsional)'}<input
          bind:value={approved}
          placeholder="Subset ID roster"
        /></label
      >
      <label
        >Tutup otomatis (WIB, opsional)<input
          type="datetime-local"
          bind:value={closesAt}
        /></label
      >
      {#if !canBrowseQuestions}
        <label
          >ID soal yang disetujui pengelola bank soal<textarea
            rows="3"
            value={selected.join(', ')}
            oninput={(event) =>
              (selected = [
                ...new Set(
                  event.currentTarget.value.split(/[\s,]+/).filter(Boolean)
                )
              ])}
            required
          ></textarea></label
        >
        <p class="notice">
          Izin activation tidak memberi akses rubric atau bank soal privat.
          Gunakan ID soal dari pengelola; server memvalidasi course dan snapshot
          soal.
        </p>
      {:else}
        <fieldset class="stack">
          <legend>Pilih 1–20 soal · dipilih {selected.length}</legend>
          {#if loadingQuestions}<p role="status">Memuat pilihan soal…</p>{/if}
          {#if questionIssue}<p class="notice error">{questionIssue}</p>
            <button
              type="button"
              class="secondary"
              onclick={() => loadQuestions()}>Coba muat soal</button
            >{/if}
          {#each questions as question}<label class="check"
              ><input
                type="checkbox"
                value={question.id}
                bind:group={selected}
              />{question.type} · {question.points} poin · {question.prompt.slice(
                0,
                120
              )}</label
            >{:else}{#if !loadingQuestions}<p>
                Belum ada soal. Minta pengelola bank soal menambahkan soal.
              </p>{/if}{/each}
          <div class="actions">
            <button
              type="button"
              class="secondary"
              disabled={questionOffset === 0 || loadingQuestions}
              onclick={() => loadQuestions(questionOffset - 25)}
              >Soal sebelumnya</button
            ><button
              type="button"
              class="secondary"
              disabled={questions.length < 25 || loadingQuestions}
              onclick={() => loadQuestions(questionOffset + 25)}
              >Soal berikutnya</button
            >
          </div>
        </fieldset>
      {/if}
      {#if !canActivate}<p class="notice">
          Tidak ada grant session.manage untuk scope activation ini.
        </p>{/if}
      <button
        disabled={busy ||
          !canActivate ||
          selected.length === 0 ||
          selected.length > 20 ||
          !courseId}>{busy ? 'Memproses…' : 'Aktifkan snapshot'}</button
      >
    </form>
  {/if}
  <section class="stack">
    <h2>Activation sesi</h2>
    {#each detail.activations as activation}<article
        class="panel"
        data-activation-id={activation.id}
      >
        <h3>
          {detail.courses.find((c) => c.id === activation.course_id)?.title ??
            activation.course_id} · {activation.is_open ? 'Terbuka' : 'Ditutup'}
        </h3>
        <p class="muted">{scopeLabel(activation.scope)}</p>
        <p>
          {activation.question_count} soal · {activation.timer_mode} · ID {activation.id}
        </p>
        <p>
          Deadline: {jakarta(activation.deadline_at)}<br />Penutupan: {jakarta(
            activation.closes_at
          )}
        </p>
        <details>
          <summary>Snapshot roster dan soal</summary>
          <p class="prompt">
            Roster: {(activation.student_ids ?? []).join(', ')}<br />Disetujui
            masuk: {(activation.entry_approved_ids ?? []).join(', ')}<br />ID
            soal: {(activation.question_ids ?? []).join(', ')}
          </p>
        </details>
        {#if activation.is_open && permitted(principal, 'practicum.session.manage', activation.scope)}
          <button
            class="danger"
            disabled={busy}
            onclick={() => (closeId = activation.id)}>Tutup activation</button
          >
          {#if closeId === activation.id}<section class="notice">
              <h3>Tutup course untuk activation ini?</h3>
              <p>
                Semua attempt aktif dikumpulkan. Penutupan permanen; clock tidak
                dapat direset. Retake harus memakai activation baru.
              </p>
              <div class="actions">
                <button
                  class="danger"
                  disabled={busy}
                  onclick={() => close(activation)}>Ya, tutup activation</button
                ><button
                  class="secondary"
                  disabled={busy}
                  onclick={() => (closeId = '')}>Batal</button
                >
              </div>
            </section>{/if}
        {/if}
      </article>{:else}<p class="notice">
        Belum ada activation dalam scope Anda.
      </p>{/each}
  </section>
</div>
