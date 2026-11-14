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
<section class="stack" aria-labelledby="siakad-overview-title">
  <h2 id="siakad-overview-title">Kelas, jadwal, dan notifikasi saya</h2>
  {#if !data.principal}
    <p class="notice">
      Anda belum masuk. Gunakan tautan Masuk di atas untuk melihat jadwal kelas,
      melakukan presensi QR, atau meminjam alat laboratorium.
    </p>
  {:else}
    <div class="actions">
      <button class="secondary" onclick={() => load()} disabled={loading}
        >Perbarui data</button
      >
      <a class="button secondary" href="/absensi">Buka Presensi QR</a>
      <a class="button secondary" href="/penyewaan">Sewa / Pinjam Alat</a>
      {#if canAdmin && adminApp}<a
          class="button secondary"
          href={`${adminApp.origin}/login?return_to=/siakad`}
          >Buka Admin SIAKAD</a
        >{/if}
    </div>
    {#if loading}<p role="status">Memuat data akademik…</p>{/if}
    {#if issue}<div role="alert" class="notice error">
        {issue}
        <button onclick={() => load()} disabled={loading}>Coba lagi</button>
      </div>{/if}

    <section class="stack" aria-label="Daftar jadwal laboratorium">
      <h3>Jadwal laboratorium</h3>
      <div class="grid">
        {#each schedules as item}
          <article class="panel stack" data-schedule-id={item.schedule.id}>
            <span class="badge"
              >{item.schedule.day_of_week} · Shift {item.schedule.shift}</span
            >
            <h4>{item.schedule.course_name} ({item.schedule.class_code})</h4>
            <p class="muted">
              Ruang: {item.schedule.room} · {item.schedule.starts_at} – {item
                .schedule.ends_at}
            </p>
            {#if item.open_session}
              <p class="notice success">
                Sesi presensi terbuka: <strong>{item.open_session.title}</strong
                ><br />
                Batas waktu: {jakarta(item.open_session.expires_at)}
              </p>
              <a class="button" href="/absensi">Isi presensi sekarang</a>
            {/if}
          </article>
        {:else}
          {#if !loading}<p class="notice">
              Belum ada jadwal kelas yang terdaftar untuk akun Anda.
            </p>{/if}
        {/each}
      </div>
      {#if schedulePage.hasMore}
        <button
          class="secondary"
          disabled={loading}
          onclick={() => load('schedules')}>Muat jadwal lainnya</button
        >
      {/if}
    </section>

    <section class="stack" aria-label="Daftar kelas terdaftar">
      <h3>Kelas terdaftar</h3>
      <div class="grid">
        {#each classes as cls}
          <article class="panel" data-class-id={cls.id}>
            <p class="muted">{cls.period_name} · Shift {cls.shift}</p>
            <h4>{cls.code} — {cls.course_name}</h4>
            <p class="muted">Kode MK: {cls.course_code}</p>
          </article>
        {:else}
          {#if !loading}<p class="notice">
              Tidak ada kelas aktif untuk akun ini. Pengguna peminjam tetap
              dapat mengakses menu Inventaris & Penyewaan.
            </p>{/if}
        {/each}
      </div>
      {#if classPage.hasMore}
        <button
          class="secondary"
          disabled={loading}
          onclick={() => load('classes')}>Muat kelas lainnya</button
        >
      {/if}
    </section>

    <section class="stack" aria-label="Notifikasi SIAKAD">
      <h3>Notifikasi terbaru</h3>
      {#each notifications as notif}
        <article class="panel" data-notification-id={notif.id}>
          <strong>{notif.title}</strong>
          <p>{notif.message}</p>
          <p class="muted">{jakarta(notif.created_at)}</p>
        </article>
      {:else}
        {#if !loading}<p class="notice">Belum ada notifikasi baru.</p>{/if}
      {/each}
      {#if notificationPage.hasMore}
        <button
          class="secondary"
          disabled={loading}
          onclick={() => load('notifications')}>Muat notifikasi lainnya</button
        >
      {/if}
    </section>
  {/if}
</section>
