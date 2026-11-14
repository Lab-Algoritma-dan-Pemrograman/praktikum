<script lang="ts">
  import './tokens.css';
  import { onMount, tick } from 'svelte';
  import {
    adminNavigation,
    canAccessAdminRoute,
    hasPermission,
    scopeContains,
    scopeLabel,
    type Principal,
    type ApplicationLink,
    type Scope
  } from '@algohub/auth-client';
  export let activePath = '/';
  export let principal: Principal;
  export let applications: ApplicationLink[] = [];
  export let loading = false;
  export let logoutPending = false;
  export let onLogout: () => void;
  let drawerOpen = false;
  let ready = false;
  let compact = false;
  let drawer: HTMLElement;
  let menuButton: HTMLButtonElement;
  let selectedScope = '';

  $: roleLabel =
    principal.assignments
      .filter((assignment) => assignment.active)
      .map(
        (assignment) =>
          `${assignment.role}${assignment.division ? ` · ${assignment.division}` : ''}`
      )
      .join(', ') || 'Tanpa penugasan';
  $: scopes = [
    ...new Map(
      principal.grants.map((grant) => {
        const key = JSON.stringify(
          Object.fromEntries(
            Object.entries(grant.scope).sort(([a], [b]) => a.localeCompare(b))
          )
        );
        return [key, grant.scope] as const;
      })
    ).entries()
  ].filter(([, scope]) => !scopeContains(scope, {}));
  $: if (selectedScope && !scopes.some(([key]) => key === selectedScope))
    selectedScope = '';
  $: scope = scopes.find(([key]) => key === selectedScope)?.[1];
  $: visibleItems = adminNavigation.filter(
    (item) =>
      canAccessAdminRoute(principal, item.href) &&
      (!scope ||
        ['/', '/profil', '/pengaturan'].includes(item.href) ||
        item.permissions.some((permission) =>
          hasPermission(
            principal.grants.filter((grant) =>
              scopeContains(grant.scope, scope as Scope)
            ),
            permission
          )
        ))
  );
  $: activeLabel =
    adminNavigation.find((item) => item.href === activePath)?.label ?? 'Admin';

  async function closeDrawer(restoreFocus = true) {
    drawerOpen = false;
    document.body.style.overflow = '';
    if (restoreFocus) {
      await tick();
      menuButton?.focus();
    }
  }

  async function openDrawer() {
    drawerOpen = true;
    document.body.style.overflow = 'hidden';
    await tick();
    drawer.querySelector<HTMLElement>('button, a, select')?.focus();
  }

  function handleKey(event: KeyboardEvent) {
    if (!drawerOpen || !compact) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      void closeDrawer();
    }
    if (event.key === 'Tab') {
      const elements = [
        ...drawer.querySelectorAll<HTMLElement>(
          'a[href], button:not(:disabled), select, summary, [tabindex="0"]'
        )
      ].filter((element) => element.getClientRects().length > 0);
      const first = elements[0];
      const last = elements[elements.length - 1];
      if (
        event.shiftKey &&
        (document.activeElement === first || document.activeElement === drawer)
      ) {
        event.preventDefault();
        last?.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first?.focus();
      }
    }
  }

  onMount(() => {
    ready = true;
    const media = window.matchMedia('(max-width: 767px)');
    const update = () => {
      compact = media.matches;
      if (!compact) {
        drawerOpen = false;
        document.body.style.overflow = '';
      }
    };
    update();
    media.addEventListener('change', update);
    return () => {
      media.removeEventListener('change', update);
      document.body.style.overflow = '';
    };
  });
</script>

