<script lang="ts">
  import '../app.css';
  import { AppSidebar, RequestError } from '@algohub/ui';
  import { page } from '$app/state';
  import {
    AuthClient,
    asApiError,
    type ApiError,
    type AppIdentityData
  } from '@algohub/auth-client';
  export let data: AppIdentityData;

  const navItems = [
    { href: '/', label: 'Ringkasan', hint: 'Kelas & jadwal' },
    { href: '/absensi', label: 'Presensi QR', hint: 'Isi kehadiran' },
    { href: '/penyewaan', label: 'Inventaris & sewa', hint: 'Peminjaman alat' }
  ];

  let logoutPending = false;
  let issue: ApiError | null = null;

  // Cabut sesi SSO lewat API auth, lalu tampilkan halaman konfirmasi keluar.
  async function logout() {
    logoutPending = true;
    issue = null;
    try {
      await new AuthClient().logout();
      location.assign(`${data.authOrigin}/logout`);
    } catch (error) {
      issue = asApiError(error);
    } finally {
      logoutPending = false;
    }
  }

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
  onLogout={principal ? logout : undefined}
  {logoutPending}
>
  <div class="siakad">
    <RequestError error={issue} returnTo={page.url.pathname} />
    <slot />
  </div>
</AppSidebar>

<style>
  /* Gutter sudah disediakan .console-content milik AppSidebar. */
  .siakad {
    padding: 0;
  }
</style>
