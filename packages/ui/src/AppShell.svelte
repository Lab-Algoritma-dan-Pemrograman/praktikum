<script lang="ts">
  import './tokens.css';
  import { onMount } from 'svelte';
  import {
    AuthClient,
    asApiError,
    type ApiError,
    type Principal,
    type ApplicationLink
  } from '@algohub/auth-client';
  import RequestError from './RequestError.svelte';
  export let appName = 'Algohub';
  export let domain = 'algohub.web.id';
  export let description = 'Identity dan ruang aplikasi Algohub.';
  export let principal: Principal | null = null;
  export let applications: ApplicationLink[] = [];
  export let authOrigin = '';
  /**
   * `identity` renders the full identity page (auth center).
   * `app` renders only a slim header so app pages keep their content above the
   * fold; account details and logout move into a compact account menu.
   */
  export let variant: 'identity' | 'app' = 'identity';
  let ready = false;
  let busy = false;
  onMount(() => {
    ready = true;
  });
  let issue: ApiError | null = null;

  $: roleText =
    (principal?.assignments ?? [])
      .filter((assignment) => assignment.active)
      .map(
        (assignment) =>
          `${assignment.role}${assignment.division ? ` · ${assignment.division}` : ''}`
      )
      .join(', ') || 'Belum ada penugasan. Hubungi koordinator untuk akses.';
  $: initials = (principal?.user.name ?? 'A')
    .split(/\s+/)
    .slice(0, 2)
    .map((word) => word[0])
    .join('')
    .toUpperCase();

  async function logout() {
    busy = true;
    issue = null;
    try {
      await new AuthClient().logout();
      location.assign(`${authOrigin}/logout`);
    } catch (error) {
      issue = asApiError(error);
    } finally {
      busy = false;
    }
  }
</script>

