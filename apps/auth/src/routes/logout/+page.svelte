<svelte:head
  ><title>Sesi diakhiri — Algohub</title><meta
    name="robots"
    content="noindex"
  /></svelte:head
>
<main>
  <p class="eyebrow">AUTH CENTER</p>
  <h1>Sesi telah diakhiri.</h1>
  <p>
    Logout dari aplikasi mengakhiri seluruh sesi aplikasi dalam login SSO yang
    sama. Anda dapat masuk kembali untuk memulai sesi baru.
  </p>
  <p class="note">Membuka halaman ini saja tidak melakukan logout.</p>
  <a href="/login">Masuk kembali</a>
</main>

<style>
  main {
    width: min(38rem, 90vw);
    margin: 0 auto;
    padding: 5rem 0;
    color: var(--ah-console-text);
  }
  .eyebrow {
    color: var(--ah-console-accent-ink);
    font-size: 0.75rem;
    font-weight: 800;
    letter-spacing: 0.1em;
  }
  h1 {
    color: var(--ah-console-heading);
    font-size: clamp(2rem, 6vw, 3.5rem);
    letter-spacing: -0.04em;
  }
  p {
    line-height: 1.65;
  }
  .note {
    color: var(--ah-console-muted);
    font-size: 0.85rem;
  }
  a {
    display: inline-flex;
    min-height: 44px;
    align-items: center;
    padding: 0.7rem 1rem;
    border-radius: var(--ah-console-radius-control);
    color: var(--ah-console-card);
    background: var(--ah-console-accent);
    text-decoration: none;
  }
</style>
