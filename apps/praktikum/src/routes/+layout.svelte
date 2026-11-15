<script lang="ts">
  import '../app.css';
  import { AppSidebar } from '@algohub/ui';
  import { AuthClient, asApiError, type ApiError } from '@algohub/auth-client';
  import { page } from '$app/state';
  import { afterNavigate } from '$app/navigation';
  import type { AppIdentityData } from '@algohub/auth-client';

  export let data: AppIdentityData;

  const navItems = [
    { href: '/', label: 'Sesi praktikum', hint: 'Kelas & aktivasi' },
    { href: '/nilai', label: 'Nilai & riwayat', hint: 'Hasil pengerjaan' }
  ];

  $: loginHref = `/login?${new URLSearchParams({ return_to: page.url.pathname })}`;

  let busy = false;
  let issue: ApiError | null = null;

  $: principal = data.principal;
  $: roleLabel = principal?.assignments?.[0]?.role ?? '';

  async function logout() {
    busy = true;
    issue = null;
    try {
      await new AuthClient().logout();
      location.assign(`${data.authOrigin}/logout`);
    } catch (error) {
      issue = asApiError(error);
    } finally {
      busy = false;
    }
  }

  afterNavigate(({ type, from, to }) => {
    if (type !== 'enter' && from?.url.pathname !== to?.url.pathname)
      document.getElementById('content')?.focus();
  });
</script>

<AppSidebar
  appName="Praktikum"
  currentPath={page.url.pathname}
  {navItems}
  userName={principal?.user.name ?? ''}
  userRole={roleLabel}
  initial={(principal?.user.name ?? 'A').charAt(0).toUpperCase()}
  loggedIn={!!principal}
  {loginHref}
  onLogout={principal ? logout : undefined}
  logoutPending={busy}
>
  <div class="practicum">
    {#if issue}
      <div class="notice error" role="alert">{issue.message}</div>
    {/if}
    <slot />
  </div>
</AppSidebar>
