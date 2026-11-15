<script lang="ts">
  import { onMount } from 'svelte';
  import type { Principal } from '@algohub/auth-client';
  import {
    PracticumClient,
    permitted,
    command,
    reusableCommand,
    errorMessage,
    jakarta,
    type Command
  } from '@algohub/practicum-client';

  export let principal: Principal | null;

  interface LaporanTarget {
    class_id: string;
    shift: number;
    buka_at: string | null;
    deadline_at: string;
    late_until: string | null;
  }
  interface LaporanBesar {
    id: string;
    judul: string;
    deskripsi: string;
    butuh_laporan: boolean;
    butuh_analisa: boolean;
    active: boolean;
    targets: LaporanTarget[];
    version: number;
  }
  interface PengumpulanRow {
    user_id: number;
    user_name: string;
    class_id?: string;
    shift: number;
    status: 'belum' | 'terkumpul';
    sumber: string;
    laporan_path?: string;
    analisa_path?: string;
    submitted_at: string | null;
    is_late: boolean;
    manual_catatan?: string;
    override_alasan?: string;
    version: number;
  }
  interface AcademicClass {
    id: string;
    code: string;
    course_name: string;
    shift: number;
    active: boolean;
  }

  type Jenis = 'laporan' | 'analisa';

  const client = new PracticumClient();
  let laporanList: LaporanBesar[] = [];
  let selectedID = '';
  let kelas: AcademicClass[] = [];
  let rows: PengumpulanRow[] = [];
  let offset = 0;
  let loading = false;
  let savingForm = false;
  let savingTarget = false;
  let issue = '';
  let notice = '';

  let form = {
    judul: 'Laporan Besar',
    deskripsi: '',
    active: true,
    butuh_laporan: true,
    butuh_analisa: false
  };
  let targets: LaporanTarget[] = [];
  let addKelasId = '';
  let filterKelas = 'all';
  let selected: Record<string, boolean> = {};
  let pending: Command | null = null;

  $: current = laporanList.find((l) => l.id === selectedID) ?? null;
  $: canManage = !!principal?.grants.some(
    (g) => g.permission === 'laporan.manage'
  );
  $: filtered =
    filterKelas === 'all'
      ? rows
      : rows.filter((r) => r.class_id === filterKelas);

  function namaKelas(classId: string): string {
    const k = kelas.find((c) => c.id === classId);
    return k ? `${k.course_name} — ${k.code}` : classId || '—';
  }

  function toLocalInput(iso: string | null): string {
    if (!iso) return '';
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return '';
    const pad = (n: number) => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
  }
  function fromLocalInput(v: string): string | null {
    if (!v) return null;
    const d = new Date(v);
    return Number.isNaN(d.getTime()) ? null : d.toISOString();
  }

  // Late penalty 1/7..7/7, purely informational.
  function formatPenalti(
    submitted: string | null,
    deadline: string | null
  ): string {
    if (!submitted || !deadline) return '?/7';
    const s = new Date(submitted).getTime();
    const d = new Date(deadline).getTime();
    if (Number.isNaN(s) || Number.isNaN(d)) return '?/7';
    const telatHari = Math.max(
      1,
      Math.min(7, Math.ceil((s - d) / (24 * 60 * 60 * 1000)))
    );
    return `${telatHari}/7`;
  }

  function targetDeadline(classId: string): string | null {
    return (
      current?.targets.find((t) => t.class_id === classId)?.deadline_at ?? null
    );
  }

  async function load() {
    loading = true;
    issue = '';
    try {
      laporanList = await client.get<LaporanBesar[]>('/admin/laporan');
      if (laporanList.length && !laporanList.some((l) => l.id === selectedID)) {
        pilihLaporan(laporanList[0].id);
      } else if (current) {
        form = {
          judul: current.judul,
          deskripsi: current.deskripsi,
          active: current.active,
          butuh_laporan: current.butuh_laporan,
          butuh_analisa: current.butuh_analisa
        };
        targets = current.targets.map((t) => ({ ...t }));
      }
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loading = false;
    }
  }

  async function loadKelas() {
    try {
      kelas = await client.get<AcademicClass[]>(
        '/admin/siakad/kelas?limit=100'
      );
    } catch {
      kelas = []; // ponytail: single page of 100 classes; paginate when more exist
    }
  }

  async function loadPengumpulan() {
    if (!current) {
      rows = [];
      return;
    }
    try {
      rows = await client.get<PengumpulanRow[]>(
        `/admin/laporan/${encodeURIComponent(current.id)}/pengumpulan?limit=100&offset=${offset}`
      );
    } catch (error) {
      issue = errorMessage(error);
    }
  }

  function pilihLaporan(id: string) {
    selectedID = id;
    const l = laporanList.find((x) => x.id === id);
    if (l) {
      form = {
        judul: l.judul,
        deskripsi: l.deskripsi,
        active: l.active,
        butuh_laporan: l.butuh_laporan,
        butuh_analisa: l.butuh_analisa
      };
      targets = l.targets.map((t) => ({ ...t }));
    }
    void loadPengumpulan();
  }

  async function savePengaturan(event: SubmitEvent) {
    event.preventDefault();
    if (!canManage) return;
    savingForm = true;
    issue = '';
    notice = '';
    try {
      if (current) {
        await client.execute(
          command(`/admin/laporan/${encodeURIComponent(current.id)}`, 'PUT', {
            version: current.version,
            ...form,
            targets: current.targets
          })
        );
      } else {
        await client.execute(
          command('/admin/laporan', 'POST', { ...form, targets: [] })
        );
      }
      notice = 'Pengaturan laporan besar tersimpan.';
      await load();
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      savingForm = false;
    }
  }

  function kelasTersedia(): AcademicClass[] {
    const dipakai = new Set(targets.map((t) => t.class_id));
    return kelas.filter((k) => !dipakai.has(k.id));
  }

  function tambahTarget() {
    if (!addKelasId) return;
    targets = [
      ...targets,
      {
        class_id: addKelasId,
        shift: 0,
        buka_at: null,
        deadline_at: new Date(Date.now() + 7 * 86400000).toISOString(),
        late_until: null
      }
    ];
    addKelasId = '';
  }

  async function saveTargets() {
    if (!current || !canManage) return;
    savingTarget = true;
    issue = '';
    notice = '';
    try {
      await client.execute(
        command(`/admin/laporan/${encodeURIComponent(current.id)}`, 'PUT', {
          version: current.version,
          judul: current.judul,
          deskripsi: current.deskripsi,
          butuh_laporan: current.butuh_laporan,
          butuh_analisa: current.butuh_analisa,
          active: current.active,
          targets
        })
      );
      notice = 'Target kelas tersimpan.';
      await load();
      await loadPengumpulan();
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      savingTarget = false;
    }
  }

  async function markManual(
    row: PengumpulanRow,
    status: 'terkumpul' | 'belum'
  ) {
    if (!current || !canManage) return;
    issue = '';
    notice = '';
    let catatan: string | undefined;
    if (status === 'terkumpul') {
      const input = window.prompt(
        `Catatan manual untuk ${row.user_name} (opsional):`,
        row.manual_catatan ?? ''
      );
      if (input === null) return;
      catatan = input;
    }
    try {
      await client.execute(
        reusableCommand(
          pending,
          `/admin/laporan/${encodeURIComponent(current.id)}/mark/${row.user_id}`,
          'POST',
          { version: row.version, status, catatan: catatan ?? '' }
        )
      );
      notice =
        status === 'terkumpul'
          ? 'Ditandai terkumpul.'
          : 'Ditandai belum terkumpul.';
      await loadPengumpulan();
    } catch (error) {
      issue = errorMessage(error);
    }
  }

  async function bukaBlokir(row: PengumpulanRow) {
    if (!current || !canManage) return;
    issue = '';
    notice = '';
    const alasan = window.prompt(
      `Alasan membuka blokir untuk ${row.user_name} (wajib diisi):`,
      ''
    );
    if (alasan === null) return;
    if (!alasan.trim() || alasan.trim().length < 3) {
      issue = 'Alasan buka blokir wajib diisi (minimal 3 karakter).';
      return;
    }
    try {
      await client.execute(
        command(
          `/admin/laporan/${encodeURIComponent(current.id)}/override/${row.user_id}`,
          'POST',
          {
            version: row.version,
            alasan: alasan.trim()
          }
        )
      );
      notice = 'Blokir dibuka.';
      await loadPengumpulan();
    } catch (error) {
      issue = errorMessage(error);
    }
  }

  async function cabutBlokir(row: PengumpulanRow) {
    if (!current || !canManage) return;
    if (!window.confirm(`Cabut izin buka blokir untuk ${row.user_name}?`))
      return;
    issue = '';
    notice = '';
    const alasan = window.prompt(`Alasan pencabutan (wajib diisi):`, '');
    if (alasan === null || !alasan.trim() || alasan.trim().length < 3) {
      issue = 'Alasan pencabutan wajib diisi (minimal 3 karakter).';
      return;
    }
    try {
      await client.execute(
        command(
          `/admin/laporan/${encodeURIComponent(current.id)}/override/${row.user_id}`,
          'DELETE',
          {
            version: row.version,
            alasan: alasan.trim()
          }
        )
      );
      notice = 'Buka blokir dicabut.';
      await loadPengumpulan();
    } catch (error) {
      issue = errorMessage(error);
    }
  }

  // Both preview and download go through the same signed-berkas endpoint.
  async function signedBerkas(
    row: PengumpulanRow,
    jenis: Jenis,
    download: boolean
  ) {
    if (!current) return null;
    const res = await client.get<{ url: string; filename: string }>(
      `/laporan/${encodeURIComponent(current.id)}/berkas/${row.user_id}?jenis=${jenis}${download ? '&download=1' : ''}`
    );
    return res?.url ? res : null;
  }

  async function unduh(row: PengumpulanRow, jenis: Jenis) {
    issue = '';
    try {
      const res = await signedBerkas(row, jenis, true);
      if (!res) {
        issue = 'Berkas belum tersedia.';
        return;
      }
      const a = document.createElement('a');
      a.href = res.url;
      a.download = res.filename || '';
      document.body.appendChild(a);
      a.click();
      a.remove();
    } catch (error) {
      issue = errorMessage(error);
    }
  }

  let previewUrl = '';
  let previewJenis: Jenis = 'laporan';
  let previewRow: PengumpulanRow | null = null;
  let previewBusy = false;

  async function pratinjau(row: PengumpulanRow, jenis: Jenis) {
    issue = '';
    previewRow = row;
    previewJenis = jenis;
    previewUrl = '';
    previewBusy = true;
    try {
      const res = await signedBerkas(row, jenis, false);
      previewUrl = res?.url ?? '';
      if (!previewUrl) issue = 'Tidak bisa memuat pratinjau.';
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      previewBusy = false;
    }
  }

  function statusLabel(row: PengumpulanRow): string {
    if (row.override_alasan) return 'Buka blokir';
    if (row.status === 'terkumpul')
      return row.is_late ? 'Terkumpul (telat)' : 'Terkumpul';
    return 'Belum';
  }

  onMount(() => {
    void Promise.all([load(), loadKelas()]).then(() => loadPengumpulan());
  });
