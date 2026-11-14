<script lang="ts">
  import '../app.css';
  import { AppSidebar } from '@algohub/ui';
  import { page } from '$app/state';
  import type { AppIdentityData } from '@algohub/auth-client';
  export let data: AppIdentityData;

  const navItems = [
    { href: '/', label: 'Ringkasan', hint: 'Kelas & jadwal' },
    { href: '/absensi', label: 'Presensi QR', hint: 'Isi kehadiran' },
    { href: '/penyewaan', label: 'Inventaris & sewa', hint: 'Peminjaman alat' }
  ];

  $: principal = data.principal;
  $: roleLabel = principal?.assignments?.[0]?.role ?? '';
</script>

<AppSidebar
  appName="Siakad"
  currentPath={page.url.pathname}
  {navItems}
  userName={principal?.user.name ?? ''}
  userRole={roleLabel}
  initial={(principal?.user.name ?? 'A').charAt(0).toUpperCase()}
>
  <slot />
</AppSidebar>
