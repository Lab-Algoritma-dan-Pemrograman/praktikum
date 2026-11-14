<script lang="ts">
  import { SiteFooter } from '@algohub/ui';
  import {
    AuthClient,
    asApiError,
    type ApiError,
    type AppIdentityData,
    type Principal
  } from '@algohub/auth-client';

  const { data }: { data: AppIdentityData & { principal: Principal } } =
    $props();

  const catalogue: Record<string, string> = {
    elearning: 'Kurikulum, level, XP, dan riwayat belajar.',
    praktikum: 'Sesi praktikum, soal, jawaban, dan penilaian.',
    siakad: 'Kelas, jadwal, presensi, dan inventaris.'
  };

  const hasAdmin = $derived(
    data.principal.grants.some((grant) => grant.app === 'admin')
  );

  const workspace = $derived(
    data.applications
      .filter(
        (app) =>
          app.name !== 'portal' &&
          app.name !== 'auth' &&
          (app.name === 'admin' ? hasAdmin : app.name in catalogue)
      )
      .map((app) => ({
        ...app,
        hint: catalogue[app.name] ?? 'Kelola pengguna, permission, dan audit.',
        href: `${app.origin}/api/auth/start?return_to=%2F`
      }))
  );

  const badges = $derived(
    data.principal.assignments
      .filter((assignment) => assignment.active)
      .map((assignment) =>
        assignment.division
          ? `${assignment.role} · ${assignment.division}`
          : assignment.role
      )
  );

  let signingOut = $state(false);
  let issue = $state<ApiError | null>(null);

  async function signOut() {
    signingOut = true;
    issue = null;
    try {
      await new AuthClient().logout();
      location.assign(`${data.authOrigin}/logout`);
    } catch (cause) {
      issue = asApiError(cause);
      signingOut = false;
    }
  }
</script>

<svelte:head>
  <title>Ruang kerja — Algohub</title>
  <meta name="robots" content="noindex" />
</svelte:head>

