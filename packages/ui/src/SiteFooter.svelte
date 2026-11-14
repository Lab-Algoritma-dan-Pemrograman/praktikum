<script lang="ts">
  type FooterLink = { label: string; href: string };

  export let links: FooterLink[] = [
    { label: 'Portal', href: '/' },
    { label: 'Panduan', href: '/' },
    { label: 'Bantuan', href: '/' }
  ];
  export let year: number = new Date().getFullYear();
  export let note = 'Lab Algoritma & Pemrograman';
  export let brandHref = '/';
</script>

<footer class="site-footer">
  <div class="site-footer-inner">
    <a class="site-footer-brand" href={brandHref} aria-label="Algohub beranda">
      <img
        src="/logo-mark.png"
        alt=""
        width="55"
        height="96"
        decoding="async"
      />
      <span class="site-footer-wordmark"
        ><span class="site-footer-algo">Algo</span
        ><span class="site-footer-hub">Hub</span></span
      >
    </a>

    <p class="site-footer-copy">© {year} {note}</p>

    {#if links.length > 0}
      <nav class="site-footer-links" aria-label="Navigasi footer">
        {#each links as link (link.href + link.label)}
          <a href={link.href}>{link.label}</a>
        {/each}
      </nav>
    {/if}
  </div>
</footer>

<style>
  .site-footer {
    border-top: 1px solid var(--ah-line);
    background: var(--ah-cream);
    color: var(--ah-ink);
  }

  .site-footer-inner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem 2rem;
    width: min(75rem, 100%);
    margin: 0 auto;
    padding: 1.1rem clamp(1.25rem, 3vw, 2.75rem);
  }

  .site-footer-brand {
    display: inline-flex;
    min-height: 44px;
    align-items: center;
    gap: 0.5rem;
    text-decoration: none;
  }

  .site-footer-brand img {
    height: 1.55rem;
    width: auto;
  }

  .site-footer-wordmark {
    display: inline-flex;
    align-items: baseline;
    font-family: 'Fredoka', ui-rounded, 'Segoe UI', system-ui, sans-serif;
    font-size: 1.05rem;
    font-weight: 500;
    letter-spacing: -0.02em;
    line-height: 1;
  }

  .site-footer-algo {
    color: var(--ah-ink);
  }

  .site-footer-hub {
    color: var(--ah-coral);
  }

  .site-footer-copy {
    margin: 0;
    color: var(--ah-muted);
    font-size: 0.8rem;
  }

  .site-footer-links {
    display: flex;
    flex-wrap: wrap;
    gap: 0 1.5rem;
  }

  .site-footer-links a {
    display: inline-flex;
    min-height: 44px;
    align-items: center;
    color: var(--ah-ink);
    font-size: 0.85rem;
    font-weight: 500;
    text-decoration: none;
  }

  .site-footer-links a:hover,
  .site-footer-links a:focus-visible {
    color: var(--ah-coral);
  }

  .site-footer a:focus-visible {
    outline: var(--ah-focus-brand);
    outline-offset: 2px;
  }

  @media (max-width: 60rem) {
    .site-footer-inner {
      flex-direction: column;
      align-items: flex-start;
      gap: 0.4rem;
      padding-block: 1rem;
    }

    .site-footer-links {
      gap: 0 1.25rem;
    }
  }
</style>
