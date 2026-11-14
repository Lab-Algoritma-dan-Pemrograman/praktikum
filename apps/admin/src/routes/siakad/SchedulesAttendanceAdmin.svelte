<script lang="ts">
  import { onMount } from 'svelte';
  import { hasPermission, type Principal } from '@algohub/auth-client';
  import {
    SiakadClient,
    permitted,
    reusableCommand,
    versionedCommand,
    PendingCommands,
    refreshAttendanceDrafts,
    emptyPage,
    loadResourcePage,
    mergeResources,
    errorMessage,
    jakarta,
    type AcademicAttendanceRecord,
    type AcademicAttendanceSession,
    type AcademicClass,
    type AcademicSchedule,
    type AcademicScheduleView,
    type AttendanceStatus,
    type AttendanceDraftConflict,
    type DayOfWeek
  } from '@algohub/siakad-client';
  export let classes: AcademicClass[];
  export let principal: Principal | null;
  export let active = true;
  export let classId = '';
  let mounted = false;
  $: if (mounted && active) void load();
  const client = new SiakadClient();
  let schedules: AcademicScheduleView[] = [];
  let records: AcademicAttendanceRecord[] = [];
  let schedulePage = emptyPage<AcademicScheduleView>();
  let recordPage = emptyPage<AcademicAttendanceRecord>();
  let loadingSchedules = false;
  let loadingRecords = false;
  let dayOfWeek: DayOfWeek = 'Senin';
  let shift = 1;
  let room = 'Lab Dasar 1';
  let startsAt = '08:00 WIB';
  let endsAt = '10:00 WIB';

  let scheduleId = '';
  let sessionTitle = 'Pertemuan Laboratorium';
  let durationMinutes = 30;
  let qrToken = '';

  let validateStatus: Record<string, AttendanceStatus> = {};
  let validateNotes: Record<string, string> = {};
  let validationConflicts: Record<string, AttendanceDraftConflict> = {};

  let busy = false;
  let issue = '';
  let notice = '';
  const pending = new PendingCommands();

  $: if (!classId && classes.length > 0) classId = classes[0].id;
  $: if (!scheduleId && schedules.length > 0)
    scheduleId = schedules[0].schedule.id;
  $: hasScheduleWrite = hasPermission(
    principal?.grants ?? [],
    'schedule.write',
    'siakad'
  );
  $: selectedClass = classes.find((cls) => cls.id === classId);
  $: selectedSchedule = schedules.find(
    (s) => s.schedule.id === scheduleId
  )?.schedule;
  $: canCreateSchedule =
    !!selectedClass &&
    permitted(principal, 'schedule.write', {
      ...selectedClass.scope,
      class_id: selectedClass.id,
      shift
    }) &&
    permitted(principal, 'schedule.write', {
      ...selectedClass.scope,
      class_id: selectedClass.id,
      shift: selectedClass.shift
    });
  $: canOpenSession =
    !!selectedSchedule &&
    permitted(principal, 'schedule.write', {
      ...selectedSchedule.scope,
      class_id: selectedSchedule.class_id,
      shift: selectedSchedule.shift
    });

  function canValidate(record: AcademicAttendanceRecord): boolean {
    const scope = {
      ...record.scope,
      class_id: record.class_id,
      session_id: record.session_id
    };
    return (
      permitted(principal, 'attendance.read', scope) ||
      permitted(principal, 'schedule.write', scope)
    );
  }

  async function loadSchedules(more = false) {
    if (loadingSchedules) return;
    loadingSchedules = true;
    issue = '';
    const selected = schedules.find((row) => row.schedule.id === scheduleId);
    try {
      schedulePage = await loadResourcePage(
        client,
        '/admin/siakad/jadwal',
        schedulePage,
        (row) => row.schedule.id,
        more,
        selected ? [selected] : []
      );
      schedules = schedulePage.rows;
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loadingSchedules = false;
    }
  }

  async function loadRecords(more = false) {
    if (loadingRecords) return;
    loadingRecords = true;
    issue = '';
    const drafts = records.filter(
      (rec) =>
        validateStatus[rec.id] !== rec.status ||
        validateNotes[rec.id] !== rec.notes ||
        !!validationConflicts[rec.id] ||
        !!pending.get(
          `/admin/siakad/absensi/records/${encodeURIComponent(rec.id)}`,
          'PATCH'
        )
    );
    try {
      recordPage = await loadResourcePage(
        client,
        '/admin/siakad/absensi/records',
        recordPage,
        (row) => row.id,
        more,
        drafts
      );
      const refreshedDrafts = refreshAttendanceDrafts(
        records,
        recordPage.rows,
        validateStatus,
        validateNotes,
        pending,
        validationConflicts
      );
      records = recordPage.rows;
      validateStatus = refreshedDrafts.statuses;
      validateNotes = refreshedDrafts.notes;
      validationConflicts = refreshedDrafts.conflicts;
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loadingRecords = false;
    }
  }

  async function load() {
    await Promise.all([loadSchedules(), loadRecords()]);
  }

  async function createSchedule() {
    if (busy || !canCreateSchedule || !classId) return;
    busy = true;
    issue = '';
    notice = '';
    const path = '/admin/siakad/jadwal';
    const mutation = pending.put(
      reusableCommand(pending.get(path, 'POST'), path, 'POST', {
        class_id: classId,
        day_of_week: dayOfWeek,
        shift,
        room,
        starts_at: startsAt,
        ends_at: endsAt
      })
    );
    try {
      const created = await client.execute<AcademicSchedule>(mutation);
      pending.complete(mutation);
      schedules = mergeResources(
        schedules,
        [{ schedule: created, open_session: null }],
        (row) => row.schedule.id
      );
      schedulePage = { ...schedulePage, rows: schedules };
      scheduleId = created.id;
      notice = 'Jadwal laboratorium berhasil dibuat.';
    } catch (error) {
      pending.failed(mutation, error);
      issue = errorMessage(error);
    } finally {
      busy = false;
    }
  }

  async function openSession() {
    if (busy || !canOpenSession || !scheduleId) return;
    busy = true;
    issue = '';
    notice = '';
    const path = '/admin/siakad/absensi/sesi';
    const mutation = pending.put(
      reusableCommand(pending.get(path, 'POST'), path, 'POST', {
        schedule_id: scheduleId,
        title: sessionTitle,
        duration_minutes: durationMinutes
      })
    );
    try {
      const session = await client.execute<AcademicAttendanceSession>(mutation);
      pending.complete(mutation);
      schedules = schedules.map((row) =>
        row.schedule.id === session.schedule_id
          ? { ...row, open_session: session }
          : row
      );
      schedulePage = { ...schedulePage, rows: schedules };
      qrToken = session.qr_token ?? '';
      notice = qrToken
        ? `Sesi presensi ${session.id} dibuka.`
        : `Sesi presensi ${session.id} sudah dibuka, tetapi token sekali-pakai tidak dapat diambil ulang. Tutup sesi ini lalu buka sesi baru.`;
      await loadRecords();
    } catch (error) {
      pending.failed(mutation, error);
      issue = errorMessage(error);
    } finally {
      busy = false;
    }
  }

  async function closeSession(id: string) {
    const session = schedules.find(
      (s) => s.open_session?.id === id
    )?.open_session;
    if (
      busy ||
      !session ||
      !permitted(principal, 'schedule.write', session.scope)
    )
      return;
    busy = true;
    issue = '';
    notice = '';
    const path = `/admin/siakad/absensi/sesi/${encodeURIComponent(id)}/close`;
    const mutation = pending.put(
      reusableCommand(pending.get(path, 'POST'), path, 'POST', {})
    );
    try {
      await client.execute(mutation);
      pending.complete(mutation);
      schedules = schedules.map((row) =>
        row.open_session?.id === id ? { ...row, open_session: null } : row
      );
      schedulePage = { ...schedulePage, rows: schedules };
      qrToken = '';
      notice = `Sesi presensi ${id} ditutup.`;
    } catch (error) {
      pending.failed(mutation, error);
      issue = errorMessage(error);
    } finally {
      busy = false;
    }
  }

  async function validateRecord(rec: AcademicAttendanceRecord) {
    if (busy || !canValidate(rec) || validationConflicts[rec.id]) return;
    busy = true;
    issue = '';
    notice = '';
    const path = `/admin/siakad/absensi/records/${encodeURIComponent(rec.id)}`;
    const mutation = pending.put(
      versionedCommand(pending.get(path, 'PATCH'), path, {
        version: rec.version,
        status: validateStatus[rec.id] ?? rec.status,
        validated: true,
        notes: validateNotes[rec.id] ?? ''
      })
    );
    try {
      const updated = await client.execute<AcademicAttendanceRecord>(mutation);
      pending.complete(mutation);
      records = mergeResources(records, [updated], (row) => row.id);
      recordPage = { ...recordPage, rows: records };
      validateStatus[rec.id] = updated.status;
      validateNotes[rec.id] = updated.notes;
      notice = `Presensi ${rec.user_name} berhasil divalidasi.`;
    } catch (error) {
      pending.failed(mutation, error);
      issue = errorMessage(error);
    } finally {
      busy = false;
    }
  }

  function resolveValidationConflict(
    rec: AcademicAttendanceRecord,
    useServer: boolean
  ) {
    if (busy || !canValidate(rec)) return;
    if (useServer) {
      validateStatus[rec.id] = rec.status;
      validateNotes[rec.id] = rec.notes;
    }
    const { [rec.id]: _resolved, ...remaining } = validationConflicts;
    validationConflicts = remaining;
  }

  onMount(() => {
    mounted = true;
  });
