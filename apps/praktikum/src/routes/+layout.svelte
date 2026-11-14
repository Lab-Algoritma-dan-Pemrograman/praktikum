<script lang="ts">
  import '../app.css';
  import { AppShell } from '@algohub/ui';
  import { page } from '$app/state';
  import type { AppIdentityData } from '@algohub/auth-client';
  export let data: AppIdentityData;

  const navItems = [
    { href: '/', label: 'Daftar sesi' },
    { href: '/nilai', label: 'Nilai saya' }
  ];
</script>

<AppShell
  appName="Praktikum"
  domain="praktikum.algohub.web.id"
  description="Sesi, pengerjaan, dan nilai praktikum. Waktu pengerjaan mengikuti server dalam WIB."
  variant="app"
  {...data}
/>
<div class="practicum">
  <nav aria-label="Navigasi praktikum">
    {#each navItems as item}
      <a
        href={item.href}
        aria-current={page.url.pathname === item.href ? 'page' : undefined}
        >{item.label}</a
      >
    {/each}
  </nav>
  <slot />
</div>
