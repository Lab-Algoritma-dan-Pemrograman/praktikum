<svelte:head><title>SSO Callback — Algohub</title></svelte:head>
<main class="callback">
  <p class="eyebrow">SSO CALLBACK</p>
  <h1>Memproses sesi…</h1>
  <p>
    Callback hanya membawa one-time code. Session cookie dibuat oleh aplikasi
    tujuan.
  </p>
</main>

<style>
  .callback {
    width: min(42rem, 90vw);
    margin: 0 auto;
    padding: 8rem 0;
  }
  .eyebrow {
    color: #3b7b78;
    font-size: 0.72rem;
    font-weight: 800;
    letter-spacing: 0.12em;
  }
  h1 {
    color: #102a43;
    font-size: clamp(2rem, 6vw, 4rem);
    letter-spacing: -0.05em;
  }
  p {
    color: #58717a;
    line-height: 1.65;
  }
</style>
