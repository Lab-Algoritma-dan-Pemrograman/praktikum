<script lang="ts">
  import { onMount } from 'svelte';
  import {
    SiakadClient,
    errorMessage,
    jakarta,
    rupiah,
    type AcademicReportSummary
  } from '@algohub/siakad-client';
  import { hasPermission, type Principal } from '@algohub/auth-client';
  const client = new SiakadClient();
  export let principal: Principal | null;
  $: canFinance = hasPermission(
    principal?.grants ?? [],
    'finance.manage',
    'siakad'
  );
  $: csv =
    report?.csv_export
      .split('\n')
      .map((row) =>
        !canFinance && row.startsWith('total_verified_revenue,')
          ? 'total_verified_revenue,0'
          : row
      )
      .join('\n') ?? '';
  let report: AcademicReportSummary | null = null;
  let loading = false;
  let issue = '';

  async function load() {
    loading = true;
    issue = '';
    try {
      report = await client.get<AcademicReportSummary>('/admin/siakad/laporan');
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    void load();
  });
</script>

<div class="stack">
  <div class="actions">
    <h2>Laporan akademik, inventaris, dan keuangan</h2>
    <button class="secondary" onclick={load} disabled={loading}
      >Perbarui laporan</button
    >
  </div>
  {#if loading}<p role="status">Memuat laporan…</p>{/if}
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if report}
    <div class="grid">
      <section class="panel">
        <h3>Akademik & Presensi</h3>
        <p>
          Kelas terdaftar: <strong>{report.classes_count}</strong> · Jadwal:
          <strong>{report.schedules_count}</strong>
        </p>
        <p>
          Sesi presensi terbuka: <strong
            >{report.open_attendance_sessions}</strong
          >
        </p>
        <p>
          Hadir: <strong>{report.attendance_by_status.hadir ?? 0}</strong> ·
          Izin: <strong>{report.attendance_by_status.izin ?? 0}</strong> ·
          Terlambat:
          <strong>{report.attendance_by_status.terlambat ?? 0}</strong> · Alpha:
          <strong>{report.attendance_by_status.alpha ?? 0}</strong>
        </p>
      </section>
      <section class="panel">
        <h3>Inventaris & Keuangan</h3>
        <p>
          Total item inventaris: <strong>{report.total_inventory_items}</strong>
          · Unit tersedia: <strong>{report.available_inventory_units}</strong>
        </p>
        <p>
          Penyewaan aktif: <strong>{report.active_rentals_count}</strong>
        </p>
        {#if canFinance}
          <p data-testid="verified-revenue">
            Pendapatan sewa terverifikasi sesuai scope:
            <strong>{rupiah(report.total_verified_revenue)}</strong>
          </p>
        {/if}
        <p class="muted">Dibuat: {jakarta(report.generated_at)}</p>
      </section>
    </div>
    <section class="panel stack" aria-label="Export CSV laporan SIAKAD">
      <div class="actions">
        <h3>Ringkasan CSV Export</h3>
        <a
          class="button secondary"
          href={`data:text/csv;charset=utf-8,${encodeURIComponent(csv)}`}
          download="laporan-siakad.csv">Unduh laporan CSV</a
        >
      </div>
      <!-- svelte-ignore a11y_no_noninteractive_tabindex (Scrollable CSV preview must be keyboard reachable.) -->
      <pre tabindex="0" aria-label="CSV laporan SIAKAD">{csv}</pre>
    </section>
  {/if}
</div>
