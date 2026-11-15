<script lang="ts">
  import type { PrismCard } from './prism-cards';

  export let items: PrismCard[] = [];
  export let actionLabel = 'Buka';

  const bands = Array.from({ length: 10 }, (_, band) => band);

  function usesLightInk(hex: string): boolean {
    const value = hex.replace('#', '');
    const full =
      value.length === 3
        ? value
            .split('')
            .map((part) => part + part)
            .join('')
        : value;
    const red = parseInt(full.slice(0, 2), 16) / 255;
    const green = parseInt(full.slice(2, 4), 16) / 255;
    const blue = parseInt(full.slice(4, 6), 16) / 255;
    const luminance =
      0.2126 * red ** 2.2 + 0.7152 * green ** 2.2 + 0.0722 * blue ** 2.2;
    return luminance < 0.32;
  }

  function point(event: PointerEvent): void {
    const card = event.currentTarget as HTMLElement;
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const box = card.getBoundingClientRect();
    const x = (event.clientX - box.left) / box.width - 0.5;
    const y = (event.clientY - box.top) / box.height - 0.5;
    card.style.setProperty('--px', x.toFixed(3));
    card.style.setProperty('--py', y.toFixed(3));
    card.style.setProperty('--shift', `${(x * 96).toFixed(1)}px`);
    card.style.setProperty(
      '--mx',
      `${((event.clientX - box.left) / box.width) * 100}%`
    );
    card.style.setProperty(
      '--my',
      `${((event.clientY - box.top) / box.height) * 100}%`
    );
  }

  function reset(event: PointerEvent): void {
    const card = event.currentTarget as HTMLElement;
    card.style.setProperty('--px', '0');
    card.style.setProperty('--py', '0');
    card.style.setProperty('--shift', '0px');
  }
</script>

<div class="prism-cards">
  {#each items as item, position}
    <a
      class="card"
      href={item.href}
      style={`--card:${item.color};--ink:${usesLightInk(item.color) ? '#ffffff' : '#141413'};--stagger:${position * 32}ms`}
      on:pointermove={point}
      on:pointerleave={reset}
    >
      <span class="prism" aria-hidden="true">
        {#each bands as band}
          <span class="band" style={`--band:${band}`}></span>
        {/each}
      </span>
      <span class="shade" aria-hidden="true"></span>

      <span class="head">
        <span class="index">{item.index}</span>
        <span class="mark" aria-hidden="true"></span>
      </span>

      <span class="body">
        <span class="title">{item.title}</span>
        <span class="text">{item.text}</span>
      </span>

      <span class="action">
        {item.action ?? `${actionLabel} ${item.title}`}
        <span aria-hidden="true">↗</span>
      </span>
    </a>
  {/each}
</div>

<style>
  .prism-cards {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 12px;
  }

  .card {
    position: relative;
    isolation: isolate;
    overflow: hidden;
    display: flex;
    min-width: 0;
    min-height: 21rem;
    flex-direction: column;
    gap: 1.5rem;
    padding: clamp(1.4rem, 2.4vw, 2.5rem);
    border-radius: 12px;
    color: var(--ink);
    background: var(--card);
    text-decoration: none;
    box-shadow:
      0 1px 2px rgba(20, 20, 19, 0.06),
      0 14px 32px -18px rgba(20, 20, 19, 0.35);
    transform: perspective(900px) rotateX(calc(var(--py, 0) * -4deg))
      rotateY(calc(var(--px, 0) * 5deg));
    transition:
      transform 0.35s cubic-bezier(0.22, 1, 0.36, 1),
      box-shadow 0.35s ease;
    animation: rise 0.5s cubic-bezier(0.22, 1, 0.36, 1) both;
    animation-delay: var(--stagger);
  }

  .card:hover,
  .card:focus-visible {
    transform: perspective(900px) rotateX(calc(var(--py, 0) * -4deg))
      rotateY(calc(var(--px, 0) * 5deg)) translateY(-4px);
    box-shadow:
      0 2px 4px rgba(20, 20, 19, 0.08),
      0 26px 46px -20px rgba(20, 20, 19, 0.45);
  }

  .card:focus-visible {
    outline: 2px solid var(--ink);
    outline-offset: 3px;
  }

  .head,
  .body,
  .action {
    position: relative;
    z-index: 1;
  }

  .head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .index {
    font-size: 0.78rem;
    font-weight: 600;
    letter-spacing: 0.14em;
    opacity: 0.72;
  }

  .mark {
    width: 1.6rem;
    height: 1.6rem;
    border: 1.5px solid currentColor;
    border-radius: 50%;
    opacity: 0.45;
    transition:
      opacity 0.3s ease,
      border-radius 0.4s cubic-bezier(0.22, 1, 0.36, 1);
  }

  .card:hover .mark,
  .card:focus-visible .mark {
    opacity: 0.8;
    border-radius: 50% 50% 0 50%;
  }

  .body {
    display: grid;
    gap: 0.6rem;
    margin-top: auto;
  }

  .title {
    font-size: clamp(1.25rem, 1.85vw, 1.625rem);
    line-height: 1.04;
    letter-spacing: -0.025em;
    font-weight: 600;
    text-wrap: balance;
  }

  .text {
    font-size: 0.875rem;
    line-height: 1.45;
    opacity: 0.82;
  }

  .action {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    padding-top: 0.9rem;
    border-top: 1px solid color-mix(in srgb, currentColor 22%, transparent);
    font-size: 0.82rem;
    font-weight: 500;
  }

  .prism,
  .shade {
    position: absolute;
    inset: 0;
    pointer-events: none;
  }

  .prism {
    z-index: 0;
    opacity: 0;
    transition: opacity 0.45s ease;
    mix-blend-mode: soft-light;
  }

  .card:hover .prism,
  .card:focus-visible .prism {
    opacity: 0.85;
  }

  .band {
    position: absolute;
    inset: -30% -40%;
    background: linear-gradient(
      102deg,
      transparent 0%,
      hsl(calc(180 + var(--band) * 17) 95% 72%) 42%,
      transparent 74%
    );
    transform: translateX(calc(var(--shift, 0px) * (1 + var(--band) * 0.07)))
      skewX(-8deg);
    transition: transform 0.5s cubic-bezier(0.2, 0.9, 0.2, 1);
    transition-delay: calc(var(--band) * 32ms);
    opacity: 0.5;
  }

  .shade {
    z-index: 0;
    opacity: 0;
    background: radial-gradient(
      15rem circle at var(--mx, 50%) var(--my, 50%),
      rgba(20, 20, 19, 0.1),
      transparent 70%
    );
    transition: opacity 0.35s ease;
  }

  .card:hover .shade,
  .card:focus-visible .shade {
    opacity: 1;
  }

  @keyframes rise {
    from {
      opacity: 0;
      translate: 0 14px;
    }
    to {
      opacity: 1;
      translate: 0 0;
    }
  }

  @media (max-width: 1080px) {
    .prism-cards {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  @media (max-width: 620px) {
    .prism-cards {
      grid-template-columns: minmax(0, 1fr);
    }

    .card {
      min-height: 17rem;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .card {
      animation: none;
      transform: none;
      transition: none;
    }

    .card:hover,
    .card:focus-visible {
      transform: translateY(-2px);
    }

    .prism,
    .shade {
      display: none;
    }

    .band {
      transition: none;
    }
  }
</style>
