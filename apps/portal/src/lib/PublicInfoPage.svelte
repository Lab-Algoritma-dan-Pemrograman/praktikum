<script lang="ts">
  export let eyebrow: string;
  export let title: string;
  export let description: string;
  export let state: 'available' | 'blocked' = 'blocked';
  export let detail = '';
  export let actionHref = '/';
  export let actionLabel = 'Kembali ke beranda';
</script>

<svelte:head>
  <title>{title} — Algohub</title>
  <meta name="description" content={description} />
</svelte:head>

<div class="public-page">
  <a class="skip-link" href="#main-content">Lewati ke konten</a>
  <header class="public-header">
    <a class="brand" href="/" aria-label="Algohub beranda">
      <span class="brand-mark" aria-hidden="true"
        ><span></span><span></span><span></span><span></span></span
      >
      <span>Algohub</span>
    </a>
    <nav aria-label="Navigasi publik">
      <a href="/">Beranda</a>
      <a href="/bantuan">Bantuan</a>
      <a href="/login">Masuk</a>
    </nav>
  </header>

  <main id="main-content" class="public-main">
    <p class="eyebrow">{eyebrow}</p>
    <h1>{title}</h1>
    <p class="lead">{description}</p>
    <section
      class:blocked={state === 'blocked'}
      class="status-card"
      aria-labelledby="status-title"
    >
      <span class="status-mark" aria-hidden="true"
        >{state === 'blocked' ? '!' : '✓'}</span
      >
      <div>
        <h2 id="status-title">
          {state === 'blocked'
            ? 'Belum tersedia untuk publik'
            : 'Informasi tersedia'}
        </h2>
        <p>
          {detail ||
            'Konten akan ditampilkan setelah sumber data resmi tersedia.'}
        </p>
      </div>
    </section>
    <a class="button" href={actionHref}>{actionLabel}</a>
  </main>

  <footer class="public-footer">
    <span>© 2026 Algohub. Sistem akademik dan laboratorium.</span>
    <span><a href="/privacy">Privasi</a> · <a href="/terms">Ketentuan</a></span>
  </footer>
</div>

<style>
  .public-page {
    min-height: 100vh;
    color: var(--ah-ink);
    background: var(--ah-cream);
  }
  .skip-link {
    position: fixed;
    z-index: 10;
    top: 0.75rem;
    left: 0.75rem;
    padding: 0.75rem 1rem;
    color: var(--ah-cream);
    background: var(--ah-ink);
    border-radius: var(--ah-radius-sm);
    transform: translateY(-160%);
  }
  .skip-link:focus-visible {
    transform: none;
  }
  .public-header {
    display: flex;
    width: min(100% - 2rem, var(--ah-content-max));
    min-height: 4.5rem;
    align-items: center;
    gap: 1rem;
    margin: 0 auto;
    border-bottom: 1px solid var(--ah-line);
  }
  .brand {
    display: inline-flex;
    align-items: center;
    gap: 0.7rem;
    color: inherit;
    font-weight: 800;
    text-decoration: none;
  }
  .brand-mark {
    display: grid;
    width: 2.4rem;
    height: 2.4rem;
    grid-template-columns: repeat(2, 1fr);
    gap: 0.18rem;
    padding: 0.35rem;
    border-radius: 1rem 1rem 0.45rem 0.45rem;
    background: var(--ah-ink);
  }
  .brand-mark span {
    border-radius: 50%;
    background: var(--ah-cream);
  }
  .brand-mark span:nth-child(2) {
    border-radius: 50% 50% 0 50%;
    background: var(--ah-coral);
  }
  .brand-mark span:nth-child(3) {
    border-radius: 50% 0 50% 50%;
    background: var(--ah-blue);
  }
  nav {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    margin-left: auto;
  }
  nav a {
    display: inline-flex;
    min-height: 44px;
    align-items: center;
    padding: 0 0.9rem;
    border-radius: 999px;
    color: var(--ah-muted);
    font-size: 0.9rem;
    font-weight: 700;
    text-decoration: none;
  }
  nav a:hover,
  nav a:focus-visible {
    color: var(--ah-ink);
    background: var(--ah-surface);
  }
  .public-main {
    width: min(100% - 2rem, 52rem);
    margin: 0 auto;
    padding: clamp(5rem, 12vw, 10rem) 0;
  }
  .eyebrow {
    margin: 0 0 1rem;
    color: var(--ah-green-ink);
    font-size: 0.78rem;
    font-weight: 850;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }
  h1 {
    max-width: 14ch;
    margin: 0;
    font-size: clamp(3rem, 8vw, 6rem);
    line-height: 0.96;
    letter-spacing: -0.06em;
  }
  .lead {
    max-width: 42rem;
    margin: 1.5rem 0 0;
    color: var(--ah-muted);
    font-size: clamp(1.05rem, 2vw, 1.25rem);
    line-height: 1.65;
  }
  .status-card {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 1rem;
    margin: 2.5rem 0;
    padding: clamp(1.25rem, 4vw, 2rem);
    border: 1.5px solid var(--ah-ink);
    border-radius: var(--ah-radius-lg);
    background: var(--ah-green);
  }
  .status-card.blocked {
    background: var(--ah-surface);
  }
  .status-mark {
    display: grid;
    width: 2.8rem;
    height: 2.8rem;
    place-items: center;
    border-radius: 50%;
    color: var(--ah-cream);
    background: var(--ah-ink);
    font-weight: 900;
  }
  h2 {
    margin: 0;
    font-size: clamp(1.35rem, 3vw, 2rem);
    letter-spacing: -0.035em;
  }
  .status-card p {
    margin: 0.5rem 0 0;
    color: var(--ah-muted);
    line-height: 1.6;
  }
  .button {
    display: inline-flex;
    min-height: 52px;
    align-items: center;
    justify-content: center;
    padding: 0 1.4rem;
    border: 1.5px solid var(--ah-ink);
    border-radius: 1.6rem 1.6rem 0.65rem 0.65rem;
    color: var(--ah-ink);
    background: var(--ah-coral);
    font-weight: 800;
    text-decoration: none;
  }
  .button:hover,
  .button:focus-visible {
    border-radius: 0.65rem 0.65rem 1.6rem 1.6rem;
  }
  .public-footer {
    display: flex;
    width: min(100% - 2rem, var(--ah-content-max));
    justify-content: space-between;
    gap: 1rem;
    margin: 0 auto;
    padding: 1.5rem 0 2rem;
    border-top: 1px solid var(--ah-line);
    color: var(--ah-muted);
    font-size: 0.82rem;
  }
  .public-footer a {
    color: inherit;
  }
  @media (max-width: 560px) {
    .public-header {
      width: min(100% - 1.25rem, var(--ah-content-max));
    }
    .public-header nav {
      gap: 0;
    }
    .public-header nav a {
      padding-inline: 0.35rem;
      font-size: 0.78rem;
    }
    .public-main {
      width: min(100% - 1.25rem, 52rem);
    }
    .public-footer {
      width: min(100% - 1.25rem, var(--ah-content-max));
      flex-direction: column;
    }
  }
  @media (max-width: 360px) {
    .public-header {
      gap: 0.35rem;
    }
    .public-header .brand {
      gap: 0.35rem;
      font-size: 0.9rem;
    }
    .public-header .brand-mark {
      width: 2rem;
      height: 2rem;
    }
    .public-header nav a {
      padding-inline: 0.2rem;
      font-size: 0.72rem;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .button {
      transition: none;
    }
  }
</style>
