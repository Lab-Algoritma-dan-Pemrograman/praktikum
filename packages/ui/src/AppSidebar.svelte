<script lang="ts">
  import ThemeToggle from './ThemeToggle.svelte';

  type NavItem = { href: string; label: string; hint?: string };

  export let appName: string;
  export let navItems: NavItem[] = [];
  export let currentPath = '/';
  export let userName = '';
  export let userRole = '';
  export let initial = '';
  export let logoutHref = '/logout';
  export let loginHref = '/login';
  export let loggedIn = false;
  export let onLogout: (() => void) | undefined = undefined;
  export let logoutPending = false;

  let open = false;

  $: loggedIn = loggedIn || !!userName;
  $: initial =
    initial || (userName ? userName.trim().charAt(0).toUpperCase() : 'A');
</script>

<div class="console">
  <a class="console-skip" href="#content">Lewati ke konten</a>

  <aside
    class="console-sidebar"
    class:is-open={open}
    aria-label="Navigasi aplikasi"
  >
    <div class="console-brandrow">
      <a class="console-brand" href="/" aria-label="Algohub beranda">
        <img
          src="/logo-mark.png"
          alt=""
          width="55"
          height="96"
          decoding="async"
        />
        <span class="console-wordmark"
          ><span class="console-algo">Algo</span><span class="console-hub"
            >Hub</span
          ></span
        >
      </a>
      <ThemeToggle />
    </div>

    <p class="console-app">{appName}</p>

    <nav class="console-nav" aria-label={`Navigasi ${appName}`}>
      {#each navItems as item (item.href)}
        <a
          href={item.href}
          class:is-active={currentPath === item.href}
          aria-current={currentPath === item.href ? 'page' : undefined}
          onclick={() => (open = false)}
        >
          <span>{item.label}</span>
          {#if item.hint}<small>{item.hint}</small>{/if}
        </a>
      {/each}
    </nav>

    {#if loggedIn}
      <div class="console-account">
        <details class="user-menu">
          <summary
            aria-label={userName ? `Menu akun ${userName}` : 'Menu akun'}
          >
            <span class="console-avatar" aria-hidden="true">{initial}</span>
            <span class="console-identity">
              <strong>{userName || 'Tamu'}</strong>
              <small>{userRole || 'Belum masuk'}</small>
            </span>
          </summary>
          <div class="user-panel">
            {#if onLogout}
              <button type="button" onclick={onLogout} disabled={logoutPending}
                >{logoutPending ? 'Mengakhiri sesi…' : 'Keluar'}</button
              >
            {:else}
              <a class="console-logout" href={logoutHref}>Keluar</a>
            {/if}
          </div>
        </details>
      </div>
    {:else}
      <div class="console-account">
        <a class="console-login" href={loginHref}>Masuk</a>
      </div>
    {/if}
  </aside>

  {#if open}
    <button
      class="console-scrim"
      type="button"
      aria-label="Tutup navigasi"
      onclick={() => (open = false)}
    ></button>
  {/if}

  <button
    class="console-toggle"
    type="button"
    aria-expanded={open}
    aria-controls="content"
    onclick={() => (open = !open)}
  >
    <span aria-hidden="true">{open ? '×' : '☰'}</span>
    {open ? 'Tutup' : 'Menu'}
  </button>

  <main id="content" class="console-content" tabindex="-1">
    <slot />
  </main>
</div>

<style>
  .console {
    display: grid;
    grid-template-columns: 16.5rem minmax(0, 1fr);
    min-height: 100vh;
    min-height: 100dvh;
    background: var(--ah-cream);
    color: var(--ah-ink);
  }

  .console-skip {
    position: fixed;
    z-index: 100;
    top: 0.75rem;
    left: 0.75rem;
    padding: 0.7rem 1rem;
    border-radius: 0.75rem;
    color: var(--ah-cream);
    background: var(--ah-ink);
    transform: translateY(-160%);
  }

  .console-skip:focus-visible {
    transform: none;
  }

  .console-sidebar {
    position: sticky;
    top: 0;
    display: flex;
    height: 100vh;
    height: 100dvh;
    flex-direction: column;
    gap: 0.35rem;
    padding: 1.25rem 0.9rem 1rem;
    border-right: 1px solid var(--ah-line);
    background: #ffffff;
    overflow-y: auto;
  }

  .console-brand {
    display: inline-flex;
    min-height: 44px;
    align-items: center;
    gap: 0.5rem;
    padding-inline: 0.35rem;
    text-decoration: none;
  }

  .console-brandrow {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
  }

  .console-brand img {
    height: 1.65rem;
    width: auto;
  }

  .console-wordmark {
    display: inline-flex;
    align-items: baseline;
    font-family: 'Fredoka', ui-rounded, 'Segoe UI', system-ui, sans-serif;
    font-size: 1.2rem;
    font-weight: 500;
    letter-spacing: -0.02em;
    line-height: 1;
  }

  .console-algo {
    color: var(--ah-ink);
  }

  .console-hub {
    color: var(--ah-coral);
  }

  .console-app {
    margin: 0.35rem 0 0.5rem;
    padding-inline: 0.35rem;
    color: var(--ah-coral);
    font-size: 0.75rem;
    font-weight: 700;
    letter-spacing: 0.14em;
    text-transform: uppercase;
  }

  .console-nav {
    display: grid;
    gap: 0.15rem;
  }

  .console-nav a {
    position: relative;
    display: grid;
    min-height: 44px;
    align-content: center;
    gap: 0.05rem;
    padding: 0.45rem 0.65rem 0.45rem 0.85rem;
    border-radius: 0.65rem;
    color: var(--ah-ink);
    font-size: 0.9rem;
    font-weight: 500;
    text-decoration: none;
  }

  .console-nav a small {
    color: var(--ah-muted);
    font-size: 0.75rem;
    font-weight: 400;
  }

  .console-nav a.is-active small {
    color: var(--ah-console-heading);
  }

  .console-nav a:hover {
    background: color-mix(in srgb, var(--ah-coral) 6%, transparent);
  }

  .console-nav a.is-active {
    background: var(--ah-surface);
    font-weight: 600;
  }

  .console-nav a.is-active::before {
    position: absolute;
    top: 50%;
    left: 0;
    width: 3px;
    height: 1.35rem;
    border-radius: 999px;
    background: var(--ah-coral);
    content: '';
    transform: translateY(-50%);
  }

  .console-account {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    margin-top: auto;
    padding: 0.7rem 0.35rem 0.2rem;
    border-top: 1px solid var(--ah-line);
  }

  .console-avatar {
    display: grid;
    width: 2rem;
    height: 2rem;
    flex: 0 0 auto;
    place-items: center;
    border-radius: 999px;
    color: var(--ah-cream);
    background: var(--ah-ink);
    font-size: 0.8rem;
    font-weight: 600;
  }

  .console-identity {
    display: grid;
    min-width: 0;
    flex: 1;
  }

  .console-identity strong {
    overflow: hidden;
    font-size: 0.82rem;
    font-weight: 600;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .console-identity small {
    color: var(--ah-muted);
    font-size: 0.75rem;
  }

  .user-menu {
    position: relative;
    margin-top: 0;
  }

  .user-menu > summary {
    display: flex;
    min-height: 44px;
    align-items: center;
    gap: 0.55rem;
    padding: 0.2rem 0.35rem;
    border-radius: 0.65rem;
    cursor: pointer;
    list-style: none;
  }

  .user-menu > summary::-webkit-details-marker {
    display: none;
  }

  .user-menu > summary:hover {
    background: color-mix(in srgb, var(--ah-coral) 6%, transparent);
  }

  .user-panel {
    position: absolute;
    bottom: calc(100% + 0.35rem);
    left: 0;
    z-index: 30;
    display: grid;
    gap: 0.4rem;
    min-width: 12rem;
    padding: 0.75rem;
    border: 1px solid var(--ah-line);
    border-radius: 0.75rem;
    background: var(--ah-console-card);
    box-shadow: var(--ah-shadow-float);
  }

  .user-panel button {
    display: inline-flex;
    min-height: 40px;
    align-items: center;
    justify-content: center;
    padding: 0.5rem 0.75rem;
    border: 1px solid var(--ah-console-line-strong);
    border-radius: 0.6rem;
    color: var(--ah-console-text);
    background: var(--ah-console-card);
    font: inherit;
    font-weight: 600;
    cursor: pointer;
  }

  .console-login {
    display: inline-flex;
    min-height: 44px;
    flex: 1;
    align-items: center;
    justify-content: center;
    padding: 0.5rem 0.9rem;
    border: 1px solid var(--ah-console-accent);
    border-radius: 0.65rem;
    color: var(--ah-console-card);
    background: var(--ah-console-accent);
    font-size: 0.85rem;
    font-weight: 600;
    text-decoration: none;
  }

  .console-logout {
    display: inline-flex;
    min-height: 40px;
    align-items: center;
    justify-content: center;
    color: var(--ah-muted);
    font-size: 0.8rem;
    font-weight: 600;
    text-decoration: none;
  }

  .console-logout:hover,
  .console-logout:focus-visible {
    color: var(--ah-coral);
  }

  .console-content {
    min-width: 0;
    padding: clamp(1.25rem, 2.4vw, 2.25rem) clamp(1rem, 2.4vw, 2.5rem)
      clamp(2rem, 4vw, 3rem);
  }

  .console-toggle,
  .console-scrim {
    display: none;
  }

  .console-toggle:focus-visible,
  .console-sidebar a:focus-visible {
    outline: var(--ah-focus-brand);
    outline-offset: 2px;
  }

  @media (max-width: 60rem) {
    .console {
      grid-template-columns: minmax(0, 1fr);
    }

    .console-sidebar {
      position: fixed;
      z-index: 60;
      top: 0;
      bottom: 0;
      left: 0;
      width: min(17rem, 82vw);
      border-right: 1px solid var(--ah-line);
      transform: translateX(-102%);
      transition: transform var(--ah-motion-fast, 160ms) ease;
    }

    .console-sidebar.is-open {
      transform: none;
      box-shadow: var(--ah-shadow-drawer);
    }

    .console-scrim {
      position: fixed;
      z-index: 55;
      inset: 0;
      display: block;
      border: 0;
      background: rgb(27 31 42 / 40%);
      cursor: pointer;
    }

    .console-toggle {
      position: fixed;
      z-index: 65;
      bottom: 1rem;
      left: 1rem;
      display: inline-flex;
      min-height: 44px;
      align-items: center;
      gap: 0.4rem;
      padding: 0.55rem 0.9rem;
      border: 0;
      border-radius: 999px;
      color: var(--ah-cream);
      background: var(--ah-ink);
      font: inherit;
      font-size: 0.82rem;
      font-weight: 600;
      box-shadow: var(--ah-shadow-float);
      cursor: pointer;
    }
  }
</style>