<svelte:head><meta name="description" content={description} /></svelte:head>
<div class="shell" data-variant={variant}>
  <header class="topbar">
    <a class="brand" href="/" aria-label="Algohub beranda"
      ><span class="brand-mark" aria-hidden="true">A</span><span>Algohub</span
      ></a
    >
    <span class="domain">{domain}</span>
    <div class="topbar-actions">
      <details class="app-switcher">
        <summary>Pindah aplikasi</summary>
        <nav aria-label="Aplikasi Algohub">
          {#each applications as application}<a
              href={`${application.origin}/login`}>{application.label}</a
            >{/each}
        </nav>
      </details>
      {#if principal}
        <details class="user-menu">
          <summary aria-label={`Menu akun ${principal.user.name}`}>
            <span class="user-avatar" aria-hidden="true">{initials}</span>
            <span class="user-menu-name">{principal.user.name}</span>
          </summary>
          <div class="user-panel">
            <p class="user-panel-name">{principal.user.name}</p>
            <p class="email">{principal.user.email}</p>
            <p class="roles">{roleText}</p>
            <button type="button" onclick={logout} disabled={!ready || busy}
              >{busy ? 'Mengakhiri sesi…' : 'Keluar'}</button
            >
          </div>
        </details>
      {:else}
        <a class="button" href="/login">Masuk</a>
      {/if}
    </div>
  </header>
  {#if variant === 'app'}
    <h1 class="visually-hidden">{appName}</h1>
    {#if issue}<div class="shell-alert">
        <RequestError error={issue} />
      </div>{/if}
  {:else}
    <main class="content">
      <img
        class="lab-wordmark"
        src="/logo-lab.png"
        alt="Laboratorium Algoritma dan Pemrograman"
        width="480"
        height="165"
      />
      <p class="eyebrow">IDENTITY / SINGLE SIGN-ON</p>
      <h1>{appName}</h1>
      <p class="description">{description}</p>
      <section class="identity" aria-label="Sesi aplikasi">
        {#if principal}
          <p class="signed-in">
            Masuk sebagai <strong>{principal.user.name}</strong>
          </p>
          <p class="email">{principal.user.email}</p>
          <p>{roleText}</p>
          <button type="button" onclick={logout} disabled={!ready || busy}
            >{busy ? 'Mengakhiri sesi…' : 'Keluar'}</button
          >
        {:else}
          <p>
            Masuk melalui Auth Center untuk menggunakan identitas yang sama di
            semua aplikasi.
          </p>
          <a class="button" href="/login">Masuk</a>
        {/if}
        <RequestError error={issue} />
      </section>
      <p class="note">
        Gunakan menu Pindah aplikasi untuk membuka layanan sesuai penugasan dan
        izin akses Anda.
      </p>
    </main>
  {/if}
</div>

<style>
  .shell {
    min-height: 100vh;
    color: var(--ah-console-text);
  }
  .shell[data-variant='app'] {
    min-height: auto;
  }
  .visually-hidden {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    clip-path: inset(50%);
    white-space: nowrap;
  }
  .topbar {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 1rem;
    min-height: 4.5rem;
    padding: 0.75rem 5vw;
    border-bottom: 1px solid var(--ah-console-line-subtle);
    background: var(--ah-console-raised);
  }
  .brand {
    display: inline-flex;
    align-items: center;
    gap: 0.65rem;
    color: inherit;
    font-weight: 700;
    text-decoration: none;
  }
  .lab-wordmark {
    display: block;
    width: clamp(9rem, 26vw, 13rem);
    height: auto;
    margin: 0 0 1.5rem;
  }
  .brand-mark {
    display: grid;
    width: 2rem;
    height: 2rem;
    place-items: center;
    border-radius: var(--ah-console-radius-badge);
    color: var(--ah-console-card);
    background: var(--ah-console-accent);
  }
  .domain {
    color: var(--ah-console-muted);
    font-size: 0.85rem;
    overflow-wrap: anywhere;
  }
  .topbar-actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-left: auto;
  }
  .app-switcher,
  .user-menu {
    position: relative;
  }
  summary {
    display: flex;
    min-height: 44px;
    align-items: center;
    gap: 0.5rem;
    color: var(--ah-console-accent);
    cursor: pointer;
    font-weight: 700;
  }
  .user-menu summary {
    padding: 0.25rem 0.65rem 0.25rem 0.35rem;
    border: 1px solid var(--ah-console-line);
    border-radius: 999px;
    color: var(--ah-console-heading);
    background: var(--ah-console-card);
    font-weight: 600;
  }
  .user-avatar {
    display: grid;
    width: 2rem;
    height: 2rem;
    flex-shrink: 0;
    place-items: center;
    border-radius: 999px;
    color: var(--ah-console-card);
    background: var(--ah-console-accent);
    font-size: 0.78rem;
    font-weight: 800;
    letter-spacing: 0;
  }
  .user-menu-name {
    max-width: 12rem;
    overflow: hidden;
    font-size: 0.9rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .app-switcher nav,
  .user-panel {
    position: absolute;
    z-index: 2;
    right: 0;
    display: grid;
    min-width: 12rem;
    padding: 0.5rem;
    border: 1px solid var(--ah-console-line);
    border-radius: var(--ah-console-radius-control);
    background: var(--ah-console-card);
    box-shadow: var(--ah-shadow-float);
  }
  .user-panel {
    gap: 0.25rem;
    min-width: 15rem;
    padding: 0.85rem;
  }
  .user-panel p {
    margin: 0;
    line-height: 1.5;
  }
  .user-panel-name {
    color: var(--ah-console-heading);
    font-weight: 700;
  }
  .app-switcher nav a {
    display: flex;
    min-height: 44px;
    align-items: center;
    padding: 0.65rem;
    border-radius: var(--ah-console-radius-badge);
    color: var(--ah-console-accent);
    text-decoration: none;
  }
  .app-switcher nav a:hover {
    background: var(--ah-console-accent-soft);
  }
  .user-panel .roles {
    color: var(--ah-console-muted);
    font-size: 0.85rem;
  }
  .shell-alert {
    width: min(73.5rem, 92vw);
    margin: 1rem auto 0;
  }
  .content {
    width: min(44rem, 90vw);
    margin: 0 auto;
    padding: clamp(3rem, 9vw, 7rem) 0;
  }
  .eyebrow {
    margin: 0 0 1rem;
    color: var(--ah-console-accent-ink);
    font-size: 0.75rem;
    font-weight: 800;
    letter-spacing: 0.12em;
  }
  h1 {
    max-width: 14ch;
    margin: 0;
    color: var(--ah-console-heading);
    font-size: clamp(2.5rem, 8vw, 5rem);
    line-height: 1;
    letter-spacing: -0.055em;
  }
  p {
    line-height: 1.65;
    overflow-wrap: anywhere;
  }
  .description {
    color: var(--ah-console-muted);
    font-size: clamp(1rem, 2vw, 1.2rem);
  }
  .identity {
    margin-top: 2rem;
    padding: clamp(1rem, 3vw, 1.5rem);
    border: 1px solid var(--ah-console-line);
    border-radius: var(--ah-console-radius-card);
    background: var(--ah-console-card);
    box-shadow: var(--ah-shadow-card);
  }
  .signed-in,
  .email {
    margin: 0.25rem 0;
  }
  .email,
  .note {
    color: var(--ah-console-muted);
    font-size: 0.9rem;
  }
  button,
  .button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-height: 44px;
    padding: 0.7rem 1rem;
    border: 0;
    border-radius: var(--ah-console-radius-control);
    color: var(--ah-console-card);
    background: var(--ah-console-accent);
    font: inherit;
    text-decoration: none;
    cursor: pointer;
  }
  button:disabled {
    opacity: 0.65;
    cursor: wait;
  }
  @media (max-width: 640px) {
    .domain {
      display: none;
    }
    .topbar {
      gap: 0.6rem;
      padding: 0.6rem 1rem;
    }
    .user-menu-name {
      display: none;
    }
    .user-panel {
      min-width: min(16rem, calc(100vw - 2rem));
    }
  }
</style>
