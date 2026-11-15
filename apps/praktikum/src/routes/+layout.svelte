<script lang="ts">
  import '../app.css';
  import { AppShell } from '@algohub/ui';
  import { page } from '$app/state';
  import { afterNavigate } from '$app/navigation';
  import type { AppIdentityData } from '@algohub/auth-client';
  export let data: AppIdentityData;
  const navItems = [
    { href: '/', label: 'Sesi praktikum' },
    { href: '/nilai', label: 'Nilai & riwayat' }
  ];
  afterNavigate(({ type, from, to }) => {
    if (type !== 'enter' && from?.url.pathname !== to?.url.pathname)
      document.getElementById('praktikum-content')?.focus();
  });
</script>

<a class="practicum-skip" href="#praktikum-content">Lewati ke konten</a>
<AppShell
  appName="Praktikum"
  domain={page.url.host}
  description="Sesi, pengerjaan, dan nilai praktikum. Waktu pengerjaan mengikuti server dalam WIB."
  variant="app"
  renderAppHeading={false}
  loginReturnTo={page.url.pathname}
  {...data}
/>
<div class="practicum">
  {#if data.principal}
    <nav class="practicum-nav" aria-label="Navigasi praktikum">
      {#each navItems as item}
        <a
          href={item.href}
          class:section-active={page.url.pathname === item.href ||
            (item.href === '/' &&
              (page.url.pathname.startsWith('/sesi/') ||
                page.url.pathname.startsWith('/attempts/')))}
          aria-current={page.url.pathname === item.href ? 'page' : undefined}
          >{item.label}</a
        >
      {/each}
    </nav>
  {/if}
  <main id="praktikum-content" tabindex="-1"><slot /></main>
</div>
