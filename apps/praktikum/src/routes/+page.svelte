<script lang="ts">
  import { onMount } from 'svelte';
  import type { AppIdentityData } from '@algohub/auth-client';
  import {
    PracticumClient,
    errorMessage,
    type Session
  } from '@algohub/practicum-client';
  export let data: AppIdentityData;
  let sessions: Session[] = [];
  let loading = false;
  let issue = '';
  let offset = 0;
  $: student = data.principal?.assignments.some(
    (a) => a.active && a.role === 'mahasiswa'
  );
  $: admin = data.applications.find((a) => a.name === 'admin');
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
  });
</script>

<svelte:head><title>Sesi Praktikum — Algohub</title></svelte:head>
<section aria-labelledby="sessions-title" class="stack">
  <h2 id="sessions-title">Sesi praktikum saya</h2>
  {#if !data.principal}
    <p class="notice">
      Anda belum masuk. Gunakan tautan Masuk di atas untuk membuka sesi yang
      terdaftar.
    </p>
  {:else if !student}
    <div class="notice">
      <p>
        Akun ini tidak memiliki penugasan mahasiswa aktif. Tidak ada sesi
        mahasiswa untuk ditampilkan.
      </p>
      {#if admin}<a href={`${admin.origin}/login?return_to=/praktikum`}
          >Buka Admin Praktikum</a
        >{/if}
    </div>
  {:else}
    {#if loading}<p role="status">Memuat sesi…</p>{/if}
    {#if issue}<div role="alert" class="notice error">
        {issue}
        <button onclick={() => load()} disabled={loading}>Coba lagi</button>
      </div>{/if}
    {#if !loading && !issue && sessions.length === 0}<p class="notice">
        Belum ada sesi yang tersedia dalam roster Anda. Hubungi pengajar untuk
        akses kelas/shift.
      </p>{/if}
    <div class="grid">
      {#each sessions as session}<article class="panel">
          <p class="muted">
            Sesi {session.sequence} · {session.active ? 'Aktif' : 'Tidak aktif'}
          </p>
          <h3>{session.title}</h3>
          <a class="button" href={`/sesi/${encodeURIComponent(session.id)}`}
            >Buka sesi</a
          >
        </article>{/each}
    </div>
    <div class="actions">
      <button
        class="secondary"
        disabled={offset === 0 || loading}
        onclick={() => load(offset - 25)}>Sebelumnya</button
      ><button
        class="secondary"
        disabled={sessions.length < 25 || loading}
        onclick={() => load(offset + 25)}>Berikutnya</button
      >
    </div>
  {/if}
</section>
