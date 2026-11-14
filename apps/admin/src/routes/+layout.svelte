<script lang="ts">
  import { onMount } from 'svelte';
  import { page, navigating } from '$app/state';
  import { invalidateAll } from '$app/navigation';
  import {
    AuthClient,
    asApiError,
    type ApiError,
    type AppIdentityData,
    type Principal
  } from '@algohub/auth-client';
  import { AdminShell, RequestError } from '@algohub/ui';
  import type { Snippet } from 'svelte';
  import '../app.css';

  let {
    data,
    children
  }: { data: AppIdentityData & { principal: Principal }; children: Snippet } =
    $props();
  let logoutPending = $state(false);
  let issue = $state<ApiError | null>(null);
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
  onMount(() => {
    const checkAccess = () => {
      void invalidateAll();
    };
    window.addEventListener('focus', checkAccess);
    return () => window.removeEventListener('focus', checkAccess);
  });
</script>

<svelte:head><title>Algohub Admin</title></svelte:head>
<AdminShell
  activePath={page.url.pathname}
  principal={data.principal}
  applications={data.applications}
  loading={!!navigating.to}
  {logoutPending}
  onLogout={logout}
>
  <RequestError error={issue} returnTo={page.url.pathname} />
  {@render children()}
</AdminShell>
