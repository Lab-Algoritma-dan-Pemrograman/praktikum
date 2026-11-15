<script lang="ts">
  import { onMount } from 'svelte';
  import {
    canAccessAdminRoute,
    type AppIdentityData
  } from '@algohub/auth-client';
  import {
    PracticumClient,
    errorMessage,
    type Session
  } from '@algohub/practicum-client';
  export let data: AppIdentityData;
  let sessions: Session[] = [];
  let loading = true;
  let issue = '';
  let offset = 0;
  $: student = data.principal?.assignments.some(
    (a) => a.active && a.role === 'mahasiswa'
  );
  $: admin =
    data.principal && canAccessAdminRoute(data.principal, '/praktikum')
      ? data.applications.find((a) => a.name === 'admin')
      : undefined;
  async function load(next = offset) {
    loading = true;
    issue = '';
    try {
      sessions = await new PracticumClient().sessions(next);
      offset = next;
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loading = false;
    }
  }
  onMount(() => {
    if (student) void load();
    else loading = false;
  });
</script>

<svelte:head><title>Sesi Praktikum — Algohub</title></svelte:head>
<header class="page-heading">
  <p class="eyebrow">PRAKTIKUM</p>
  <h1>Sesi praktikum saya</h1>
  <p class="muted">
    Pilih sesi kelas Anda, buka kegiatan yang tersedia, lalu kerjakan sesuai
    waktu yang ditetapkan pengajar.
  </p>
</header>
{#if !data.principal}
  <section class="panel empty-state">
    <h2>Masuk untuk melihat sesi Anda</h2>
    <p>
      Gunakan identitas AlgoHub yang terdaftar pada kelas dan shift praktikum.
    </p>
    <a class="button" href="/login">Masuk ke Praktikum</a>
  </section>
{:else if !student}
  <section class="panel empty-state">
    <h2>Belum ada penugasan mahasiswa aktif</h2>
    <p>
      Sesi baru mengikuti penugasan kelas Anda. Riwayat pengerjaan milik Anda
      tetap dapat dilihat.
    </p>
    <div class="actions">
      <a class="button secondary" href="/nilai">Lihat nilai & riwayat</a>
      {#if admin}<a
          class="button"
          href={`${admin.origin}/login?return_to=/praktikum`}
          >Buka Admin Praktikum</a
        >{/if}
    </div>
  </section>
{:else}
  <section aria-label="Daftar sesi" aria-busy={loading}>
    <div class="section-heading">
      <h2>Sesi kelas Anda</h2>
      <button class="secondary" onclick={() => load()} disabled={loading}
        >Perbarui sesi</button
      >
    </div>
    {#if loading}<p role="status" class="muted">Memuat sesi…</p>{/if}
    {#if issue}<div role="alert" class="notice error">
        {issue}<button
          class="secondary"
          onclick={() => load()}
          disabled={loading}>Coba lagi</button
        >
      </div>{/if}
    {#if !loading && !issue && sessions.length === 0}
      <div class="panel empty-state">
        <h3>Belum ada sesi tersedia</h3>
        <p>
          Pastikan Anda terdaftar pada kelas dan shift yang benar. Hubungi
          pengajar jika sesi seharusnya sudah dibuka.
        </p>
        <a href="/nilai">Lihat riwayat pengerjaan sebelumnya</a>
      </div>
    {/if}
    <div class="session-grid">
      {#each sessions as session (session.id)}
        <article class="panel session-card">
          <div class="card-meta">
            <span class="eyebrow">Sesi {session.sequence}</span><span
              class="badge">{session.active ? 'Aktif' : 'Tidak aktif'}</span
            >
          </div>
          <h3>{session.title}</h3>
          <p class="muted">
            {session.active
              ? 'Lihat kegiatan dan aktivasi yang tersedia untuk kelas Anda.'
              : 'Sesi ini belum aktif. Lihat detail untuk informasi kegiatan.'}
          </p>
          <a
            class="button"
            href={`/sesi/${encodeURIComponent(session.id)}`}
            aria-label={`Buka sesi ${session.title}`}>Buka sesi</a
          >
        </article>
      {/each}
    </div>
    {#if sessions.length > 0 || offset > 0}
      <div class="pagination" aria-label="Halaman sesi">
        <button
          class="secondary"
          disabled={offset === 0 || loading}
          onclick={() => load(offset - 25)}>Sebelumnya</button
        ><span class="muted">Halaman {Math.floor(offset / 25) + 1}</span><button
          class="secondary"
          disabled={sessions.length < 25 || loading}
          onclick={() => load(offset + 25)}>Berikutnya</button
        >
      </div>
    {/if}
  </section>
{/if}
