<script lang="ts">
  import '../app.css';
  import '@algohub/siakad-client/styles.css';
  import { AppShell } from '@algohub/ui';
  import type { AppIdentityData } from '@algohub/auth-client';
  export let data: AppIdentityData;
</script>

<div class="identity-shell">
  <AppShell
    appName="SIAKAD"
    domain="siakad.algohub.web.id"
    description="Jadwal kelas, presensi QR laboratorium, inventaris, dan peminjaman alat Algohub."
    {...data}
  />
</div>
<div class="siakad">
  <nav aria-label="Navigasi SIAKAD">
    <a href="/">Kelas & Jadwal</a>
    <a href="/absensi">Presensi QR</a>
    <a href="/penyewaan">Inventaris & Penyewaan</a>
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
