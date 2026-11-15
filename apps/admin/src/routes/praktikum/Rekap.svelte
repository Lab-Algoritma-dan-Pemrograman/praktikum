<script lang="ts">
  import type { Principal } from '@algohub/auth-client';
  import {
    PracticumClient,
    permitted,
    attemptLabels,
    errorMessage,
    jakarta,
    type SessionDetail
  } from '@algohub/practicum-client';
  export let detail: SessionDetail;
  export let principal: Principal | null;
  const client = new PracticumClient();
  let rows: RekapRow[] = [];
  let courseId = '';
  let offset = 0;
  let loading = false;
  let issue = '';
  interface RekapRow {
    attempt_id: string;
    user_id: number;
    user_name: string;
    status: 'in_progress' | 'submitted' | 'graded';
    score: number | null;
    max_score: number;
    percentage: number | null;
    keaktifan?: number | null;
    submitted_at: string | null;
  }
  function canRead() {
    return detail.activations.some(
      (a) =>
        a.course_id === courseId &&
        permitted(principal, 'practicum.grade.write', a.scope)
    );
  }
  async function load(next = offset) {
    if (!courseId) {
      rows = [];
      return;
    }
    if (!canRead()) {
      issue = 'Tidak ada izin rekap untuk course ini.';
      rows = [];
      return;
    }
    loading = true;
    try {
      rows = await client.get<RekapRow[]>(
        `/admin/praktikum/rekap?${new URLSearchParams({ course_id: courseId, limit: '25', offset: String(next) })}`
      );
      offset = next;
      issue = '';
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loading = false;
    }
  }
</script>

<div class="stack">
  <h2>Rekap nilai per course</h2>
  <p class="muted">
    Nilai attempt yang sudah dikumpulkan pada satu course. Untuk course tanpa
    keaktifan, kolom itu kosong.
  </p>
  <div class="actions">
    <label
      >Course<select
        bind:value={courseId}
        onchange={() => {
          offset = 0;
          void load(0);
        }}
        disabled={loading}
        ><option value="" disabled>Pilih course</option
        >{#each detail.courses as course}<option value={course.id}
            >{course.title}</option
          >{/each}</select
      ></label
    ><button
      class="secondary"
      onclick={() => load()}
      disabled={loading || !courseId}>Perbarui rekap</button
    >
  </div>
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if loading}<p role="status">Memuat rekap…</p>{/if}
  <p class="muted" id="rekap-scroll">
    Pada layar kecil, geser tabel secara horizontal untuk melihat semua kolom.
  </p>
  <!-- svelte-ignore a11y_no_noninteractive_tabindex (Horizontal table scrolling must be keyboard reachable.) -->
  <div
    class="table-wrap"
    role="region"
    aria-label="Tabel rekap nilai"
    aria-describedby="rekap-scroll"
    tabindex="0"
  >
    <table>
      <caption>Nilai peserta per course</caption><thead
        ><tr
          ><th>Peserta</th><th>Status</th><th>Skor</th><th>Keaktifan</th><th
            >Waktu submit</th
          ></tr
        ></thead
      ><tbody
        >{#each rows as row}<tr data-attempt-id={row.attempt_id}
            ><td
              >{row.user_name}<br /><span class="muted">ID {row.user_id}</span
              ></td
            ><td>{attemptLabels[row.status] ?? row.status}</td><td
              >{row.score ?? 'Belum dinilai'} / {row.max_score}<br /><span
                class="muted"
                >{row.percentage === null
                  ? '—'
                  : `${row.percentage.toFixed(1)}%`}</span
              ></td
            ><td>{row.keaktifan ?? '—'}</td><td>{jakarta(row.submitted_at)}</td
            ></tr
          >{:else}<tr
            ><td colspan="5">
              {courseId
                ? 'Belum ada nilai pada course ini.'
                : 'Pilih course untuk melihat rekap.'}</td
            ></tr
          >{/each}</tbody
      >
    </table>
  </div>
  <div class="actions">
    <button
      class="secondary"
      disabled={offset === 0 || loading}
      onclick={() => load(offset - 25)}>Sebelumnya</button
    ><button
      class="secondary"
      disabled={rows.length < 25 || loading}
      onclick={() => load(offset + 25)}>Berikutnya</button
    >
  </div>
</div>
