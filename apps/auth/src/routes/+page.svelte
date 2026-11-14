<script lang="ts">
  import { AppShell } from '@algohub/ui';
  import type { AppIdentityData } from '@algohub/auth-client';
  export let data: AppIdentityData;
</script>

<svelte:head><title>Algohub — Auth Center</title></svelte:head>
<AppShell
  appName="Auth Center"
  domain="auth.algohub.web.id"
  description="Satu identitas untuk seluruh ruang Algohub. Sesi aplikasi dibuat secara terpisah melalui SSO."
  {...data}
/>
