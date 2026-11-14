<script lang="ts">
  import { onMount } from 'svelte';

  export let signedIn = false;
  export let mode: 'landing' | 'page' = 'page';
  export let animateBrand = false;
  export let currentPath = '';
  export let loginHref = '/login';
  export let workspaceHref = '/workspace';

  type Item = { href: string; label: string };

  const items: Item[] = [
    { href: '/', label: 'Beranda' },
    { href: '/jadwal', label: 'Jadwal' },
    { href: '/berkas', label: 'Berkas' },
    { href: '/asisten', label: 'Asisten' }
  ];

  let navEl: HTMLAnchorElement extends never ? never : HTMLElement;
  let railEl: HTMLElement;
  let wmEl: HTMLElement;
  let activeHref = '#services';

  $: isCurrent = (href: string) =>
    currentPath !== '' && href === currentPath ? 'page' : undefined;

  onMount(() => {
    const nav = navEl;
    const rail = railEl;
    if (!nav || !rail) return;

    const links = Array.from(nav.querySelectorAll<HTMLAnchorElement>('a'));
    const anchors = links.filter((link) =>
      (link.getAttribute('href') ?? '').startsWith('#')
    );
    const currentLink = links.find(
      (link) => link.getAttribute('href') === currentPath
    );
    let hovered: HTMLAnchorElement | null = null;

    const resting =
      links.find((link) => link.getAttribute('href') === activeHref) ??
      currentLink ??
      links[0];
    const target = () => hovered ?? resting;

    const place = () => {
      const link = target();
      if (!link) return;
      const navBox = nav.getBoundingClientRect();
      const box = link.getBoundingClientRect();
      rail.style.width = `${box.width}px`;
      rail.style.transform = `translateX(${box.left - navBox.left}px)`;
      rail.style.opacity = '1';
    };

    const sync = () => {
      for (const link of anchors) {
        if (link.getAttribute('href') === activeHref)
          link.setAttribute('aria-current', 'true');
        else link.removeAttribute('aria-current');
      }
      place();
    };

    const onEnter = (event: Event) => {
      hovered = event.currentTarget as HTMLAnchorElement;
      place();
    };
    const onLeave = () => {
      hovered = null;
      place();
    };

    for (const link of links) {
      link.addEventListener('pointerenter', onEnter);
      link.addEventListener('focus', onEnter);
      link.addEventListener('pointerleave', onLeave);
      link.addEventListener('blur', onLeave);
    }

    const sections = anchors
      .map((link) => document.querySelector(link.getAttribute('href') as string))
      .filter((section): section is Element => Boolean(section));
    const visible = new Map<Element, boolean>();
    const observer = sections.length
      ? new IntersectionObserver(
          (entries) => {
            for (const entry of entries)
              visible.set(entry.target, entry.isIntersecting);
            const first = sections.find((section) => visible.get(section));
            if (first?.id) activeHref = `#${first.id}`;
            sync();
          },
          { rootMargin: '-45% 0px -45% 0px' }
        )
      : null;
    for (const section of sections) observer?.observe(section);

    const onResize = () => place();
    window.addEventListener('resize', onResize);
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      rail.style.transition = 'none';
    }
    requestAnimationFrame(sync);

    return () => {
      for (const link of links) {
        link.removeEventListener('pointerenter', onEnter);
        link.removeEventListener('focus', onEnter);
        link.removeEventListener('pointerleave', onLeave);
        link.removeEventListener('blur', onLeave);
      }
      observer?.disconnect();
      window.removeEventListener('resize', onResize);
    };
  });

  // Brand intro: "<>" opens into "<AlgoHub>", then the brackets leave and the
  // two-tone wordmark stays. Web Animations API, skipped for reduced motion.
  onMount(() => {
    const wm = wmEl;
    if (!animateBrand || !wm || typeof wm.animate !== 'function') return;
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const left = wm.querySelector<HTMLElement>('.wm-br-l');
    const right = wm.querySelector<HTMLElement>('.wm-br-r');
    const text = wm.querySelector<HTMLElement>('.wm-text');
    if (!left || !right || !text) return;

    const box = wm.getBoundingClientRect();
    const centre = box.left + box.width / 2;
    const lBox = left.getBoundingClientRect();
    const rBox = right.getBoundingClientRect();
    const gap = 5;
    const dxL = centre - (lBox.left + lBox.width / 2) - gap;
    const dxR = centre - (rBox.left + rBox.width / 2) + gap;
    const ease = 'cubic-bezier(0.22, 1, 0.36, 1)';
    const timing: KeyframeAnimationOptions = { duration: 1600, fill: 'both' };
    const frame = (x: number, opacity: number, offset: number, move: boolean) => ({
      transform: `translateX(${x}px)`,
      opacity,
      offset,
      easing: move ? ease : 'linear'
    });

    const runs = [
      left.animate(
        [
          frame(dxL, 0, 0, false),
          frame(dxL, 1, 0.1, true),
          frame(dxL, 1, 0.24, true),
          frame(0, 1, 0.58, true),
          frame(0, 1, 0.76, false),
          frame(-3, 0, 1, true)
        ],
        timing
      ),
      right.animate(
        [
          frame(dxR, 0, 0, false),
          frame(dxR, 1, 0.1, true),
          frame(dxR, 1, 0.24, true),
          frame(0, 1, 0.58, true),
          frame(0, 1, 0.76, false),
          frame(3, 0, 1, true)
        ],
        timing
      ),
      text.animate(
        [
          { opacity: 0, transform: 'translateY(3px)', offset: 0, easing: 'linear' },
          { opacity: 0, transform: 'translateY(3px)', offset: 0.28, easing: ease },
          { opacity: 1, transform: 'translateY(0)', offset: 0.6, easing: 'linear' },
          { opacity: 1, transform: 'translateY(0)', offset: 1, easing: 'linear' }
        ],
        timing
      )
    ];

    void Promise.all(runs.map((run) => run.finished.catch(() => undefined))).then(
      () => {
        for (const run of runs) run.cancel();
      }
    );
  });
