<script lang="ts">
  import { onMount } from 'svelte';
  import type { AppIdentityData } from '@algohub/auth-client';
  import {
    PracticumClient,
    command,
    errorMessage,
    jakarta,
    type Command
  } from '@algohub/practicum-client';

  export let data: AppIdentityData;

  interface LaporanStatus {
    laporan_id: string;
    judul: string;
    deskripsi: string;
    butuh_laporan: boolean;
    butuh_analisa: boolean;
    class_id?: string;
    shift: number;
    buka_at: string | null;
    deadline_at: string | null;
    late_until: string | null;
    fase: 'belum_dibuka' | 'dibuka' | 'telat' | 'ditutup';
    sisa_detik: number;
    bisa_kumpul: boolean;
    status: 'belum' | 'terkumpul';
    sumber: string;
    terlambat: boolean;
    submitted_at: string | null;
    has_laporan: boolean;
    has_analisa: boolean;
  }
  interface UploadURLResult {
    upload_url: string;
    path: string;
  }

  type Jenis = 'laporan' | 'analisa';
  type Slot = 'idle' | 'uploading' | 'done' | 'error';

  const MIME: Record<Jenis, string> = {
    laporan: 'application/pdf',
    analisa:
      'application/vnd.openxmlformats-officedocument.wordprocessingml.document'
  };
  const ACCEPT: Record<Jenis, string> = {
    laporan: '.pdf,application/pdf',
    analisa:
      '.docx,application/vnd.openxmlformats-officedocument.wordprocessingml.document'
  };
  const LABEL: Record<Jenis, string> = {
    laporan: 'Laporan (PDF)',
    analisa: 'Analisa (DOCX)'
  };
  const MAX_UPLOAD = 10 * 1024 * 1024;

  const client = new PracticumClient();
  let status: LaporanStatus | null = null;
  let loading = true;
  let issue = '';

  let slot: Record<Jenis, Slot> = { laporan: 'idle', analisa: 'idle' };
  let paths: Record<Jenis, string | null> = {
    laporan: null,
    analisa: null
  };
  let namaFile: Record<Jenis, string | null> = {
    laporan: null,
    analisa: null
  };
  let uploadErr = '';
  let submitting = false;
  let submitErr = '';
  let batalBusy = false;
  let pending: Command | null = null;

  $: terkumpul = status?.status === 'terkumpul';
  $: manual = status?.sumber === 'manual';
  $: terkunci =
    !status || status.fase === 'belum_dibuka' || status.fase === 'ditutup';
  // Window applies only when the student's class has a target window.
  $: berlaku = !!status && !!status.deadline_at;
  $: bolehUpload = berlaku && !!status?.bisa_kumpul && (!terkumpul || !manual);
  $: semuaSiap =
    !!status &&
    (!status.butuh_laporan || slot.laporan === 'done') &&
    (!status.butuh_analisa || slot.analisa === 'done');
  $: adaProses = slot.laporan === 'uploading' || slot.analisa === 'uploading';
  // Countdown points at the deadline, or late_until while in the late window.
  $: countdownTarget =
    status?.fase === 'telat'
      ? status.late_until
      : status?.fase === 'belum_dibuka'
        ? status.buka_at
        : status?.fase === 'dibuka'
          ? status.deadline_at
          : null;
  $: sisaDetik = status?.sisa_detik ?? 0;

  const faseLabel: Record<LaporanStatus['fase'], string> = {
    belum_dibuka: 'Belum dibuka',
    dibuka: 'Sedang dikumpul',
    telat: 'TELAT (masih bisa)',
    ditutup: 'Ditutup'
  };

  function formatCountdown(detik: number): string {
    const s = Math.max(0, Math.floor(detik));
    const hari = Math.floor(s / 86400);
    const jam = Math.floor((s % 86400) / 3600);
    const menit = Math.floor((s % 3600) / 60);
    const detikSisa = s % 60;
    return `${hari} hari ${String(jam).padStart(2, '0')}:${String(menit).padStart(2, '0')}:${String(detikSisa).padStart(2, '0')}`;
  }

  // Late penalty 1/7..7/7, purely informational.
  function formatPenalti(submitted: string, deadline: string): string {
    const s = new Date(submitted).getTime();
    const d = new Date(deadline).getTime();
    if (Number.isNaN(s) || Number.isNaN(d)) return '?/7';
    const telatHari = Math.max(
      1,
      Math.min(7, Math.ceil((s - d) / (24 * 60 * 60 * 1000)))
    );
    return `${telatHari}/7`;
  }

  async function load() {
    loading = true;
    issue = '';
    try {
      status = await client.get<LaporanStatus>('/laporan/saya');
      // Realign slot state with files already stored server-side.
      if (status.has_laporan) {
        slot.laporan = 'done';
        paths.laporan = '__tersimpan__';
      }
      if (status.has_analisa) {
        slot.analisa = 'done';
        paths.analisa = '__tersimpan__';
      }
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loading = false;
    }
  }

  // Placeholder means the file is already on the server; only send real new paths.
  function pathFinal(j: Jenis): string | null {
    const p = paths[j];
    return p && p !== '__tersimpan__' ? p : null;
  }

  async function pilihFile(j: Jenis, ev: Event) {
    uploadErr = '';
    const input = ev.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file || !status) return;

    const extHarap = j === 'laporan' ? 'pdf' : 'docx';
    const extFile = (file.name.split('.').pop() || '').toLowerCase();
    if (extFile !== extHarap) {
      uploadErr = `File ${LABEL[j]} harus berekstensi .${extHarap}.`;
      input.value = '';
      return;
    }
    if (file.size > MAX_UPLOAD) {
      uploadErr = `File ${LABEL[j]} terlalu besar (${(file.size / 1024 / 1024).toFixed(1)} MB). Maksimal 10 MB.`;
      input.value = '';
      return;
    }

    namaFile[j] = file.name;
    slot[j] = 'uploading';
    try {
      const res = await client.execute<UploadURLResult>(
        command(`/laporan/upload-url`, 'POST', { jenis: j, ext: extHarap })
      );
      // Browser uploads straight to the signed URL; not through the API client.
      const put = await fetch(res.upload_url, {
        method: 'PUT',
        body: file,
        headers: { 'Content-Type': MIME[j] }
      });
      if (!put.ok)
        throw new Error(`Unggah gagal ke penyimpanan (HTTP ${put.status}).`);
      paths[j] = res.path;
      slot[j] = 'done';
    } catch (error) {
      slot[j] = 'error';
      uploadErr = errorMessage(error);
    } finally {
      input.value = '';
    }
  }

  async function kumpulkan() {
    if (!status) return;
    submitErr = '';
    const laporan_path = status.butuh_laporan ? pathFinal('laporan') : null;
    const analisa_path = status.butuh_analisa ? pathFinal('analisa') : null;
    if (status.butuh_laporan && !laporan_path) {
      submitErr = 'File Laporan belum terunggah.';
      return;
    }
    if (status.butuh_analisa && !analisa_path) {
      submitErr = 'File Analisa belum terunggah.';
      return;
    }
    submitting = true;
    try {
      await client.execute(
        command('/laporan/kumpul', 'POST', { laporan_path, analisa_path })
      );
      slot = { laporan: 'idle', analisa: 'idle' };
      paths = { laporan: null, analisa: null };
      await load();
    } catch (error) {
      submitErr = errorMessage(error);
    } finally {
      submitting = false;
    }
  }

  async function batal() {
    batalBusy = true;
    submitErr = '';
    try {
      await client.execute(command('/laporan/batalkan', 'POST', {}));
      slot = { laporan: 'idle', analisa: 'idle' };
      paths = { laporan: null, analisa: null };
      namaFile = { laporan: null, analisa: null };
      await load();
    } catch (error) {
      submitErr = errorMessage(error);
    } finally {
      batalBusy = false;
    }
  }

  onMount(() => {
    if (data.principal) void load();
    else loading = false;
  });
