<script lang="ts">
  import '../app.css';
  import { AppShell } from '@algohub/ui';
  import { page } from '$app/state';
  import type { AppIdentityData } from '@algohub/auth-client';
  export let data: AppIdentityData;

  const navItems = [
    { href: '/', label: 'Kelas & Jadwal' },
    { href: '/absensi', label: 'Presensi QR' },
    { href: '/penyewaan', label: 'Inventaris & Penyewaan' }
  ];
</script>

<AppShell
  appName="SIAKAD"
  domain="siakad.algohub.web.id"
  description="Jadwal kelas, presensi QR laboratorium, inventaris, dan peminjaman alat Algohub."
  variant="app"
  {...data}
/>
<div class="siakad">
  <nav aria-label="Navigasi SIAKAD">
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
