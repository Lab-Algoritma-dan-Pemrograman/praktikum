<script lang="ts">
  import type { Principal } from '@algohub/auth-client';
  import {
    SiakadClient,
    permitted,
    reusableCommand,
    PendingCommands,
    mergeResources,
    errorMessage,
    type AcademicClass
  } from '@algohub/siakad-client';
  export let classes: AcademicClass[];
  export let principal: Principal | null;
  export let refresh: (saved: AcademicClass) => Promise<void>;
  const client = new SiakadClient();

  let selectedId = '';
  let version = 0;
  let code = '';
  let courseCode = '';
  let courseName = '';
  let periodName = 'Ganjil 2026/2027';
  let shift = 1;
  let studentIdsText = '';
  let assistantIdsText = '';
  let institution = '';
  let active = true;
  let busy = false;
  let issue = '';
  let notice = '';
  const pending = new PendingCommands();

  $: selectedClass = classes.find((cls) => cls.id === selectedId);
  $: scope = selectedClass?.scope ?? {
    domain: 'siakad',
    ...(institution.trim() ? { institution: institution.trim() } : {})
  };
  $: canManage =
    permitted(principal, 'class.manage', {
      ...scope,
      ...(selectedId ? { class_id: selectedId } : {}),
      shift
    }) &&
    (!selectedClass ||
      permitted(principal, 'class.manage', {
        ...selectedClass.scope,
        class_id: selectedClass.id,
        shift: selectedClass.shift
      }));

  function parseIDs(text: string): number[] | null {
    if (!text.trim()) return [];
    const ids = text
      .trim()
      .split(/[\s,]+/)
      .map(Number);
    return ids.every((id) => Number.isSafeInteger(id) && id > 0)
      ? [...new Set(ids)]
      : null;
  }

  function choose(id: string) {
    const cls = classes.find((c) => c.id === id);
    selectedId = id;
    version = cls?.version ?? 0;
    code = cls?.code ?? '';
    courseCode = cls?.course_code ?? '';
    courseName = cls?.course_name ?? '';
    periodName = cls?.period_name ?? 'Ganjil 2026/2027';
    shift = cls?.shift ?? 1;
    studentIdsText = (cls?.student_ids ?? []).join(', ');
    assistantIdsText = (cls?.assistant_ids ?? []).join(', ');
    active = cls?.active ?? true;
  }

  async function save() {
    if (busy || !canManage) return;
    const studentIDs = parseIDs(studentIdsText);
    const assistantIDs = parseIDs(assistantIdsText);
    if (!studentIDs || !assistantIDs) {
      issue =
        'ID anggota harus berupa bilangan bulat positif, dipisahkan koma atau spasi.';
      return;
    }
    busy = true;
    issue = '';
    notice = '';
    const path = `/admin/siakad/kelas${selectedId ? `/${encodeURIComponent(selectedId)}` : ''}`;
    const method = selectedId ? 'PATCH' : 'POST';
    const mutation = pending.put(
      reusableCommand(pending.get(path, method), path, method, {
        ...(selectedId ? { version, active } : { scope }),
        code,
        course_code: courseCode,
        course_name: courseName,
        period_name: periodName,
        shift,
        student_ids: studentIDs,
        assistant_ids: assistantIDs
      })
    );
    try {
      const saved = await client.execute<AcademicClass>(mutation);
      pending.complete(mutation);
      classes = mergeResources(classes, [saved], (row) => row.id);
      choose(saved.id);
      await refresh(saved);
      notice = 'Data kelas berhasil disimpan.';
    } catch (error) {
      pending.failed(mutation, error);
      issue = errorMessage(error);
      if (!pending.get(path, method) && selectedId) {
        issue +=
          ' Muat ulang halaman, pilih kembali kelas ini, lalu periksa data terbaru sebelum memasukkan ulang perubahan Anda.';
      }
    } finally {
      busy = false;
    }
  }
</script>

<section class="panel stack">
  <h2>Kelola kelas dan keanggotaan</h2>
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if notice}<p class="notice" role="status">{notice}</p>{/if}
  <label
    >Pilih kelas untuk diedit<select
      value={selectedId}
      disabled={busy}
      onchange={(e) => choose(e.currentTarget.value)}
    >
      <option value="">Kelas baru</option>
      {#each classes as cls}<option value={cls.id}
          >{cls.code} — {cls.course_name} (Shift {cls.shift})</option
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
      {#if !selectedId}
        <label
          >Institusi kelas (opsional)<input
            bind:value={institution}
            maxlength="100"
          /></label
        >
      {/if}
      <label
        >Kode kelas<input bind:value={code} required maxlength="200" /></label
      >
      <label
        >Kode mata kuliah<input
          bind:value={courseCode}
          required
          maxlength="200"
        /></label
      >
      <label
        >Nama mata kuliah<input
          bind:value={courseName}
          required
          maxlength="200"
        /></label
      >
      <label
        >Periode akademik<input
          bind:value={periodName}
          required
          maxlength="200"
        /></label
      >
      <label
        >Shift kelas<select bind:value={shift}>
          <option value={1}>Shift 1</option>
          <option value={2}>Shift 2</option>
        </select></label
      >
    </div>
    <label
      >ID mahasiswa kelas<input
        bind:value={studentIdsText}
        placeholder="1, 2"
      /></label
    >
    <label
      >ID asisten kelas<input
        bind:value={assistantIdsText}
        placeholder="2, 3"
      /></label
    >
    {#if selectedId}
      <label class="check"
        ><input type="checkbox" bind:checked={active} />Kelas aktif</label
      >
    {/if}
    <button disabled={!canManage || busy}
      >{selectedId ? 'Simpan kelas' : 'Buat kelas'}</button
    >
    {#if !canManage}<p class="notice">
        Scope kelas ini tidak tercakup grant Anda.
      </p>{/if}
  </form>
</section>