</script>

<header class="site-header">
  <div class="nav-shell">
    <a class="brand" href="/" aria-label="Algohub beranda">
      <img
        class="brand-mark"
        src="/logo-mark.png"
        alt=""
        width="55"
        height="96"
        decoding="async"
      />
      <span class="brand-word wm" bind:this={wmEl}
        ><span class="wm-br wm-br-l" aria-hidden="true">&lt;</span><span
          class="wm-text"
          ><span class="wm-algo">Algo</span><span class="wm-hub">Hub</span></span
        ><span class="wm-br wm-br-r" aria-hidden="true">&gt;</span></span
      >
    </a>

    <nav class="desktop-nav" aria-label="Navigasi utama" bind:this={navEl}>
      <span class="nav-indicator" aria-hidden="true" bind:this={railEl}></span>
      {#each items as item (item.href)}
        <a
          href={item.href}
          aria-current={isCurrent(item.href)}
          >{item.label}</a
        >
      {/each}
    </nav>

    <div class="header-actions">
      {#if !signedIn}
        <a class="text-link" href={loginHref}>Masuk</a>
      {/if}
      <a class="button button-small" href={signedIn ? workspaceHref : loginHref}>
        <span class="button-label"
          >{signedIn ? 'Buka ruang kerja' : mode === 'landing' ? 'Mulai sekarang' : 'Masuk'}</span
        >
      </a>
    </div>

    <details class="mobile-menu">
      <summary>Menu</summary>
      <nav aria-label="Navigasi mobile">
        {#each items as item (item.href)}
          <a href={item.href} aria-current={isCurrent(item.href)}>{item.label}</a>
        {/each}
        <a href={signedIn ? workspaceHref : loginHref}>
          {signedIn ? 'Buka ruang kerja' : 'Masuk ke Algohub'}
        </a>
      </nav>
    </details>
  </div>
</header>

<style>
  .site-header {
    position: fixed;
    inset: 0 0 auto;
    z-index: 40;
    border-bottom: 1px solid var(--ah-line);
    background: var(--ah-cream);
  }

  .nav-shell {
    position: relative;
    display: flex;
    width: var(--wrap, min(var(--ah-content-max), calc(100% - 2.5rem)));
    min-height: 4.25rem;
    align-items: center;
    gap: 0.5rem;
    margin: 0 auto;
    padding: 0.5rem 0;
  }

  .brand {
    display: inline-flex;
    min-height: 44px;
    align-items: center;
    gap: 0.65rem;
    padding: 0.15rem 0.9rem 0.15rem 0;
    font-size: 1.4rem;
    font-weight: 500;
    letter-spacing: -0.04em;
    text-decoration: none;
    white-space: nowrap;
  }

  .brand-word {
    padding-top: 0.15rem;
    font-family: 'Fredoka', ui-rounded, 'Segoe UI', system-ui, sans-serif;
    font-size: 1.4rem;
    font-weight: 500;
    letter-spacing: -0.02em;
  }

  .wm-algo {
    color: var(--ah-ink);
  }

  .wm-hub {
    color: var(--ah-coral);
  }

  .wm {
    position: relative;
    display: inline-flex;
    align-items: baseline;
    padding-inline: 0.42em;
  }

  .wm-br {
    position: absolute;
    top: 0.15rem;
    color: var(--ah-coral);
    opacity: 0;
    pointer-events: none;
    will-change: transform, opacity;
  }

  .wm-br-l {
    left: 0.04em;
  }

  .wm-br-r {
    right: 0.04em;
  }

  .brand-mark {
    width: auto;
    height: 2.6rem;
    object-fit: contain;
  }

  .desktop-nav {
    position: relative;
    display: flex;
    align-items: center;
    gap: 0.1rem;
    margin-left: auto;
    isolation: isolate;
  }

  .nav-indicator {
    position: absolute;
    top: 0;
    left: 0;
    z-index: 0;
    height: 2.75rem;
    border-radius: 999px;
    background: var(--ah-cream);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.9),
      0 8px 18px -12px rgba(45, 49, 66, 0.5);
    opacity: 0;
    pointer-events: none;
    transition:
      transform 0.38s var(--ah-ease-out),
      width 0.38s var(--ah-ease-out),
      opacity 0.2s ease;
    will-change: transform, width;
  }

  .desktop-nav a,
  .text-link {
    position: relative;
    z-index: 1;
    display: inline-flex;
    min-height: 2.75rem;
    align-items: center;
    padding: 0 0.95rem;
    border-radius: 999px;
    color: var(--ah-muted);
    font-size: 0.95rem;
    font-weight: 500;
    text-decoration: none;
    white-space: nowrap;
    transition: color 0.3s ease;
  }

  .desktop-nav a:hover,
  .desktop-nav a:focus-visible,
  .desktop-nav a[aria-current='true'],
  .desktop-nav a[aria-current='page'] {
    color: var(--ah-ink);
  }

  .text-link:hover,
  .text-link:focus-visible {
    color: var(--ah-ink);
  }

  .header-actions {
    display: flex;
    align-items: center;
    gap: 0.15rem;
  }

  .button {
    position: relative;
    isolation: isolate;
    overflow: hidden;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border: 0;
    color: var(--ah-cream);
    background: var(--ah-coral);
    font-weight: 500;
    line-height: 1;
    letter-spacing: -0.005em;
    text-decoration: none;
    cursor: pointer;
    transition: color 0.3s ease;
    border-radius: 1.625rem 1.625rem 0.625rem 0.625rem;
  }

  .button::before {
    content: '';
    position: absolute;
    inset: -2px;
    z-index: -1;
    border-radius: 50% 50% 0 0 / 70% 70% 0 0;
    background: var(--ah-ink);
    transform: translateY(102%);
    transition:
      transform 0.5s cubic-bezier(0.3, 0.7, 0.2, 1),
      border-radius 0.5s cubic-bezier(0.3, 0.7, 0.2, 1);
  }

  .button-label {
    position: relative;
  }

  .button-small {
    min-height: 2.75rem;
    padding: 0 1.15rem;
    font-size: 0.9rem;
  }

  @media (hover: hover) {
    .button:hover::before {
      transform: translateY(0);
      border-radius: 0;
    }
  }

  .button:focus-visible::before {
    transform: translateY(0);
    border-radius: 0;
  }

  .mobile-menu {
    display: none;
    margin-left: auto;
  }

  .mobile-menu summary {
    display: inline-flex;
    min-height: 2.75rem;
    align-items: center;
    padding: 0 1.15rem;
    border-radius: 999px;
    background: var(--ah-cream);
    cursor: pointer;
    font-weight: 500;
    list-style: none;
  }

  .mobile-menu summary::-webkit-details-marker {
    display: none;
  }

  .mobile-menu[open] summary {
    color: var(--ah-cream);
    background: var(--ah-ink);
  }

  .mobile-menu nav {
    position: absolute;
    top: calc(100% + 0.5rem);
    right: 0;
    left: 0;
    display: grid;
    gap: 0.15rem;
    padding: 0.75rem;
    border-radius: 2rem;
    color: var(--ah-cream);
    background: var(--ah-coral);
  }

  .mobile-menu nav a {
    display: flex;
    min-height: 3.25rem;
    align-items: center;
    padding: 0.5rem 0.75rem;
    border-bottom: 1.5px solid
      color-mix(in srgb, var(--ah-cream) 24%, transparent);
    font-size: clamp(1.35rem, 6vw, 1.9rem);
    font-weight: 500;
    letter-spacing: -0.03em;
    text-decoration: none;
  }

  .mobile-menu nav a:last-child {
    border-bottom: 0;
  }

  .mobile-menu nav a[aria-current='page'] {
    text-decoration: underline;
    text-underline-offset: 0.25rem;
  }

  @media (max-width: 64rem) {
    .brand-mark {
      height: 2.2rem;
    }

    .desktop-nav a {
      padding: 0 0.55rem;
      font-size: 0.88rem;
    }

    .text-link {
      display: none;
    }

    .button-small {
      padding: 0 0.9rem;
      font-size: 0.85rem;
    }
  }

  @media (max-width: 60rem) {
    .desktop-nav,
    .header-actions {
      display: none;
    }

    .mobile-menu {
      display: block;
    }

    .nav-shell {
      min-height: 3.9rem;
    }
  }

  @media (max-width: 30rem) {
    .brand-mark {
      height: 1.9rem;
    }

    .brand-word {
      font-size: 1.25rem;
    }
  }
</style>
