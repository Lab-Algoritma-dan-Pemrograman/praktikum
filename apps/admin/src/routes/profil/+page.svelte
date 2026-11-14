<script lang="ts">
  import { onMount } from 'svelte';
  import { invalidateAll } from '$app/navigation';
  import {
    AuthClient,
    asApiError,
    type ApiError,
    type SessionView,
    type UserProfile
  } from '@algohub/auth-client';
  import { RequestError } from '@algohub/ui';
  export let data: { profile: UserProfile; sessions: SessionView[] };
  const api = new AuthClient();
  let name = '';
  let busy = true;
  onMount(() => {
    busy = false;
  });
  let notice = '';
  let issue: ApiError | null = null;
  let confirmation = '';
  $: name = data.profile.name;
  const date = (value: string) =>
    new Date(value).toLocaleString('id-ID', { timeZone: 'UTC' });

  async function reload() {
    issue = null;
    notice = '';
    await invalidateAll();
  }

  async function perform(action: () => Promise<unknown>, message: string) {
    busy = true;
    notice = '';
    issue = null;
    try {
      await action();
      confirmation = '';
      notice = message;
      await invalidateAll();
    } catch (error) {
      issue = asApiError(error);
    } finally {
      busy = false;
    }
  }
</script>

<svelte:head><title>Profil — Algohub Admin</title></svelte:head>
<section class="identity-page">
  <p class="eyebrow">IDENTITY / PROFIL</p>
  <h1>Profil dan sesi</h1>
  <p>
    Kelola nama tampilan dan sesi Anda. Email dan penugasan dikelola melalui
    identity pusat.
  </p>
  <RequestError error={issue} onRetry={reload} returnTo="/profil" />
  {#if notice}<p class="notice" role="status">{notice}</p>{/if}
  <section class="identity-panel" aria-labelledby="profile-title">
    <h2 id="profile-title">Profil aplikasi</h2>
    <form
      onsubmit={(event) => {
        event.preventDefault();
        void perform(
          () => api.updateProfile(name.trim()),
          'Profil berhasil diperbarui.'
        );
      }}
    >
      <label for="profile-email"
        >Email<input
          id="profile-email"
          type="email"
          value={data.profile.email}
          readonly
        /></label
      >
      <label for="profile-name"
        >Nama tampilan<input
          id="profile-name"
          bind:value={name}
          autocomplete="name"
          required
          maxlength="120"
          disabled={busy}
        /></label
      >
      <button type="submit" disabled={busy || !name.trim()}
        >{busy ? 'Memproses…' : 'Simpan profil'}</button
      >
    </form>
  </section>
  <section
    class="identity-panel"
    aria-labelledby="sessions-title"
    aria-busy={busy}
  >
    <h2 id="sessions-title">Sesi aplikasi</h2>
    <p class="muted">
      Waktu ditampilkan dalam UTC. Memperbarui token tidak memperpanjang batas
      waktu sesi.
    </p>
    <button
      class="secondary"
      type="button"
      disabled={busy}
      onclick={() =>
        perform(
          () => api.refresh(),
          'Token sesi diperbarui tanpa memperpanjang masa berlaku.'
        )}>Perbarui token sesi</button
    >
    {#if !data.sessions.length}<p>
        Belum ada sesi yang dapat ditampilkan.
      </p>{/if}
    <ul>
      {#each data.sessions as session}
        <li class="session" data-session-id={session.id}>
          <h3>{session.host}</h3>
          <p class="muted">
            Dibuat {date(session.created_at)} · Berakhir {date(
              session.expires_at
            )} UTC
          </p>
          {#if session.revoked}<p>Sesi telah dicabut.</p>
          {:else if confirmation === session.id}
            <p>
              Cabut sesi pada host ini? Sesi yang dicabut harus masuk kembali.
            </p>
            <div class="actions">
              <button
                class="danger"
                type="button"
                disabled={busy}
                onclick={() =>
                  perform(
                    () => api.revokeSession(session.id),
                    'Sesi berhasil dicabut.'
                  )}>Ya, cabut sesi</button
              ><button
                class="secondary"
                type="button"
                disabled={busy}
                onclick={() => (confirmation = '')}>Batal</button
              >
            </div>
          {:else}<button
              class="danger"
              type="button"
              disabled={busy}
              aria-label={`Cabut sesi ${session.host}`}
              onclick={() => (confirmation = session.id)}>Cabut sesi</button
            >{/if}
        </li>
      {/each}
    </ul>
  </section>
</section>
