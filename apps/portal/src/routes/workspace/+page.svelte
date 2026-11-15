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

  const catalogue: Record<string, { kicker: string; hint: string }> = {
    siakad: {
      kicker: 'Akademik',
      hint: 'Kelas, jadwal, presensi, dan inventaris.'
    },
    elearning: {
      kicker: 'Pembelajaran',
      hint: 'Kurikulum, level, XP, dan riwayat belajar.'
    },
    praktikum: {
      kicker: 'Praktikum',
      hint: 'Sesi praktikum, soal, jawaban, dan penilaian.'
    },
    admin: {
      kicker: 'Operasional',
      hint: 'Kelola pengguna, permission, dan audit.'
    }
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
        kicker: catalogue[app.name]?.kicker ?? 'Operasional',
        hint:
          catalogue[app.name]?.hint ??
          'Kelola pengguna, permission, dan audit.',
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
    {:else}
      <p class="badges-empty">
        Belum ada penugasan. Hubungi koordinator untuk akses.
      </p>
    {/if}

    {#if issue}
      <p class="issue" role="alert">{issue.message}</p>
    {/if}

    <ul class="tiles">
      {#each workspace as app (app.name)}
        <li>
          <a class="tile" href={app.href} data-app={app.name}>
            <span class="tile-media" aria-hidden="true"></span>
            <span class="tile-scrim" aria-hidden="true"></span>
            <span class="tile-orbit" aria-hidden="true"></span>
            <span class="tile-kicker">{app.kicker}</span>
            <span class="tile-body">
              <span class="tile-title">{app.label}</span>
              <span class="tile-hint">{app.hint}</span>
            </span>
            <span class="tile-go" aria-hidden="true">
              <svg
                viewBox="0 0 24 24"
                width="22"
                height="22"
                fill="none"
                stroke="currentColor"
                stroke-width="2.25"
                stroke-linecap="round"
                stroke-linejoin="round"
              >
                <path d="M7 17 17 7" />
                <path d="M8.5 7H17v8.5" />
              </svg>
            </span>
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

  .badges-empty {
    margin: 1.5rem 0 0;
    font-size: 0.875rem;
    color: var(--ah-muted);
  }

  .issue {
    margin: 1.5rem 0 0;
    border: 1px solid var(--ah-line-strong);
    border-radius: var(--ah-radius-sm);
    padding: 0.75rem 1rem;
    font-size: 0.875rem;
  }

  .tiles {
    --tile-pad: 1.25rem;
    --tile-lime: #c7f19d;
    --tile-ink: #0f1f10;
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(15rem, 1fr));
    gap: 1rem;
    margin: 2.5rem 0 0;
    padding: 0;
    list-style: none;
  }

  .tiles li {
    display: flex;
  }

  /* Circle Expand Card: media fills the tile, one lime circle grows on hover. */
  .tile {
    position: relative;
    isolation: isolate;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    gap: 2.5rem;
    width: 100%;
    min-height: 16.5rem;
    box-sizing: border-box;
    padding: var(--tile-pad);
    border: 0;
    border-radius: 1rem;
    overflow: hidden;
    background: var(--ah-ink-strong);
    text-decoration: none;
    color: #fff;
    transition: color 320ms var(--ah-ease-out);
  }

  .tile-media,
  .tile-scrim,
  .tile-orbit {
    position: absolute;
    inset: 0;
    pointer-events: none;
  }

  .tile-media {
    z-index: 0;
    background: linear-gradient(150deg, #8a1538 0%, #2d3142 62%, #1b1f2a 100%);
  }

  .tile-media::after {
    content: '';
    position: absolute;
    inset: 0;
    background:
      radial-gradient(
        120% 90% at 78% 8%,
        rgba(255, 255, 255, 0.35),
        transparent 58%
      ),
      radial-gradient(
        80% 70% at 12% 96%,
        rgba(255, 255, 255, 0.14),
        transparent 60%
      );
    mix-blend-mode: soft-light;
  }

  /* Dark overlay + bottom text mask, same idea as the reference card. */
  .tile-scrim {
    z-index: 0;
    background:
      linear-gradient(180deg, rgba(0, 0, 0, 0) 34%, rgba(0, 0, 0, 0.45) 100%),
      rgba(0, 0, 0, 0.3);
    transition: opacity 320ms var(--ah-ease-out);
  }

  /* The circle sits exactly under the arrow button, then grows to fill. */
  .tile-orbit {
    z-index: 1;
    inset: auto;
    top: var(--tile-pad);
    right: var(--tile-pad);
    width: 3.375rem;
    height: 3.375rem;
    border-radius: 999px;
    background: var(--tile-lime);
    transform: scale(1);
    transform-origin: center;
    transition: transform 520ms var(--ah-ease-out);
  }

  .tile-kicker {
    position: relative;
    z-index: 2;
    padding-right: 4rem;
    font-size: 0.75rem;
    font-weight: 650;
    letter-spacing: 0.05em;
    line-height: 1;
    text-transform: uppercase;
  }

  .tile-body {
    position: relative;
    z-index: 2;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .tile-title {
    font-size: 2rem;
    font-weight: 700;
    line-height: 1.2;
    letter-spacing: -0.02em;
  }

  .tile-hint {
    font-size: 0.9375rem;
    font-weight: 500;
    line-height: 1.4;
    letter-spacing: -0.01em;
    color: rgba(255, 255, 255, 0.9);
    transition: color 320ms var(--ah-ease-out);
  }

  .tile-go {
    position: absolute;
    z-index: 3;
    top: var(--tile-pad);
    right: var(--tile-pad);
    display: grid;
    place-items: center;
    width: 3.375rem;
    height: 3.375rem;
    color: var(--tile-ink);
    transition:
      transform 320ms var(--ah-ease-out),
      color 320ms var(--ah-ease-out);
  }

  .tile:hover,
  .tile:focus-visible {
    color: var(--tile-ink);
  }

  .tile:hover .tile-scrim,
  .tile:focus-visible .tile-scrim {
    opacity: 0;
  }

  .tile:hover .tile-orbit,
  .tile:focus-visible .tile-orbit {
    transform: scale(26);
  }

  .tile:hover .tile-hint,
  .tile:focus-visible .tile-hint {
    color: rgba(15, 31, 16, 0.78);
  }

  .tile:hover .tile-go,
  .tile:focus-visible .tile-go {
    transform: rotate(45deg);
  }

  .tile:focus-visible {
    outline: var(--ah-focus-brand);
    outline-offset: 3px;
  }

  .tile[data-app='siakad'] .tile-media {
    background: linear-gradient(150deg, #8a1538 0%, #2d3142 62%, #1b1f2a 100%);
  }

  .tile[data-app='elearning'] .tile-media {
    background: linear-gradient(150deg, #1f5a75 0%, #26405c 58%, #1b1f2a 100%);
  }

  .tile[data-app='praktikum'] .tile-media {
    background: linear-gradient(150deg, #0b6b57 0%, #25453c 58%, #1b1f2a 100%);
  }

  .tile[data-app='admin'] .tile-media {
    background: linear-gradient(150deg, #5a3a7a 0%, #33304f 58%, #1b1f2a 100%);
  }

  @media (prefers-reduced-motion: reduce) {
    .tile,
    .tile-scrim,
    .tile-orbit,
    .tile-go,
    .tile-hint {
      transition-duration: 1ms;
    }

    .tile:hover .tile-orbit,
    .tile:focus-visible .tile-orbit {
      transform: scale(1);
    }

    .tile:hover .tile-go,
    .tile:focus-visible .tile-go {
      transform: none;
    }
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
