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
  let ready = false;
  let busy = false;
  onMount(() => {
    ready = true;
  });
  let issue: ApiError | null = null;

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
<div class="shell">
  <header class="topbar">
    <a class="brand" href="/" aria-label="Algohub beranda"
      ><span class="brand-mark" aria-hidden="true">A</span><span>Algohub</span
      ></a
    >
    <span class="domain">{domain}</span>
    <details class="app-switcher">
      <summary>Pindah aplikasi</summary>
      <nav aria-label="Aplikasi Algohub">
        {#each applications as application}<a
            href={`${application.origin}/login`}>{application.label}</a
          >{/each}
      </nav>
    </details>
  </header>
  <main class="content">
    <p class="eyebrow">IDENTITY / SINGLE SIGN-ON</p>
    <h1>{appName}</h1>
    <p class="description">{description}</p>
    <section class="identity" aria-label="Sesi aplikasi">
      {#if principal}
        <p class="signed-in">
          Masuk sebagai <strong>{principal.user.name}</strong>
        </p>
        <p class="email">{principal.user.email}</p>
        <p>
          {principal.assignments
            .filter((assignment) => assignment.active)
            .map(
              (assignment) =>
                `${assignment.role}${assignment.division ? ` · ${assignment.division}` : ''}`
            )
            .join(', ') ||
            'Belum ada penugasan. Hubungi koordinator untuk akses.'}
        </p>
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
</div>

<style>
  .shell {
    min-height: 100vh;
    color: var(--ah-console-text);
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
  .app-switcher {
    position: relative;
    margin-left: auto;
  }
  summary {
    display: flex;
    min-height: 44px;
    align-items: center;
    color: var(--ah-console-accent);
    cursor: pointer;
    font-weight: 700;
  }
  nav {
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
  nav a {
    min-height: 44px;
    padding: 0.65rem;
    color: var(--ah-console-accent);
    text-decoration: none;
  }
  nav a:hover {
    background: var(--ah-console-accent-soft);
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
  @media (max-width: 480px) {
    .domain {
      display: none;
    }
  }
</style>