<div class="workspace">
  <header class="bar">
    <a class="brand" href="/" aria-label="Algohub beranda">
      <img
        src="/logo-mark.png"
        alt=""
        width="34"
        height="59"
        decoding="async"
      />
      <span class="wordmark">Algohub</span>
    </a>
    <div class="who">
      <span class="name">{data.principal.user.name}</span>
      <span class="nim"
        >{data.principal.user.nim ?? data.principal.user.email}</span
      >
      <button type="button" class="out" onclick={signOut} disabled={signingOut}>
        {signingOut ? 'Keluar…' : 'Keluar'}
      </button>
    </div>
  </header>

  <main id="content">
    <p class="eyebrow">RUANG KERJA</p>
    <h1>Mau mengerjakan apa hari ini?</h1>
    <p class="lead">
      Semua aplikasi memakai satu identitas. Pilih salah satu, kamu tidak perlu
      masuk ulang.
    </p>

    {#if badges.length}
      <ul class="badges" aria-label="Peran akun">
        {#each badges as badge (badge)}
          <li>{badge}</li>
        {/each}
      </ul>
    {/if}

    {#if issue}
      <p class="issue" role="alert">{issue.message}</p>
    {/if}

    <ul class="tiles">
      {#each workspace as app (app.name)}
        <li>
          <a class="tile" href={app.href}>
            <span class="tile-label">{app.label}</span>
            <span class="tile-hint">{app.hint}</span>
            <span class="tile-go" aria-hidden="true">Buka →</span>
          </a>
        </li>
      {/each}
    </ul>

    <p class="note">
      Butuh perubahan peran atau kelas? Hubungi koordinator laboratorium lewat
      halaman <a href="/bantuan">bantuan</a>.
    </p>
  </main>

  <SiteFooter
    links={[
      { label: 'Beranda', href: '/' },
      { label: 'Bantuan', href: '/bantuan' },
      { label: 'Modul', href: '/modul' }
    ]}
  />
</div>

<style>
  .workspace {
    --wrap: min(64rem, calc(100% - 2.5rem));
    display: flex;
    flex-direction: column;
    min-height: 100dvh;
    color: var(--ah-ink);
    background: var(--ah-cream);
  }

  .bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    width: var(--wrap);
    margin: 0 auto;
    padding: 1.25rem 0;
    border-bottom: 1px solid var(--ah-line);
  }

  .brand {
    display: inline-flex;
    align-items: center;
    gap: 0.55rem;
    color: var(--ah-ink-strong);
    text-decoration: none;
    font-weight: 800;
    letter-spacing: -0.02em;
  }

  .brand img {
    width: 22px;
    height: auto;
  }

  .who {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    font-size: 0.875rem;
  }

  .name {
    font-weight: 700;
    color: var(--ah-ink-strong);
  }

  .nim {
    color: var(--ah-muted);
    font-variant-numeric: tabular-nums;
  }

  .out {
    border: 1px solid var(--ah-line-strong);
    border-radius: var(--ah-radius-sm);
    background: transparent;
    padding: 0.4rem 0.9rem;
    color: var(--ah-ink-strong);
    font: inherit;
    font-weight: 650;
    cursor: pointer;
  }

  .out:hover:not(:disabled) {
    background: var(--ah-ink-strong);
    color: var(--ah-cream);
  }

  .out:disabled {
    opacity: 0.6;
    cursor: progress;
  }

  main {
    flex: 1;
    width: var(--wrap);
    margin: 0 auto;
    padding: 3.5rem 0 4rem;
  }

  .eyebrow {
    margin: 0 0 0.75rem;
    font-size: 0.75rem;
    font-weight: 800;
    letter-spacing: 0.14em;
    color: var(--ah-muted);
  }

  h1 {
    margin: 0;
    max-width: 24ch;
    font-size: clamp(2rem, 4.5vw, 3rem);
    line-height: 1.05;
    letter-spacing: -0.035em;
    color: var(--ah-ink-strong);
  }

  .lead {
    max-width: 46ch;
    margin: 1rem 0 0;
    line-height: 1.65;
    color: var(--ah-muted);
  }

  .badges {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    margin: 1.5rem 0 0;
    padding: 0;
    list-style: none;
  }

  .badges li {
    border: 1px solid var(--ah-line);
    border-radius: 999px;
    padding: 0.3rem 0.75rem;
    font-size: 0.8125rem;
    font-weight: 650;
    color: var(--ah-ink);
    background: #fff;
  }

  .issue {
    margin: 1.5rem 0 0;
    border: 1px solid var(--ah-line-strong);
    border-radius: var(--ah-radius-sm);
    padding: 0.75rem 1rem;
    font-size: 0.875rem;
  }

  .tiles {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(15rem, 1fr));
    gap: 1rem;
    margin: 2.5rem 0 0;
    padding: 0;
    list-style: none;
  }

  .tile {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    height: 100%;
    box-sizing: border-box;
    border: 1px solid var(--ah-line);
    border-radius: var(--ah-radius-md);
    padding: 1.5rem;
    background: #fff;
    color: inherit;
    text-decoration: none;
    transition:
      transform 160ms var(--ah-ease-out),
      border-color 160ms var(--ah-ease-out);
  }

  .tile:hover {
    transform: translateY(-2px);
    border-color: var(--ah-ink-strong);
  }

  .tile-label {
    font-size: 1.125rem;
    font-weight: 800;
    letter-spacing: -0.02em;
    color: var(--ah-ink-strong);
  }

  .tile-hint {
    font-size: 0.9375rem;
    line-height: 1.55;
    color: var(--ah-muted);
  }

  .tile-go {
    margin-top: auto;
    padding-top: 0.75rem;
    font-size: 0.875rem;
    font-weight: 700;
    color: var(--ah-ink-strong);
  }

  .note {
    margin: 2.5rem 0 0;
    font-size: 0.875rem;
    color: var(--ah-muted);
  }

  .note a {
    color: var(--ah-ink-strong);
  }

  @media (max-width: 40rem) {
    .bar {
      flex-wrap: wrap;
    }

    .who {
      width: 100%;
      justify-content: space-between;
    }

    main {
      padding: 2.5rem 0 3rem;
    }
  }
</style>
