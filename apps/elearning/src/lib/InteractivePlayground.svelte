<script lang="ts">
  // Playground Python interaktif: worker + SharedArrayBuffer + input() blocking.
  // Port dari legacy project_ap/praktikum/frontend/src/routes/playground.
  import { onMount, onDestroy, tick } from 'svelte';

  type Status = 'init' | 'loading' | 'ready' | 'running' | 'unsupported';

  let status: Status = $state('init');
  let output = $state('');
  let needInput = $state(false);
  let inputValue = $state('');
  let scrollback: HTMLDivElement | undefined = $state();
  let worker: Worker | null = null;
  let meta: Int32Array | null = null;
  let dataArr: Uint8Array | null = null;

  const DATA_SIZE = 4096;

  let code = $state(`nama = input("Siapa namamu? ")
print("Halo,", nama, "!")
n = int(input("Masukkan sebuah angka: "))
print("Kuadratnya adalah", n * n)
`);

  function append(text: string) {
    output += text;
    void tick().then(() => {
      if (scrollback) scrollback.scrollTop = scrollback.scrollHeight;
    });
  }

  function setupWorker() {
    // SharedArrayBuffer butuh cross-origin isolation (header COOP/COEP).
    if (typeof SharedArrayBuffer === 'undefined' || !self.crossOriginIsolated) {
      status = 'unsupported';
      return;
    }
    const sab = new SharedArrayBuffer(8 + DATA_SIZE);
    meta = new Int32Array(sab, 0, 2);
    dataArr = new Uint8Array(sab, 8);

    status = 'loading';
    worker = new Worker('/playground-worker.js');
    worker.onmessage = (event: MessageEvent) => {
      const message = event.data;
      if (message.type === 'ready') status = 'ready';
      else if (message.type === 'out') append(message.data);
      else if (message.type === 'err') append(message.data);
      else if (message.type === 'need-input') {
        needInput = true;
        void tick().then(() =>
          scrollback?.parentElement?.querySelector('input')?.focus()
        );
      } else if (message.type === 'done') {
        status = 'ready';
        needInput = false;
      } else if (message.type === 'fatal') {
        append('\n[Gagal memuat Python runtime: ' + message.data + ']\n');
        status = 'unsupported';
      }
    };
    worker.postMessage({ type: 'init', sab });
  }

  function run() {
    if (status !== 'ready' || !worker) return;
    output = '';
    needInput = false;
    status = 'running';
    worker.postMessage({ type: 'run', code });
  }

  function submitInput(event: SubmitEvent) {
    event.preventDefault();
    if (!needInput || !meta || !dataArr) return;
    const line = inputValue;
    append(line + '\n'); // echo ke terminal
    const encoded = new TextEncoder().encode(line + '\n');
    const length = Math.min(encoded.length, DATA_SIZE);
    dataArr.set(encoded.subarray(0, length));
    Atomics.store(meta, 1, length);
    Atomics.store(meta, 0, 1);
    Atomics.notify(meta, 0);
    inputValue = '';
    needInput = false;
  }

  function stop() {
    // Hentikan run yang mungkin sedang menunggu input (Atomics.wait di worker).
    worker?.terminate();
    worker = null;
    needInput = false;
    status = 'init';
    setupWorker();
  }

  onMount(setupWorker);
  onDestroy(() => worker?.terminate());
</script>

<section class="stack" aria-labelledby="playground-interactive-title">
  <h1 id="playground-interactive-title">Playground Python (Live)</h1>
  <p class="muted">
    Jalankan kode Python langsung di browser — termasuk <code>input()</code> yang
    interaktif. Runtime diunduh sekali dari CDN publik lalu ter-cache.
  </p>
  {#if status === 'unsupported'}
    <p class="notice error" role="alert">
      Browser ini belum mendukung mode interaktif (butuh <em
        >cross-origin isolation</em
      >
      / SharedArrayBuffer). Coba Chrome/Edge/Firefox terbaru lewat HTTPS, atau pakai
      <a href="/playground-preview">playground preview</a> non-interaktif.
    </p>
  {:else}
    <div class="panel stack">
      <label
        >Kode Python<textarea
          class="code"
          rows="12"
          spellcheck="false"
          maxlength="65536"
          bind:value={code}
        ></textarea></label
      >
      <div class="actions">
        <button onclick={run} disabled={status !== 'ready'}>
          {status === 'loading'
            ? 'Memuat Python…'
            : status === 'running'
              ? 'Berjalan…'
              : 'Jalankan di browser'}
        </button>
        <button
          class="secondary"
          onclick={stop}
          disabled={status === 'loading' || status === 'init'}
          >Stop / Reset</button
        >
        {#if status === 'loading'}
          <span class="muted">Mengunduh runtime (sekali, lalu ter-cache)…</span>
        {/if}
      </div>
    </div>
    <div class="panel stack">
      <p class="muted" aria-label="Output terminal">Output / Terminal:</p>
      <div
        class="terminal-scrollback"
        data-testid="terminal-scrollback"
        role="log"
        aria-live="polite"
        bind:this={scrollback}
      >
        {output || '(output akan muncul di sini)'}
      </div>
      {#if needInput}
        <form class="terminal-input" onsubmit={submitInput}>
          <label>
            Program meminta input
            <input
              bind:value={inputValue}
              maxlength="4095"
              autocomplete="off"
              placeholder="ketik input lalu Enter…"
            />
          </label>
          <button type="submit">Kirim input</button>
        </form>
      {/if}
    </div>
  {/if}
</section>

<style>
  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    align-items: center;
  }

  .terminal-scrollback {
    max-height: 24rem;
    min-height: 10rem;
    overflow: auto;
    padding: 0.8rem;
    border-radius: var(--ah-console-radius-badge);
    background: var(--ah-console-sunken);
    color: var(--ah-console-text);
    font-family: var(--ah-font-mono, ui-monospace, monospace);
    font-size: 0.875rem;
    line-height: 1.5;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .terminal-input label {
    display: grid;
    gap: 0.35rem;
  }
</style>