</script>

<div class="stack">
  <h2>Tugas akhir praktikum (Laporan Besar)</h2>
  <p class="muted">
    Satu tugas laporan berdiri sendiri. Atur judul, window per kelas, lalu
    pantau dan verifikasi pengumpulan mahasiswa.
  </p>
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if notice}<p class="notice success" role="status">{notice}</p>{/if}
  {#if loading}<p role="status">Memuat laporan besar…</p>{/if}

  {#if laporanList.length > 0}
    <label
      >Laporan besar<select
        value={selectedID}
        onchange={(event) => pilihLaporan(event.currentTarget.value)}
        disabled={loading}
        >{#each laporanList as l}<option value={l.id}>{l.judul}</option
          >{/each}</select
      ></label
    >
  {/if}

  <form class="stack" onsubmit={savePengaturan}>
    <h3>{current ? 'Pengaturan' : 'Buat laporan besar'}</h3>
    <label
      >Judul<input bind:value={form.judul} required maxlength="200" /></label
    >
    <label
      >Deskripsi<textarea bind:value={form.deskripsi} rows="3" maxlength="4000"
      ></textarea></label
    >
    <div class="actions">
      <label class="muted"
        ><input type="checkbox" bind:checked={form.active} /> Aktifkan laporan besar</label
      >
      <label class="muted"
        ><input type="checkbox" bind:checked={form.butuh_laporan} /> Wajib unggah
        laporan (PDF)</label
      >
      <label class="muted"
        ><input type="checkbox" bind:checked={form.butuh_analisa} /> Wajib unggah
        analisa (DOCX)</label
      >
    </div>
    <button disabled={!canManage || savingForm}
      >{savingForm ? 'Menyimpan…' : 'Simpan pengaturan'}</button
    >
  </form>

  {#if current}
    <h3>Target kelas</h3>
    <p class="muted">
      Atur jadwal buka, tenggat, dan batas telat per kelas. Shift 0 berlaku
      untuk semua shift.
    </p>
    <div class="actions">
      <label
        >Tambah kelas<select bind:value={addKelasId}
          ><option value="" disabled>— Pilih kelas —</option
          >{#each kelasTersedia() as k}<option value={k.id}
              >{namaKelas(k.id)}</option
            >{/each}</select
        ></label
      >
      <button class="secondary" onclick={tambahTarget} disabled={!addKelasId}
        >Tambah</button
      >
      <button onclick={saveTargets} disabled={!canManage || savingTarget}
        >{savingTarget ? 'Menyimpan…' : 'Simpan target'}</button
      >
    </div>
    <p class="muted" id="laporan-target-scroll">
      Pada layar kecil, geser tabel secara horizontal untuk melihat semua kolom.
    </p>
    <!-- svelte-ignore a11y_no_noninteractive_tabindex (Horizontal table scrolling must be keyboard reachable.) -->
    <div
      class="table-wrap panel"
      role="region"
      aria-label="Tabel target kelas"
      aria-describedby="laporan-target-scroll"
      tabindex="0"
    >
      <table>
        <caption>Window pengumpulan per kelas</caption>
        <thead>
          <tr>
            <th>Kelas</th>
            <th>Shift</th>
            <th>Buka</th>
            <th>Deadline</th>
            <th>Batas telat</th>
            <th>Aksi</th>
          </tr>
        </thead>
        <tbody>
          {#each targets as t, i (t.class_id + '-' + t.shift)}
            <tr>
              <td>{namaKelas(t.class_id)}</td>
              <td
                ><select
                  bind:value={t.shift}
                  aria-label="Shift untuk window ini"
                >
                  <option value={0}>Semua</option>
                  <option value={1}>Shift 1</option>
                  <option value={2}>Shift 2</option>
                </select></td
              >
              <td
                ><input
                  type="datetime-local"
                  value={toLocalInput(t.buka_at)}
                  oninput={(e) =>
                    (t.buka_at = fromLocalInput(
                      (e.currentTarget as HTMLInputElement).value
                    ))}
                /></td
              >
              <td
                ><input
                  type="datetime-local"
                  required
                  value={toLocalInput(t.deadline_at)}
                  oninput={(e) =>
                    (t.deadline_at =
                      fromLocalInput(
                        (e.currentTarget as HTMLInputElement).value
                      ) ?? t.deadline_at)}
                /></td
              >
              <td
                ><input
                  type="datetime-local"
                  value={toLocalInput(t.late_until)}
                  oninput={(e) =>
                    (t.late_until = fromLocalInput(
                      (e.currentTarget as HTMLInputElement).value
                    ))}
                /></td
              >
              <td
                ><button
                  class="secondary"
                  onclick={() => (targets = targets.filter((_, j) => j !== i))}
                  >Hapus</button
                ></td
              >
            </tr>
          {:else}
            <tr><td colspan="6">Belum ada target kelas.</td></tr>
          {/each}
        </tbody>
      </table>
    </div>

    <div class="section-heading">
      <h3>Pengumpulan ({filtered.length})</h3>
      <label
        >Filter kelas<select bind:value={filterKelas}
          ><option value="all">Semua kelas</option
          >{#each current.targets as t}<option value={t.class_id}
              >{namaKelas(t.class_id)}</option
            >{/each}</select
        ></label
      >
    </div>
    <p class="muted" id="laporan-rekap-scroll">
      Pada layar kecil, geser tabel secara horizontal untuk melihat semua kolom.
    </p>
    <!-- svelte-ignore a11y_no_noninteractive_tabindex (Horizontal table scrolling must be keyboard reachable.) -->
    <div
      class="table-wrap panel"
      role="region"
      aria-label="Tabel rekap pengumpulan"
      aria-describedby="laporan-rekap-scroll"
      tabindex="0"
    >
      <table>
        <caption>Rekap pengumpulan mahasiswa</caption>
        <thead>
          <tr>
            <th>Peserta</th>
            <th>Kelas</th>
            <th>Status</th>
            <th>Sumber</th>
            <th>Waktu</th>
            <th>Berkas</th>
            <th>Aksi</th>
          </tr>
        </thead>
        <tbody>
          {#each filtered as row (row.user_id)}
            <tr>
              <td
                >{row.user_name}<br /><span class="muted">ID {row.user_id}</span
                ></td
              >
              <td>{row.class_id ? namaKelas(row.class_id) : '—'}</td>
              <td>
                <span class="badge">{statusLabel(row)}</span>
                {#if row.status === 'terkumpul' && row.is_late && row.submitted_at}
                  <p class="muted">
                    Telat {formatPenalti(
                      row.submitted_at,
                      targetDeadline(row.class_id ?? '')
                    )}
                  </p>
                {/if}
                {#if row.manual_catatan}<p class="muted">
                    {row.manual_catatan}
                  </p>{/if}
                {#if row.override_alasan}<p class="muted">
                    Alasan: {row.override_alasan}
                  </p>{/if}
              </td>
              <td>{row.sumber || '—'}</td>
              <td>{jakarta(row.submitted_at)}</td>
              <td>
                <div class="actions">
                  {#if row.laporan_path}<button
                      class="secondary"
                      onclick={() => pratinjau(row, 'laporan')}
                      >Lihat laporan</button
                    ><button
                      class="secondary"
                      onclick={() => unduh(row, 'laporan')}
                      >Unduh laporan</button
                    >{/if}
                  {#if row.analisa_path}<button
                      class="secondary"
                      onclick={() => pratinjau(row, 'analisa')}
                      >Lihat analisa</button
                    ><button
                      class="secondary"
                      onclick={() => unduh(row, 'analisa')}
                      >Unduh analisa</button
                    >{/if}
                </div>
              </td>
              <td>
                <div class="actions">
                  {#if row.status === 'belum'}
                    <button
                      class="secondary"
                      disabled={!canManage}
                      onclick={() => markManual(row, 'terkumpul')}
                      >Centang manual</button
                    >
                  {:else}
                    <button
                      class="secondary"
                      disabled={!canManage}
                      onclick={() => markManual(row, 'belum')}
                      >Batal centang</button
                    >
                  {/if}
                  {#if row.override_alasan}
                    <button
                      class="secondary"
                      disabled={!canManage}
                      onclick={() => cabutBlokir(row)}>Cabut blokir</button
                    >
                  {:else}
                    <button
                      class="secondary"
                      disabled={!canManage}
                      onclick={() => bukaBlokir(row)}>Buka blokir</button
                    >
                  {/if}
                </div>
              </td>
            </tr>
          {:else}
            <tr><td colspan="7">Belum ada data pengumpulan.</td></tr>
          {/each}
        </tbody>
      </table>
    </div>
    <div class="actions">
      <button
        class="secondary"
        disabled={offset === 0 || loading}
        onclick={() => (offset = Math.max(0, offset - 100))}>Sebelumnya</button
      ><button
        class="secondary"
        disabled={rows.length < 100 || loading}
        onclick={() => (offset = offset + 100)}>Berikutnya</button
      >
    </div>
  {/if}
</div>

{#if previewRow}
  <dialog
    class="submit-dialog"
    open
    onclose={() => (previewRow = null)}
    aria-label={`Pratinjau ${previewJenis === 'laporan' ? 'laporan' : 'analisa'} — ${previewRow.user_name}`}
  >
    <div class="section-heading">
      <h3>
        Pratinjau {previewJenis === 'laporan' ? 'laporan' : 'analisa'} —
        {previewRow.user_name}
      </h3>
      <button
        class="secondary"
        aria-label="Tutup"
        onclick={() => (previewRow = null)}>Tutup</button
      >
    </div>
    {#if previewBusy}
      <p role="status">Memuat pratinjau…</p>
    {:else if previewJenis === 'analisa' || !previewUrl}
      <p class="muted">
        Pratinjau DOCX tidak tersedia inline — silakan unduh berkasnya.
      </p>
    {:else}
      <iframe
        src={previewUrl}
        title="Pratinjau berkas"
        style="width:100%;height:min(70vh,40rem);border:1px solid var(--ah-console-line)"
      ></iframe>
    {/if}
    {#if previewUrl}
      <div class="actions">
        <button
          class="secondary"
          onclick={() => unduh(previewRow!, previewJenis)}>Unduh berkas</button
        >
      </div>
    {/if}
  </dialog>
{/if}