<svelte:window onkeydown={handleKey} />
<div class="admin-shell">
  <a class="skip-link" href="#admin-content">Lewati ke konten</a>
  <header class="topbar" inert={compact && drawerOpen}>
    <button
      bind:this={menuButton}
      class="menu-button"
      type="button"
      aria-label="Buka navigasi"
      aria-controls="admin-navigation"
      aria-expanded={drawerOpen}
      disabled={!ready}
      onclick={openDrawer}><span aria-hidden="true">☰</span></button
    >
    <a class="brand" href="/" aria-label="Algohub Admin beranda"
      ><span class="brand-mark" aria-hidden="true">A</span><span
        >Algohub <strong>Admin</strong></span
      ></a
    >
    <div class="user-summary">
      <span class="user-name">{principal.user.name}</span><span
        class="user-role">{roleLabel}</span
      >
    </div>
  </header>
  <div class="workspace">
    <button
      class:open={drawerOpen}
      class="backdrop"
      type="button"
      tabindex="-1"
      aria-label="Tutup navigasi"
      onclick={() => closeDrawer()}
    ></button>
    <aside
      bind:this={drawer}
      id="admin-navigation"
      class:open={drawerOpen}
      class="sidebar"
      role={compact ? 'dialog' : undefined}
      aria-modal={compact && drawerOpen ? true : undefined}
      aria-label="Navigasi admin"
      tabindex="-1"
      inert={compact && !drawerOpen}
    >
      <button class="drawer-close" type="button" onclick={() => closeDrawer()}
        >Tutup navigasi</button
      >
      <details class="app-switcher">
        <summary>Pindah aplikasi</summary>
        <div class="app-links">
          {#each applications as application}<a
              href={`${application.origin}/login`}>{application.label}</a
            >{/each}
        </div>
      </details>
      <label class="scope-label" for="admin-scope">Scope navigasi</label>
      <select
        id="admin-scope"
        bind:value={selectedScope}
        disabled={!ready}
        aria-describedby="scope-help"
        ><option value="">Semua scope yang diberikan</option
        >{#each scopes as [key, scope]}<option value={key}
            >{scopeLabel(scope)}</option
          >{/each}</select
      >
      <p id="scope-help">Pilihan berasal dari grant aktif Anda.</p>
      <nav aria-label="Menu utama">
        {#each visibleItems as item}<a
            class:active={activePath === item.href}
            aria-current={activePath === item.href ? 'page' : undefined}
            href={item.href}
            onclick={() => {
              if (compact) void closeDrawer(false);
            }}>{item.label}</a
          >{/each}
      </nav>
      <button
        class="logout"
        type="button"
        onclick={onLogout}
        disabled={!ready || logoutPending}
        >{logoutPending ? 'Mengakhiri sesi…' : 'Keluar'}</button
      >
    </aside>
    <main
      id="admin-content"
      class="main"
      tabindex="-1"
      inert={compact && drawerOpen}
      aria-busy={loading}
    >
      <nav class="breadcrumb" aria-label="Breadcrumb">
        <a href="/">Admin</a><span aria-hidden="true">/</span><span
          aria-current="page">{activeLabel}</span
        >
      </nav>
      <p class="scope-context">
        {scope
          ? scopeLabel(scope)
          : 'Menampilkan navigasi untuk seluruh scope yang diberikan.'}
      </p>
      {#if loading}<p class="loading" role="status">
          Memeriksa akses dan memuat halaman…
        </p>{/if}
      <slot />
    </main>
  </div>
</div>

<style>
  :global(body) {
    background: #f3f6f7;
    color: #17324d;
  }
  .admin-shell {
    min-height: 100vh;
  }
  .skip-link {
    position: fixed;
    z-index: 30;
    top: -5rem;
    left: 1rem;
    padding: 0.8rem 1rem;
    color: #fff;
    background: #245b5a;
  }
  .skip-link:focus {
    top: 0.5rem;
  }
  .topbar {
    display: flex;
    align-items: center;
    gap: 1rem;
    min-height: 4.5rem;
    padding: 0.75rem 1.25rem;
    border-bottom: 1px solid #d6e1e3;
    background: #fff;
  }
  .brand {
    display: inline-flex;
    align-items: center;
    gap: 0.65rem;
    color: #102a43;
    font-weight: 700;
    text-decoration: none;
  }
  .brand strong {
    color: #326b68;
  }
  .brand-mark {
    display: grid;
    width: 2rem;
    height: 2rem;
    flex-shrink: 0;
    place-items: center;
    border-radius: 0.35rem;
    color: #fff;
    background: #245b5a;
  }
  .user-summary {
    display: grid;
    min-width: 0;
    max-width: 22rem;
    margin-left: auto;
    text-align: right;
    overflow-wrap: anywhere;
  }
  .user-name {
    font-size: 0.9rem;
    font-weight: 700;
  }
  .user-role {
    color: #4d6872;
    font-size: 0.78rem;
  }
  .menu-button,
  .drawer-close {
    display: none;
    min-height: 44px;
    border: 1px solid #b4c6c9;
    border-radius: 0.5rem;
    color: #17324d;
    background: #fff;
    font: inherit;
    cursor: pointer;
  }
  .menu-button {
    min-width: 44px;
    font-size: 1.2rem;
  }
  .workspace {
    display: flex;
    min-height: calc(100vh - 4.5rem);
  }
  .sidebar {
    display: flex;
    width: 16rem;
    flex: 0 0 16rem;
    flex-direction: column;
    gap: 0.8rem;
    padding: 1.25rem;
    border-right: 1px solid #d6e1e3;
    background: #fbfcfc;
  }
  summary {
    display: flex;
    align-items: center;
    min-height: 44px;
    color: #245b5a;
    font-weight: 800;
    cursor: pointer;
  }
  .app-links {
    display: grid;
    padding: 0.35rem;
    border: 1px solid #d6e1e3;
    border-radius: 0.45rem;
    background: #fff;
  }
  .app-links a {
    min-height: 44px;
    padding: 0.65rem;
    color: #245b5a;
    text-decoration: none;
  }
  .app-links a:hover {
    background: #e7f2f0;
  }
  .scope-label {
    color: #4d6872;
    font-size: 0.8rem;
    font-weight: 700;
  }
  select {
    width: 100%;
    min-height: 44px;
    padding: 0 0.5rem;
    border: 1px solid #b4c6c9;
    border-radius: 0.45rem;
    color: #17324d;
    background: #fff;
    font: inherit;
    font-size: 0.85rem;
  }
  #scope-help {
    margin: 0 0 0.5rem;
    color: #4d6872;
    font-size: 0.75rem;
    line-height: 1.5;
  }
  .sidebar nav {
    display: grid;
    gap: 0.25rem;
  }
  .sidebar nav a {
    min-height: 44px;
    padding: 0.72rem 0.8rem;
    border-radius: 0.45rem;
    color: #46616b;
    text-decoration: none;
  }
  .sidebar nav a:hover,
  .sidebar nav a.active {
    color: #174c4b;
    background: #e7f2f0;
  }
  .logout {
    min-height: 44px;
    margin-top: auto;
    border: 1px solid #d2a08c;
    border-radius: 0.45rem;
    color: #8b452e;
    background: #fff;
    font: inherit;
    cursor: pointer;
  }
  .logout:disabled {
    opacity: 0.65;
    cursor: wait;
  }
  .main {
    width: 100%;
    min-width: 0;
    flex: 1;
    padding: clamp(1rem, 3vw, 2.5rem);
  }
  .breadcrumb {
    display: flex;
    flex-wrap: wrap;
    gap: 0.55rem;
    color: #4d6872;
    font-size: 0.85rem;
  }
  .breadcrumb a {
    color: #245b5a;
  }
  .scope-context {
    margin: 0.8rem 0 1.75rem;
    color: #4d6872;
    font-size: 0.8rem;
    overflow-wrap: anywhere;
  }
  .loading {
    padding: 0.8rem 1rem;
    border-left: 3px solid #245b5a;
    color: #245b5a;
    background: #e7f2f0;
  }
  .backdrop {
    display: none;
  }
  @media (max-width: 767px) {
    .topbar {
      gap: 0.6rem;
      padding: 0.65rem 0.75rem;
    }
    .menu-button {
      display: inline-grid;
      flex-shrink: 0;
      place-items: center;
    }
    .brand {
      gap: 0.45rem;
      font-size: 0.9rem;
    }
    .brand-mark {
      display: none;
    }
    .user-summary {
      max-width: 9rem;
    }
    .user-name {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .user-role {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .sidebar {
      position: fixed;
      visibility: hidden;
      z-index: 20;
      top: 0;
      bottom: 0;
      left: -17rem;
      width: min(16rem, calc(100vw - 1rem));
      overflow-y: auto;
      transition: left 0.18s ease;
      box-shadow: 8px 0 24px #1839481f;
    }
    .sidebar.open {
      left: 0;
      visibility: visible;
    }
    .drawer-close {
      display: block;
      flex-shrink: 0;
    }
    .backdrop.open {
      display: block;
      position: fixed;
      z-index: 19;
      inset: 0;
      border: 0;
      background: #102a4366;
    }
    .logout {
      flex-shrink: 0;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .sidebar {
      transition: none;
    }
  }
</style>
