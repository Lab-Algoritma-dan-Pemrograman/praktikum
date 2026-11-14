<script lang="ts">
  import '../app.css';
  import { AppShell } from '@algohub/ui';
  import type { AppIdentityData } from '@algohub/auth-client';
  export let data: AppIdentityData;
</script>

<div class="identity-shell">
  <AppShell
    appName="E-learning"
    domain="elearning.algohub.web.id"
    description="Materi interaktif, latihan kode, progres XP, dan tantangan Bug Hunt Algohub."
    {...data}
  />
</div>
<div class="elearning">
  <nav aria-label="Navigasi e-learning">
    <a href="/">Kurikulum</a>
    <a href="/playground">Playground</a>
    <a href="/bug-hunt">Bug Hunt</a>
    <a href="/leaderboard">Leaderboard & Achievement</a>
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