</script>

<div class="stack">
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if notice}<p class="notice" role="status">{notice}</p>{/if}
  {#if qrToken}
    <section class="notice" aria-label="Token QR presensi">
      <h2>Token QR presensi — hanya ditampilkan sekali</h2>
      <p>
        Bagikan kepada mahasiswa di kelas. Token tidak disimpan dalam bentuk
        teks biasa di server.
      </p>
      <output data-testid="qr-token-output">{qrToken}</output>
      <div class="actions">
        <button class="secondary" onclick={() => (qrToken = '')}
          >Sembunyikan token QR</button
        >
      </div>
    </section>
  {/if}

  {#if hasScheduleWrite}
    <details class="panel">
      <summary>Buat jadwal laboratorium baru</summary>
      <form
        class="stack"
        onsubmit={(e) => {
          e.preventDefault();
          void createSchedule();
        }}
      >
        <div class="grid">
          <label
            >Kelas jadwal<select bind:value={classId} required>
              {#each classes as cls}<option value={cls.id}
                  >{cls.code} — {cls.course_name}</option
                >{/each}
            </select></label
          >
          <label
            >Hari<select bind:value={dayOfWeek}>
              {#each ['Senin', 'Selasa', 'Rabu', 'Kamis', 'Jumat', 'Sabtu'] as day}
                <option value={day}>{day}</option>
              {/each}
            </select></label
          >
          <label
            >Shift jadwal<select bind:value={shift}>
              <option value={1}>Shift 1</option>
              <option value={2}>Shift 2</option>
            </select></label
          >
          <label
            >Ruangan<input bind:value={room} required maxlength="200" /></label
          >
          <label
            >Jam mulai<input
              bind:value={startsAt}
              required
              maxlength="200"
            /></label
          >
          <label
            >Jam selesai<input
              bind:value={endsAt}
              required
              maxlength="200"
            /></label
          >
        </div>
        <button disabled={busy || !canCreateSchedule || !classId}
          >Buat jadwal</button
        >
      </form>
    </details>
  {/if}

  {#if hasScheduleWrite}
    <form
      class="panel stack"
      onsubmit={(e) => {
        e.preventDefault();
        void openSession();
      }}
    >
      <h2>Buka sesi presensi QR</h2>
      <div class="grid">
        <label
          >Jadwal sesi presensi<select bind:value={scheduleId} required>
            {#each schedules as s}<option value={s.schedule.id}
                >{s.schedule.class_code} · {s.schedule.day_of_week} Shift {s
                  .schedule.shift}</option
              >{/each}
          </select></label
        >
        <label
          >Judul pertemuan<input
            bind:value={sessionTitle}
            required
            maxlength="200"
          /></label
        >
        <label
          >Durasi QR (menit)<input
            type="number"
            min="1"
            max="240"
            bind:value={durationMinutes}
            required
          /></label
        >
      </div>
      <button disabled={busy || !canOpenSession || !scheduleId}
        >Buka sesi presensi QR</button
      >
    </form>
  {/if}

  <section class="stack" aria-label="Daftar jadwal dan sesi terbuka">
    <h2>Jadwal aktif</h2>
    {#each schedules as s}
      <article class="panel stack" data-admin-schedule={s.schedule.id}>
        <h3>
          {s.schedule.class_code} — {s.schedule.course_name} ({s.schedule
            .day_of_week} · Shift {s.schedule.shift})
        </h3>
        <p class="muted">
          {s.schedule.room} · {s.schedule.starts_at} – {s.schedule.ends_at}
        </p>
        {#if s.open_session}
          <p>
            Sesi terbuka: <strong>{s.open_session.title}</strong> ({s
              .open_session.id}) · Berakhir {jakarta(s.open_session.expires_at)}
          </p>
          {#if permitted(principal, 'schedule.write', s.open_session.scope)}
            <div class="actions">
              <button
                class="danger"
                disabled={busy}
                onclick={() => closeSession(s.open_session!.id)}
                >Tutup sesi presensi</button
              >
            </div>
          {/if}
        {/if}
      </article>
    {/each}
    {#if schedulePage.hasMore}
      <button
        class="secondary"
        disabled={busy || loadingSchedules}
        onclick={() => loadSchedules(true)}>Muat jadwal lainnya</button
      >
    {/if}
  </section>

  <section class="stack" aria-label="Validasi presensi peserta">
    <div class="actions">
      <h2>Validasi presensi peserta</h2>
      <button
        class="secondary"
        onclick={load}
        disabled={busy || loadingSchedules || loadingRecords}
        >Perbarui presensi</button
      >
    </div>
    {#each records as rec}
      <article class="panel stack" data-admin-attendance={rec.id}>
        <p>
          <strong>{rec.user_name}</strong> (ID {rec.user_id}) · Sesi {rec.session_id}
          · Status saat ini: <strong>{rec.status}</strong> ({rec.validated
            ? 'Tervalidasi'
            : 'Belum divalidasi'})
        </p>
        <div class="grid">
          <label
            >Status validasi {rec.user_name}<select
              bind:value={validateStatus[rec.id]}
              disabled={busy || !canValidate(rec)}
            >
              <option value="hadir">Hadir</option>
              <option value="izin">Izin</option>
              <option value="terlambat">Terlambat</option>
              <option value="alpha">Alpha</option>
            </select></label
          >
          <label
            >Catatan validasi {rec.user_name}<input
              bind:value={validateNotes[rec.id]}
              disabled={busy || !canValidate(rec)}
              maxlength="500"
            /></label
          >
        </div>
        {#if validationConflicts[rec.id]}
          <div class="notice error stack" role="alert">
            <p>
              Data presensi berubah pada kolom yang Anda edit. Periksa data
              server dan draft sebelum melanjutkan validasi.
            </p>
            <p>
              Data server — status: <strong>{rec.status}</strong>; catatan:
              {rec.notes || '(kosong)'}
            </p>
            <p>
              Draft Anda — status: <strong>{validateStatus[rec.id]}</strong>;
              catatan: {validateNotes[rec.id] || '(kosong)'}
            </p>
            <div class="actions">
              <button
                class="secondary"
                disabled={busy || !canValidate(rec)}
                onclick={() => resolveValidationConflict(rec, true)}
                >Gunakan data server</button
              >
              <button
                class="secondary"
                disabled={busy || !canValidate(rec)}
                onclick={() => resolveValidationConflict(rec, false)}
                >Terapkan perubahan saya</button
              >
            </div>
          </div>
        {/if}
        <div class="actions">
          <button
            disabled={busy ||
              !canValidate(rec) ||
              !!validationConflicts[rec.id]}
            onclick={() => validateRecord(rec)}
            >Validasi presensi {rec.user_name}</button
          >
        </div>
      </article>
    {:else}
      <p class="notice">Belum ada record presensi dalam scope ini.</p>
    {/each}
    {#if recordPage.hasMore}
      <button
        class="secondary"
        disabled={busy || loadingRecords}
        onclick={() => loadRecords(true)}>Muat presensi lainnya</button
      >
    {/if}
  </section>
</div>
