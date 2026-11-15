<script lang="ts">
  import { onMount } from 'svelte';

  const STORAGE_KEY = 'algohub-color-mode';
  type Mode = 'light' | 'dark';

  let mode: Mode = 'light';

  function apply(next: Mode) {
    document.documentElement.dataset.theme = next;
    document.documentElement.style.colorScheme = next;
  }

  onMount(() => {
    let next: Mode = 'light';
    try {
      const saved = localStorage.getItem(STORAGE_KEY);
      if (saved === 'light' || saved === 'dark') next = saved;
      else if (window.matchMedia('(prefers-color-scheme: dark)').matches)
        next = 'dark';
    } catch {
      /* storage unavailable */
    }
    mode = next;
    apply(next);
  });

  function toggle() {
    mode = mode === 'dark' ? 'light' : 'dark';
    apply(mode);
    try {
      localStorage.setItem(STORAGE_KEY, mode);
    } catch {
      /* storage unavailable */
    }
  }
</script>

<button
  type="button"
  class="theme-toggle"
  onclick={toggle}
  aria-pressed={mode === 'dark'}
  aria-label={mode === 'dark' ? 'Aktifkan mode terang' : 'Aktifkan mode gelap'}
  title={mode === 'dark' ? 'Mode terang' : 'Mode gelap'}
>
  <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
    {#if mode === 'dark'}
      <path
        fill="currentColor"
        d="M12 3a9 9 0 1 0 9 9c0-.46-.04-.92-.1-1.36a5.39 5.39 0 0 1-4.4 2.26 5.4 5.4 0 0 1-3.14-9.8A9.9 9.9 0 0 0 12 3Z"
      />
    {:else}
      <path
        fill="currentColor"
        d="M12 7a5 5 0 1 0 0 10 5 5 0 0 0 0-10Zm0-2.5a1 1 0 0 1 1 1V7h-2V5.5a1 1 0 0 1 1-1Zm0 14a1 1 0 0 1 1 1V21h-2v-1.5a1 1 0 0 1 1-1ZM4.5 11h2v2h-2a1 1 0 0 1 0-2Zm13 0h2a1 1 0 0 1 0 2h-2v-2ZM6.2 6.2a1 1 0 0 1 1.4 0l1.1 1.1-1.4 1.4-1.1-1.1a1 1 0 0 1 0-1.4Zm9.1 9.1 1.4-1.4 1.1 1.1a1 1 0 0 1-1.4 1.4l-1.1-1.1Zm1.4-9.1a1 1 0 0 1 0 1.4l-1.1 1.1-1.4-1.4 1.1-1.1a1 1 0 0 1 1.4 0Zm-9.1 9.1 1.4-1.4 1.1 1.1a1 1 0 0 1-1.4 1.4l-1.1-1.1Z"
      />
    {/if}
  </svg>
</button>

<style>
  .theme-toggle {
    display: inline-grid;
    width: 2.25rem;
    height: 2.25rem;
    place-items: center;
    padding: 0;
    border: 1px solid var(--ah-console-line);
    border-radius: 0.65rem;
    color: var(--ah-console-heading);
    background: var(--ah-console-card);
    cursor: pointer;
    transition: border-color var(--ah-motion-fast) ease;
  }

  .theme-toggle:hover,
  .theme-toggle:focus-visible {
    border-color: var(--ah-console-line-strong);
  }
</style>
