<script lang="ts">
  import { onMount } from 'svelte';
  import type { AppIdentityData } from '@algohub/auth-client';
  import {
    SiakadClient,
    emptyPage,
    loadResourcePage,
    errorMessage,
    jakarta,
    type AcademicClass,
    type AcademicNotification,
    type AcademicScheduleView
  } from '@algohub/siakad-client';
  export let data: AppIdentityData;
  const client = new SiakadClient();
  let classes: AcademicClass[] = [];
  let schedules: AcademicScheduleView[] = [];
  let notifications: AcademicNotification[] = [];
  let classPage = emptyPage<AcademicClass>();
  let schedulePage = emptyPage<AcademicScheduleView>();
  let notificationPage = emptyPage<AcademicNotification>();
  let loading = false;
  let issue = '';

  $: canAdmin = data.principal?.grants.some((g) => g.app === 'siakad');
  $: adminApp = data.applications.find((a) => a.name === 'admin');
  $: sessionsOpen = schedules.filter((s) => s.open_session).length;
  $: periodLabel = classes[0]?.period_name ?? 'Belum ada kelas aktif';

  async function load(
    kind: 'all' | 'classes' | 'schedules' | 'notifications' = 'all'
  ) {
    if (loading) return;
    loading = true;
    issue = '';
    try {
      const [cls, sched, notifs] = await Promise.all([
        kind === 'all' || kind === 'classes'
          ? loadResourcePage(
              client,
              '/academic/classes',
              classPage,
              (row) => row.id,
              kind !== 'all'
            )
          : classPage,
        kind === 'all' || kind === 'schedules'
          ? loadResourcePage(
              client,
              '/academic/schedules',
              schedulePage,
              (row) => row.schedule.id,
              kind !== 'all'
            )
          : schedulePage,
        kind === 'all' || kind === 'notifications'
          ? loadResourcePage(
              client,
              '/academic/notifications',
              notificationPage,
              (row) => row.id,
              kind !== 'all'
            )
          : notificationPage
      ]);
      classPage = cls;
      schedulePage = sched;
      notificationPage = notifs;
      classes = cls.rows;
      schedules = sched.rows;
      notifications = notifs.rows;
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loading = false;
    }
  }


  onMount(() => {
    if (data.principal) void load();
  });
</script>

<svelte:head><title>Algohub — SIAKAD</title></svelte:head>

