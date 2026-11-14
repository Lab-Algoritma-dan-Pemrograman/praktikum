<script lang="ts">
  import '../app.css';
  import { AppShell } from '@algohub/ui';
  import { page } from '$app/state';
  import type { AppIdentityData } from '@algohub/auth-client';
  export let data: AppIdentityData;

  const navItems = [
    { href: '/', label: 'Kurikulum' },
    { href: '/playground', label: 'Playground' },
    { href: '/bug-hunt', label: 'Bug Hunt' },
    { href: '/leaderboard', label: 'Leaderboard & Achievement' }
  ];
</script>

<AppShell
  appName="E-learning"
  domain="elearning.algohub.web.id"
  description="Materi interaktif, latihan kode, progres XP, dan tantangan Bug Hunt Algohub."
  variant="app"
  {...data}
/>
<div class="elearning">
  <nav aria-label="Navigasi e-learning">
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
