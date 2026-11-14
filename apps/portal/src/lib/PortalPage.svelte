<script lang="ts">
  import { page } from '$app/state';
  import { SiteFooter } from '@algohub/ui';

  let {
    eyebrow,
    title,
    lead,
    docTitle = `${eyebrow} — Algohub`,
    heroPoints = [],
    children
  }: {
    eyebrow: string;
    title: string;
    lead: string;
    docTitle?: string;
    heroPoints?: string[];
    children: import('svelte').Snippet;
  } = $props();

  const links = [
    { href: '/', label: 'Beranda' },
    { href: '/jadwal', label: 'Jadwal' },
    { href: '/berkas', label: 'Berkas' },
    { href: '/asisten', label: 'Asisten' },
    { href: '/laporan', label: 'Laporan' },
    { href: '/modul', label: 'Modul' }
  ];
</script>

<svelte:head>
  <title>{docTitle}</title>
  <meta name="description" content={lead} />
</svelte:head>

<div class="public">
  <a class="skip" href="#content">Lewati ke konten</a>

  <header class="topbar">
    <div class="topbar-inner">
      <a class="brand" href="/" aria-label="Algohub beranda">
        <img
          class="brand-mark"
          src="/logo-mark.png"
          alt=""
          width="55"
          height="96"
          decoding="async"
        />
        <span class="brand-word"
          ><span class="bw-algo">Algo</span><span class="bw-hub">Hub</span
          ></span
        >
      </a>
      <nav aria-label="Navigasi portal">
        {#each links as link (link.href)}
          <a
            href={link.href}
            aria-current={page.url.pathname === link.href ? 'page' : undefined}
            >{link.label}</a
          >
        {/each}
      </nav>
      <a class="login" href="/login">Masuk</a>
    </div>
  </header>

  <main id="content" class="main">
    <header class="hero">
      <div class="hero-copy">
        <p class="eyebrow">{eyebrow}</p>
        <h1>{title}</h1>
        <p class="lead">{lead}</p>
      </div>
      {#if heroPoints.length}
        <aside class="hero-index" aria-label="Ringkasan halaman">
          <p class="hero-index-label">Di halaman ini</p>
          <ol>
            {#each heroPoints as point, index (point)}
              <li>
                <span aria-hidden="true"
                  >{String(index + 1).padStart(2, '0')}</span
                >{point}
              </li>
            {/each}
          </ol>
        </aside>
      {/if}
    </header>
    {@render children()}
  </main>

  <SiteFooter
    links={[
      { href: '/jadwal', label: 'Jadwal' },
      { href: '/asisten', label: 'Asisten' },
      { href: '/bantuan', label: 'Bantuan' },
      { href: '/privacy', label: 'Privasi' }
    ]}
    note="Sistem akademik dan laboratorium."
  />
</div>

<style>
  .public {
    display: flex;
    min-height: 100vh;
    flex-direction: column;
    background: var(--ah-cream);
    color: var(--ah-ink);
    font-size: 1.0125rem;
    line-height: 1.6;
  }

  .skip {
    position: fixed;
    z-index: 50;
    top: 0.75rem;
    left: 0.75rem;
    padding: 0.75rem 1rem;
    border-radius: var(--ah-radius-sm);
    color: var(--ah-cream);
    background: var(--ah-ink);
    transform: translateY(-180%);
  }

  .skip:focus-visible {
    transform: none;
  }

  .topbar {
    padding: 1.1rem 0 0.5rem;
  }

  .topbar-inner,
  .main {
    width: min(var(--ah-content-max), calc(100% - 2.5rem));
    margin: 0 auto;
  }

  .topbar-inner {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.75rem 1.25rem;
    padding-bottom: 1rem;
    border-bottom: 1.5px solid var(--ah-line);
  }

  .brand {
    display: inline-flex;
    min-height: 44px;
    align-items: center;
    gap: 0.6rem;
    font-size: 1.25rem;
    font-weight: 600;
    letter-spacing: -0.03em;
    text-decoration: none;
  }

  .brand-mark {
    display: block;
    width: auto;
    height: 1.7rem;
  }

  .brand-word {
    display: inline-flex;
    align-items: baseline;
    font-family: 'Fredoka', ui-rounded, 'Segoe UI', system-ui, sans-serif;
    font-size: 1.22rem;
    font-weight: 500;
    letter-spacing: -0.02em;
    line-height: 1;
  }

  .bw-algo {
    color: var(--ah-ink);
  }

  .bw-hub {
    color: var(--ah-coral);
  }

  nav {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.15rem;
  }

  nav a {
    display: inline-flex;
    min-height: 2.75rem;
    align-items: center;
    padding: 0 0.7rem;
    border-radius: 999px;
    color: var(--ah-muted);
    font-size: 0.95rem;
    font-weight: 500;
    text-decoration: none;
  }

  nav a:hover,
  nav a:focus-visible {
    color: var(--ah-ink);
    background: color-mix(in srgb, var(--ah-rust) 8%, transparent);
  }

  nav a[aria-current='page'] {
    color: var(--ah-ink);
    box-shadow: inset 0 -2px 0 var(--ah-coral);
  }

  .login {
    display: inline-flex;
    min-height: 2.75rem;
    align-items: center;
    margin-left: auto;
    padding: 0 1rem;
    border-radius: 999px;
    color: var(--ah-cream);
    background: var(--ah-coral);
    font-size: 0.95rem;
    font-weight: 500;
    text-decoration: none;
  }

  .login:hover,
  .login:focus-visible {
    background: var(--ah-rust);
  }

  .main {
    flex: 1;
    padding: clamp(2rem, 5vw, 3.5rem) 0 clamp(3rem, 7vw, 6rem);
  }

  .hero {
    display: grid;
    min-height: min(30rem, 56vh);
    align-content: center;
    gap: 2rem;
    padding-bottom: clamp(1.5rem, 4vw, 3rem);
  }

  @media (min-width: 900px) {
    .hero {
      grid-template-columns: minmax(0, 1.6fr) minmax(15rem, 1fr);
      align-items: end;
    }
  }

  .hero-index {
    padding-top: 1rem;
    border-top: 2px solid var(--ah-coral);
  }

  .hero-index-label {
    margin: 0 0 0.75rem;
    color: var(--ah-muted);
    font-size: 0.75rem;
    font-weight: 600;
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  .hero-index ol {
    display: grid;
    gap: 0.55rem;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .hero-index li {
    display: grid;
    grid-template-columns: 2rem minmax(0, 1fr);
    gap: 0.5rem;
    font-size: 0.95rem;
    line-height: 1.4;
  }

  .hero-index li span {
    color: var(--ah-coral);
    font-size: 0.78rem;
    font-weight: 600;
    letter-spacing: 0.08em;
  }

  @media (prefers-reduced-motion: no-preference) {
    .hero > * {
      animation: rise 0.7s cubic-bezier(0.2, 0.7, 0.2, 1) backwards;
    }

    .hero-copy > .eyebrow {
      animation-delay: 0.05s;
    }

    .hero-copy > h1 {
      animation-delay: 0.12s;
    }

    .hero-copy > .lead {
      animation-delay: 0.2s;
    }
  }

  @keyframes rise {
    from {
      opacity: 0;
      transform: translateY(14px);
    }

    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .eyebrow {
    margin: 0 0 1rem;
    color: var(--ah-coral);
    font-size: 0.8rem;
    font-weight: 600;
    letter-spacing: 0.14em;
    text-transform: uppercase;
  }

  h1 {
    max-width: 24ch;
    margin: 0;
    font-size: clamp(2.2rem, 5vw, 3.6rem);
    font-weight: 500;
    line-height: 1.04;
    letter-spacing: -0.04em;
  }

  .lead {
    max-width: 46ch;
    margin: 1.25rem 0 0;
    color: var(--ah-muted);
    font-size: clamp(1rem, 1.4vw, 1.1rem);
  }

  /* Content primitives shared by the portal sub-pages. */
  .main :global(.sections) {
    display: grid;
    gap: clamp(2.5rem, 5vw, 4rem);
    margin-top: clamp(2.5rem, 5vw, 4rem);
  }

  .main :global(.section) {
    display: grid;
    grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.6fr);
    gap: clamp(1rem, 3vw, 3rem);
    padding-top: clamp(1.5rem, 3vw, 2.25rem);
    border-top: 1.5px solid var(--ah-line);
  }

  .main :global(.section h2) {
    max-width: 18ch;
    margin: 0;
    font-size: clamp(1.35rem, 2.2vw, 1.8rem);
    font-weight: 500;
    line-height: 1.12;
    letter-spacing: -0.03em;
  }

  .main :global(.section-body) {
    display: grid;
    gap: 1.15rem;
    min-width: 0;
  }

  .main :global(.section-body p) {
    max-width: 62ch;
    margin: 0;
    color: var(--ah-muted);
  }

  .main :global(.items) {
    display: grid;
    gap: 0;
    margin: 0;
    padding: 0;
    list-style: none;
    border-top: 1px solid var(--ah-line);
  }

  .main :global(.items > li) {
    display: grid;
    grid-template-columns: 3rem minmax(0, 1fr);
    gap: 0.35rem 1rem;
    padding: 1rem 0.6rem 1rem 0;
    border-bottom: 1px solid var(--ah-line);
    transition:
      transform var(--ah-motion-fast) ease,
      background-color var(--ah-motion-fast) ease;
  }

  @media (hover: hover) {
    .main :global(.items > li:hover) {
      transform: translateY(-2px);
      background: color-mix(in srgb, var(--ah-rust) 4%, transparent);
    }
  }

  .main :global(.items > li > .index) {
    padding-top: 0.15rem;
    color: var(--ah-coral);
    font-size: 0.8rem;
    font-weight: 600;
    letter-spacing: 0.08em;
  }

  .main :global(.items > li > strong) {
    font-weight: 600;
  }

  .main :global(.items > li > p) {
    grid-column: 2;
    max-width: 58ch;
    margin: 0;
    color: var(--ah-muted);
  }

  .main :global(.chips) {
    display: flex;
    flex-wrap: wrap;
    gap: 0.45rem;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .main :global(.chips li) {
    padding: 0.4rem 0.8rem;
    border: 1px solid var(--ah-line-strong);
    border-radius: 999px;
    font-size: 0.85rem;
    font-weight: 500;
  }

  .main :global(.note) {
    display: grid;
    gap: 0.6rem;
    margin: 0;
    padding: 1.25rem 1.4rem;
    border-left: 3px solid var(--ah-coral);
    border-radius: 0 var(--ah-radius-sm) var(--ah-radius-sm) 0;
    background: color-mix(in srgb, var(--ah-rust) 6%, transparent);
  }

  .main :global(.note p) {
    margin: 0;
    color: var(--ah-ink);
  }

  .main :global(.note-strong) {
    font-weight: 600;
  }

  .main :global(.cta) {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 1rem 1.5rem;
    margin-top: clamp(2.5rem, 5vw, 3.5rem);
    padding: clamp(1.5rem, 3vw, 2.25rem);
    border-radius: var(--ah-radius-md);
    color: var(--ah-cream);
    background: var(--ah-rust);
  }

  .main :global(.cta p) {
    max-width: 34ch;
    margin: 0;
  }

  .main :global(.cta-link) {
    display: inline-flex;
    min-height: 2.9rem;
    align-items: center;
    margin-left: auto;
    padding: 0 1.35rem;
    border-radius: 999px;
    color: var(--ah-ink);
    background: var(--ah-cream);
    font-weight: 600;
    text-decoration: none;
  }

  .main :global(.cta-link) {
    transition:
      transform var(--ah-motion-fast) ease,
      background-color var(--ah-motion-fast) ease;
  }

  .main :global(.cta-link:hover),
  .main :global(.cta-link:focus-visible) {
    background: var(--ah-yellow);
  }

  .main :global(.cta-link:active) {
    transform: translateY(1px);
  }

  @media (max-width: 760px) {
    .topbar-inner nav {
      order: 3;
      width: 100%;
      overflow-x: auto;
      padding-bottom: 0.25rem;
      scrollbar-width: thin;
    }

    .login {
      margin-left: auto;
    }

    .main :global(.section) {
      grid-template-columns: minmax(0, 1fr);
    }

    .main :global(.cta-link) {
      margin-left: 0;
    }
  }

  @media (max-width: 520px) {
    .main :global(.items > li) {
      grid-template-columns: minmax(0, 1fr);
    }

    .main :global(.items > li > p) {
      grid-column: 1;
    }
  }
</style>