</script>

<svelte:head><title>Laporan Besar — Praktikum</title></svelte:head>
<header class="page-heading">
  <p class="eyebrow">TUGAS AKHIR PRAKTIKUM</p>
  <h1>Laporan Besar</h1>
  <p class="muted">
    Unggah laporan akhir dan analisa praktikum sesuai window pengumpulan kelas
    Anda.
  </p>
</header>
{#if !data.principal}
  <section class="panel empty-state">
    <h2>Masuk untuk melihat status laporan</h2>
    <p>Laporan Besar hanya dapat diakses oleh mahasiswa yang terdaftar.</p>
    <a class="button" href="/login?return_to=/laporan">Masuk ke Praktikum</a>
  </section>
{:else if loading}
  <p role="status" class="muted">Memuat status laporan…</p>
{:else if issue}
  <div class="notice error" role="alert">
    {issue}<button class="secondary" onclick={() => load()} disabled={loading}
      >Coba lagi</button
    >
  </div>
{:else if status}
  <section class="panel stack" aria-busy={loading}>
    <div class="section-heading">
      <h2>{status.judul}</h2>
      <div class="actions">
        <span class="badge">{faseLabel[status.fase]}</span>
        {#if terkumpul}
          <span class="badge">
            {status.terlambat
              ? 'Terkumpul (telat)'
              : manual
                ? 'Terkumpul (manual)'
                : 'Terkumpul'}</span
          >
        {/if}
        <button class="secondary" onclick={() => load()} disabled={loading}
          >Perbarui status</button
        >
      </div>
    </div>
    {#if status.deskripsi}<p>{status.deskripsi}</p>{/if}

    {#if terkumpul && status.terlambat && status.submitted_at && status.deadline_at}
      <p class="notice">
        Kamu mengumpulkan <strong>telat</strong> — keterlambatan
        {formatPenalti(status.submitted_at, status.deadline_at)}. Hari penuh
        dihitung dari batas pengumpulan.
      </p>
    {/if}

    <div class="grid">
      <div>
        <p class="eyebrow">Dibuka</p>
        <p>{jakarta(status.buka_at)}</p>
      </div>
      <div>
        <p class="eyebrow">Batas pengumpulan</p>
        <p>{jakarta(status.deadline_at)}</p>
      </div>
      <div>
        <p class="eyebrow">Batas toleransi keterlambatan</p>
        <p>{status.late_until ? jakarta(status.late_until) : 'Tidak ada'}</p>
      </div>
    </div>

    {#if countdownTarget && status.fase !== 'ditutup'}
      <p class="muted">
        {status.fase === 'belum_dibuka'
          ? 'Dibuka dalam'
          : status.fase === 'telat'
            ? 'Batas akhir (masih bisa)'
            : 'Sisa waktu'}:
        <strong>{formatCountdown(sisaDetik)}</strong>
      </p>
    {/if}
  </section>

  {#if !berlaku}
    <div class="panel stack">
      <h3>Tidak berlaku untuk kelas kamu</h3>
      <p class="muted">
        Laporan Besar ini tidak diberlakukan untuk kelas kamu. Kamu tidak perlu
        mengunggah dokumen apa pun di sini. Hubungi asisten praktikum bila kamu
        merasa hal ini keliru.
      </p>
    </div>
  {:else if manual}
    <div class="panel stack">
      <h3>Pengumpulan via MS Teams</h3>
      <p class="muted">
        Laporan Besar untuk kelasmu dikumpulkan melalui MS Teams. Status
        pengumpulanmu ditetapkan oleh asisten, bukan lewat halaman ini.
      </p>
    </div>
  {:else if !bolehUpload && !terkumpul}
    <div class="panel stack">
      <h3>
        {status.fase === 'belum_dibuka'
          ? 'Belum dibuka'
          : 'Pengumpulan ditutup'}
      </h3>
      <p class="muted">
        {status.fase === 'belum_dibuka'
          ? 'Pengumpulan Laporan Besar belum dibuka. Halaman ini akan mengizinkan unggah setelah periode dimulai.'
          : 'Periode pengumpulan Laporan Besar telah ditutup. Kamu tidak dapat mengunggah lagi.'}
      </p>
    </div>
  {:else if bolehUpload}
    <section class="stack">
      <div class="section-heading">
        <h2>Unggah dokumen</h2>
      </div>
      <p class="muted">
        Format: laporan <strong>.pdf</strong>, analisa <strong>.docx</strong>.
        Maksimal <strong>10 MB</strong> per file.
      </p>
      {#if uploadErr}<p class="notice error" role="alert">{uploadErr}</p>{/if}
      <div class="grid">
        {#each ['laporan', 'analisa'] as const as j}
          {#if j === 'laporan' ? status.butuh_laporan : status.butuh_analisa}
            <div class="panel stack">
              <div class="section-heading">
                <h3>{LABEL[j]}</h3>
                {#if slot[j] === 'done'}<span class="badge">Terunggah</span
                  >{:else if slot[j] === 'error'}<span class="badge">Gagal</span
                  >{/if}
              </div>
              {#if slot[j] === 'uploading'}
                <p role="status" class="muted">Mengunggah…</p>
              {:else if slot[j] === 'done'}
                <p class="muted">
                  {namaFile[j] ?? 'Sudah tersimpan di server'}
                </p>
              {:else}
                <p class="muted">
                  {namaFile[j] ?? `Format .${j === 'laporan' ? 'pdf' : 'docx'}`}
                </p>
              {/if}
              <label class="button secondary">
                {slot[j] === 'done' ? 'Ganti berkas' : 'Pilih berkas'}
                <input
                  type="file"
                  accept={ACCEPT[j]}
                  onchange={(e) => pilihFile(j, e)}
                  disabled={slot[j] === 'uploading'}
                  style="display:none"
                />
              </label>
            </div>
          {/if}
        {/each}
      </div>

      <div class="panel stack">
        {#if submitErr}<p class="notice error" role="alert">{submitErr}</p>{/if}
        <div class="section-heading">
          <p>
            {#if terkumpul}
              <strong>Laporan sudah terkumpul</strong><br />
              <span class="muted"
                >Dikumpulkan: {jakarta(status.submitted_at)}</span
              >
            {:else if semuaSiap}
              Semua file wajib sudah terunggah. Siap dikumpulkan.
            {:else}
              <span class="muted">Unggah semua file wajib terlebih dahulu.</span
              >
            {/if}
          </p>
          {#if terkumpul && !manual}
            <button class="button" onclick={batal} disabled={batalBusy}>
              {batalBusy ? 'Membatalkan…' : 'Batalkan pengumpulan'}</button
            >
          {:else}
            <button
              class="button"
              disabled={!semuaSiap || adaProses || terkumpul || submitting}
              onclick={kumpulkan}
            >
              {submitting
                ? 'Mengumpulkan…'
                : terkumpul
                  ? 'Sudah terkumpul'
                  : 'Kumpulkan'}</button
            >
          {/if}
        </div>
      </div>
    </section>
  {/if}
{:else}
  <section class="panel empty-state">
    <h2>Belum ada Laporan Besar</h2>
    <p class="muted">
      Belum ada Laporan Besar yang berlaku untuk kelas dan shift Anda saat ini.
      Halaman ini akan menampilkan tugas begitu pengajar menerbitkannya.
    </p>
  </section>
{/if}