<header class="page-head">
  <div class="page-head-copy">
    <p class="eyebrow">Siakad</p>
    <h1>Ringkasan akademik</h1>
    <p class="lede">{periodLabel}</p>
  </div>
  {#if data.principal}
    <div class="page-head-actions">
      <button class="btn-ghost" onclick={() => load()} disabled={loading}>
        {loading ? 'Memuat…' : 'Perbarui'}
      </button>
      <a class="btn" href="/absensi">Isi presensi</a>
    </div>
  {/if}
</header>

{#if !data.principal}
  <p class="notice">
    Anda belum masuk. Masuk untuk melihat jadwal kelas, melakukan presensi QR,
    atau meminjam alat laboratorium.
  </p>
{:else}
  {#if issue}
    <div class="notice error" role="alert">
      {issue}
      <button class="btn-ghost" onclick={() => load()} disabled={loading}
        >Coba lagi</button
      >
    </div>
  {/if}

  <ul class="stats">
    <li>
      <span class="stat-label">Kelas aktif</span>
      <strong>{classes.length}</strong>
      <small>{classPage.hasMore ? 'sebagian ditampilkan' : 'semua tampil'}</small>
    </li>
    <li>
      <span class="stat-label">Jadwal terdaftar</span>
      <strong>{schedules.length}</strong>
      <small>{schedulePage.hasMore ? 'sebagian ditampilkan' : 'semua tampil'}</small>
    </li>
    <li class:sessions={sessionsOpen > 0}>
      <span class="stat-label">Sesi presensi</span>
      <strong>{sessionsOpen}</strong>
      <small>{sessionsOpen > 0 ? 'menunggu kehadiran' : 'belum ada'}</small>
    </li>
  </ul>

  <div class="cards">
    <article class="card card-primary" aria-labelledby="presensi-title">
      <div class="card-head">
        <h2 id="presensi-title">Presensi QR</h2>
        <span class="pill" class:on={sessionsOpen > 0}
          >{sessionsOpen > 0 ? 'Sesi terbuka' : 'Tidak ada sesi'}</span
        >
      </div>
      {#if sessionsOpen > 0}
        {@const sesi = schedules.find((s) => s.open_session)?.open_session}
        <p class="card-value">{sessionsOpen}</p>
        <p class="muted">
          Sesi terbuka · batas {sesi ? jakarta(sesi.expires_at) : '-'}
        </p>
        <a class="btn" href="/absensi">Isi presensi sekarang</a>
      {:else}
        <p class="muted">
          Belum ada sesi yang dibuka asisten. Halaman presensi tetap bisa dibuka
          untuk memeriksa riwayat kehadiran.
        </p>
        <a class="btn-ghost-link" href="/absensi">Buka halaman presensi →</a>
      {/if}
    </article>

    <article class="card" aria-labelledby="jadwal-title">
      <div class="card-head">
        <h2 id="jadwal-title">Jadwal pekan ini</h2>
        <a class="card-link" href="/absensi">Semua</a>
      </div>
      <ul class="mini-list">
        {#each schedules.slice(0, 4) as item (item.schedule.id)}
          <li>
            <span class="mini-day">{item.schedule.day_of_week.slice(0, 3)}</span>
            <span class="mini-copy">
              <strong>{item.schedule.course_name}</strong>
              <small
                >{item.schedule.starts_at}–{item.schedule.ends_at} · {item.schedule
                  .room}</small
              >
            </span>
            {#if item.open_session}<span class="dot" aria-label="Sesi terbuka"
              ></span>{/if}
          </li>
        {:else}
          <li class="mini-empty">{loading ? 'Memuat…' : 'Belum ada jadwal.'}</li>
        {/each}
      </ul>
    </article>

    <article class="card" aria-labelledby="notif-title">
      <div class="card-head">
        <h2 id="notif-title">Notifikasi</h2>
        <span class="card-count">{notifications.length}</span>
      </div>
      <ul class="mini-list">
        {#each notifications.slice(0, 3) as notif (notif.id)}
          <li class="mini-notif">
            <strong>{notif.title}</strong>
            <small>{jakarta(notif.created_at)}</small>
          </li>
        {:else}
          <li class="mini-empty"
            >{loading ? 'Memuat…' : 'Belum ada notifikasi.'}</li
          >
        {/each}
      </ul>
    </article>
  </div>

  <section class="block" aria-labelledby="kelas-title">
    <div class="block-head">
      <h2 id="kelas-title">Kelas terdaftar</h2>
      <span class="card-count">{classes.length}</span>
    </div>
    <div class="table-wrap">
      <table>
        <caption class="sr-only">Daftar kelas akademik yang terdaftar</caption>
        <thead>
          <tr>
            <th scope="col">Kode</th>
            <th scope="col">Mata kuliah</th>
            <th scope="col">Periode</th>
            <th scope="col">Shift</th>
          </tr>
        </thead>
        <tbody>
          {#each classes as cls (cls.id)}
            <tr data-class-id={cls.id}>
              <td class="mono">{cls.code}</td>
              <th scope="row">{cls.course_name}</th>
              <td>{cls.period_name}</td>
              <td>{cls.shift}</td>
            </tr>
          {:else}
            <tr><td class="empty-cell" colspan="4"
                >{loading
                  ? 'Memuat kelas…'
                  : 'Tidak ada kelas aktif. Menu Inventaris & sewa tetap bisa dipakai.'}</td
              ></tr>
          {/each}
        </tbody>
      </table>
    </div>
    {#if classPage.hasMore}
      <button class="btn-ghost" disabled={loading} onclick={() => load('classes')}
        >Muat kelas lainnya</button
      >
    {/if}
  </section>

  <section class="block" aria-labelledby="lab-title">
    <div class="block-head">
      <h2 id="lab-title">Jadwal laboratorium</h2>
      <span class="card-count">{schedules.length}</span>
    </div>
    <div class="table-wrap">
      <table>
        <caption class="sr-only">Jadwal laboratorium per hari dan shift</caption>
        <thead>
          <tr>
            <th scope="col">Hari</th>
            <th scope="col">Mata kuliah</th>
            <th scope="col">Ruang</th>
            <th scope="col">Jam</th>
            <th scope="col">Presensi</th>
          </tr>
        </thead>
        <tbody>
          {#each schedules as item (item.schedule.id)}
            <tr data-schedule-id={item.schedule.id}>
              <td class="mono">{item.schedule.day_of_week}</td>
              <th scope="row"
                >{item.schedule.course_name}
                <small class="row-sub">({item.schedule.class_code})</small></th
              >
              <td>{item.schedule.room}</td>
              <td class="mono"
                >{item.schedule.starts_at}–{item.schedule.ends_at}</td
              >
              <td>
                {#if item.open_session}
                  <a class="pill on" href="/absensi">Terbuka</a>
                {:else}
                  <span class="pill">-</span>
                {/if}
              </td>
            </tr>
          {:else}
            <tr><td class="empty-cell" colspan="5"
                >{loading
                  ? 'Memuat jadwal…'
                  : 'Belum ada jadwal laboratorium untuk akun ini.'}</td
              ></tr>
          {/each}
        </tbody>
      </table>
    </div>
    {#if schedulePage.hasMore}
      <button
        class="btn-ghost"
        disabled={loading}
        onclick={() => load('schedules')}>Muat jadwal lainnya</button
      >
    {/if}
  </section>

  <div class="quick">
    <a class="quick-link" href="/penyewaan">
      <strong>Inventaris & sewa</strong>
      <span>Pinjam alat laboratorium untuk praktikum.</span>
    </a>
    {#if canAdmin && adminApp}
      <a
        class="quick-link"
        href={`${adminApp.origin}/login?return_to=/siakad`}
      >
        <strong>Admin Siakad</strong>
        <span>Kelola kelas, jadwal, dan notifikasi.</span>
      </a>
    {/if}
  </div>

  {#if notificationPage.hasMore}
    <button
      class="btn-ghost"
      disabled={loading}
      onclick={() => load('notifications')}>Muat notifikasi lainnya</button
    >
  {/if}
{/if}

<style>
  .page-head {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    justify-content: space-between;
    gap: 1rem;
    margin-bottom: 1.5rem;
  }

  .eyebrow {
    margin: 0;
    color: var(--ah-coral);
    font-size: 0.72rem;
    font-weight: 700;
    letter-spacing: 0.14em;
    text-transform: uppercase;
  }

  h1 {
    margin: 0.3rem 0 0;
    font-family: 'Fredoka', ui-rounded, 'Segoe UI', system-ui, sans-serif;
    font-size: clamp(1.6rem, 2.6vw, 2.1rem);
    font-weight: 500;
    letter-spacing: -0.03em;
    line-height: 1.1;
  }

  .lede {
    margin: 0.35rem 0 0;
    color: var(--ah-muted);
    font-size: 0.9rem;
  }

  .page-head-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
  }

  .stats {
    display: grid;
    margin: 0 0 1rem;
    padding: 0;
    gap: 0.75rem;
    grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
    list-style: none;
  }

  .stats li {
    display: grid;
    padding: 0.85rem 1rem;
    border: 1px solid var(--ah-line);
    border-radius: 0.9rem;
    background: #ffffff;
    gap: 0.1rem;
  }

  .stat-label {
    color: var(--ah-muted);
    font-size: 0.75rem;
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }

  .stats strong {
    font-size: 1.9rem;
    font-weight: 500;
    letter-spacing: -0.03em;
    line-height: 1.1;
  }

  .stats small {
    color: var(--ah-muted);
    font-size: 0.75rem;
  }

  .stats li.sessions {
    border-color: color-mix(in srgb, var(--ah-coral) 40%, var(--ah-line));
    background: color-mix(in srgb, var(--ah-coral) 5%, #ffffff);
  }

  .cards {
    display: grid;
    margin-bottom: 1.75rem;
    gap: 0.75rem;
    grid-template-columns: minmax(0, 1.25fr) minmax(0, 1fr) minmax(0, 1fr);
  }

  .card {
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
    padding: 1.1rem 1.2rem 1.2rem;
    border: 1px solid var(--ah-line);
    border-radius: 1rem;
    background: #ffffff;
  }

  .card-primary {
    background: var(--ah-surface);
    border-color: color-mix(in srgb, var(--ah-coral) 24%, var(--ah-line));
  }

  .card-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
  }

  .card-head h2 {
    margin: 0;
    font-size: 0.95rem;
    font-weight: 600;
    letter-spacing: -0.01em;
  }

  .card-value {
    margin: 0.2rem 0 0;
    font-size: 2.7rem;
    font-weight: 500;
    letter-spacing: -0.04em;
    line-height: 1;
  }

  .card-link,
  .card-count {
    color: var(--ah-muted);
    font-size: 0.75rem;
    font-weight: 600;
    text-decoration: none;
  }

  .card-link {
    display: inline-flex;
    min-height: 44px;
    align-items: center;
    padding-inline: 0.15rem;
  }

  .card-link:hover,
  .card-link:focus-visible {
    color: var(--ah-coral);
  }

  .muted {
    margin: 0;
    color: var(--ah-muted);
    font-size: 0.82rem;
    line-height: 1.5;
  }

  .pill {
    display: inline-flex;
    min-height: 28px;
    align-items: center;
    padding: 0.1rem 0.6rem;
    border-radius: 999px;
    color: var(--ah-muted);
    background: color-mix(in srgb, var(--ah-ink) 7%, transparent);
    font-size: 0.75rem;
    font-weight: 600;
    text-decoration: none;
    white-space: nowrap;
  }

  .pill.on {
    color: var(--ah-green-ink);
    background: color-mix(in srgb, var(--ah-green) 26%, #ffffff);
  }

  a.pill {
    min-height: 44px;
  }

  .mini-list {
    display: grid;
    margin: 0;
    padding: 0;
    gap: 0.1rem;
    list-style: none;
  }

  .mini-list li {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.45rem 0;
    border-top: 1px solid color-mix(in srgb, var(--ah-line) 60%, transparent);
  }

  .mini-list li:first-child {
    border-top: 0;
  }

  .mini-day {
    display: grid;
    width: 2.1rem;
    height: 2.1rem;
    flex: 0 0 auto;
    place-items: center;
    border-radius: 0.55rem;
    color: var(--ah-ink);
    background: color-mix(in srgb, var(--ah-ink) 6%, transparent);
    font-size: 0.75rem;
    font-weight: 700;
    text-transform: uppercase;
  }

  .mini-copy {
    display: grid;
    min-width: 0;
    gap: 0.05rem;
  }

  .mini-copy strong {
    overflow: hidden;
    font-size: 0.82rem;
    font-weight: 600;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mini-copy small,
  .mini-notif small {
    color: var(--ah-muted);
    font-size: 0.75rem;
  }

  .mini-notif {
    display: grid;
    align-items: start;
  }

  .mini-notif strong {
    font-size: 0.82rem;
    font-weight: 600;
  }

  .mini-empty {
    color: var(--ah-muted);
    font-size: 0.8rem;
  }

  .dot {
    width: 0.45rem;
    height: 0.45rem;
    flex: 0 0 auto;
    margin-left: auto;
    border-radius: 999px;
    background: var(--ah-green);
  }

  .block {
    margin-bottom: 1.75rem;
  }

  .block-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 0.5rem;
    margin-bottom: 0.6rem;
  }

  .block-head h2 {
    margin: 0;
    font-size: 1rem;
    font-weight: 600;
    letter-spacing: -0.01em;
  }

  .table-wrap {
    overflow-x: auto;
    border: 1px solid var(--ah-line);
    border-radius: 0.9rem;
    background: #ffffff;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
  }

  th,
  td {
    padding: 0.6rem 0.9rem;
    border-bottom: 1px solid color-mix(in srgb, var(--ah-line) 70%, transparent);
    text-align: left;
    vertical-align: middle;
    white-space: nowrap;
  }

  thead th {
    color: var(--ah-muted);
    font-size: 0.75rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  tbody th {
    font-weight: 600;
  }

  tbody tr:last-child th,
  tbody tr:last-child td {
    border-bottom: 0;
  }

  tbody tr:hover {
    background: color-mix(in srgb, var(--ah-coral) 4%, transparent);
  }

  .mono {
    font-variant-numeric: tabular-nums;
    font-weight: 500;
  }

  .row-sub {
    color: var(--ah-muted);
    font-size: 0.75rem;
    font-weight: 400;
  }

  .empty-cell {
    color: var(--ah-muted);
    font-size: 0.82rem;
    white-space: normal;
  }

  .quick {
    display: grid;
    gap: 0.75rem;
    grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
  }

  .quick-link {
    display: grid;
    padding: 0.9rem 1rem;
    border: 1px solid var(--ah-line);
    border-radius: 0.9rem;
    background: #ffffff;
    gap: 0.15rem;
    text-decoration: none;
  }

  .quick-link:hover {
    border-color: color-mix(in srgb, var(--ah-coral) 35%, var(--ah-line));
  }

  .quick-link strong {
    font-size: 0.88rem;
    font-weight: 600;
  }

  .quick-link span {
    color: var(--ah-muted);
    font-size: 0.78rem;
  }

  .btn,
  .btn-ghost,
  .btn-ghost-link {
    display: inline-flex;
    min-height: 44px;
    align-items: center;
    justify-content: center;
    padding: 0.55rem 1rem;
    border: 0;
    border-radius: 0.7rem;
    font: inherit;
    font-size: 0.85rem;
    font-weight: 600;
    text-decoration: none;
    cursor: pointer;
  }

  .btn {
    color: var(--ah-cream);
    background: var(--ah-coral);
  }

  .btn-ghost {
    color: var(--ah-ink);
    background: #ffffff;
    box-shadow: inset 0 0 0 1px var(--ah-line);
  }

  .btn-ghost-link {
    justify-content: flex-start;
    min-height: 32px;
    padding: 0;
    color: var(--ah-coral);
    background: none;
  }

  .btn:hover {
    filter: brightness(1.06);
  }

  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }

  @media (max-width: 68rem) {
    .cards {
      grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    }

    .card-primary {
      grid-column: 1 / -1;
    }
  }

  @media (max-width: 44rem) {
    .cards {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
