<script lang="ts">
  import '../app.css';
  import '@algohub/practicum-client/styles.css';
  import { AppShell } from '@algohub/ui';
  import type { AppIdentityData } from '@algohub/auth-client';
  export let data: AppIdentityData;
</script>

<div class="identity-shell">
  <AppShell
    appName="Praktikum"
    domain="praktikum.algohub.web.id"
    description="Sesi, pengerjaan, dan nilai praktikum. Waktu pengerjaan mengikuti server dalam WIB."
    {...data}
  />
</div>
<div class="practicum">
  <nav aria-label="Navigasi praktikum">
    <a href="/">Daftar sesi</a><a href="/nilai">Nilai saya</a>
  </nav>
  <slot />
</div>

<style>
  .identity-shell :global(.shell) {
    min-height: auto;
  }
  .identity-shell :global(.content) {
    width: min(73.5rem, 92vw);
    padding: 1.5rem 0;
  }
  .identity-shell :global(h1) {
    font-size: 2rem;
    max-width: none;
  }
  .identity-shell :global(.identity) {
    margin-top: 1rem;
    padding: 0.8rem;
  }
  .identity-shell :global(.note),
  .identity-shell :global(.eyebrow) {
    display: none;
  }
</style>
