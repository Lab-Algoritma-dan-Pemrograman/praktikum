<script lang="ts">
  import { onMount, tick } from 'svelte';
  import {
    acquirePreview,
    normalizeBrowserTestOutput,
    type BrowserRunResult,
    type BrowserVerificationCase,
    type BrowserVerificationResult,
    type PlaygroundFallbackResult
  } from './browser-preview';

  export let runtimeOrigin: string;
  export let language: string;
  export let source: string;
  export let stdin = '';
  export let disabled = false;
  export let verify = false;
  export let verificationCases: BrowserVerificationCase[] = [];
  export let onVerification:
    ((result: BrowserVerificationResult | null) => void) | undefined =
    undefined;
  export let serverRun:
    | ((input: {
        language: 'c' | 'python';
        source: string;
        stdin: string;
      }) => Promise<PlaygroundFallbackResult>)
    | undefined = undefined;

  let preview: ReturnType<typeof acquirePreview> | undefined;
  let abort: AbortController | undefined;
  let busy = false;
  let issue = '';
  let result: BrowserRunResult | null = null;
  let verificationResults: BrowserVerificationResult['results'] = [];
  let prefetched = 0;
  let serverBusy = false;
  let serverResult: PlaygroundFallbackResult | null = null;
  let serverIssue = '';
  let interactive = false;
  let inputPrompt = '';
  let inputValue = '';
  let inputWaiting = false;
  let inputResolver: ((value: string) => void) | undefined;

  type TerminalLine = {
    kind: 'stdout' | 'stderr' | 'input' | 'system';
    text: string;
  };

  const terminalCap = 16384;
  let terminal: TerminalLine[] = [];
  let terminalBytes = 0;
  let scrollback: HTMLDivElement | undefined;
  let written = { stdout: '', stderr: '' };

  function appendTerminal(kind: TerminalLine['kind'], text: string) {
    if (!text || terminalBytes >= terminalCap) return;
    const bytes = new TextEncoder().encode(text);
    const room = terminalCap - terminalBytes;
    const chunk =
      bytes.length <= room
        ? text
        : new TextDecoder().decode(bytes.subarray(0, room));
    terminalBytes += Math.min(bytes.length, room);
    const last = terminal[terminal.length - 1];
    terminal =
      last && last.kind === kind
        ? [...terminal.slice(0, -1), { kind, text: last.text + chunk }]
        : [...terminal, { kind, text: chunk }];
    void tick().then(() => {
      if (scrollback) scrollback.scrollTop = scrollback.scrollHeight;
    });
  }

  function resetTerminal() {
    terminal = [];
    terminalBytes = 0;
    written = { stdout: '', stderr: '' };
  }

  function requestInput(prompt = '') {
    inputPrompt = prompt;
    inputValue = '';
    inputWaiting = true;
    return new Promise<string>((resolve) => {
      inputResolver = resolve;
    });
  }

  function submitInput(event: SubmitEvent) {
    event.preventDefault();
    const resolve = inputResolver;
    inputResolver = undefined;
    inputWaiting = false;
    appendTerminal('input', `${inputPrompt}${inputValue}\n`);
    inputPrompt = '';
    resolve?.(inputValue);
  }

  function focusInput(node: HTMLInputElement) {
    node.focus();
  }

  onMount(() => {
    preview = acquirePreview();
    return () => {
      abort?.abort();
      preview?.release();
    };
  });

  const cap = (text: string) => {
    const bytes = new TextEncoder().encode(text);
    return bytes.length <= 16384
      ? text
      : new TextDecoder().decode(bytes.subarray(0, 16384));
  };

  async function run() {
    if (!preview || busy || (language !== 'c' && language !== 'python')) return;
    const runLanguage = language;
    const sourceSnapshot = source;
    abort = new AbortController();
    busy = true;
    issue = '';
    result = null;
    verificationResults = [];
    onVerification?.(null);
    serverResult = null;
    serverIssue = '';
    resetTerminal();
    prefetched = 0;
    try {
      const cases = verify
        ? verificationCases.length > 0
          ? verificationCases
          : [{ description: 'Program dapat dijalankan', input: stdin }]
        : [{ description: 'Preview', input: stdin }];
      for (const testCase of cases) {
        resetTerminal();
        result = await preview.run(
          runtimeOrigin,
          {
            language: runLanguage,
            source: sourceSnapshot,
            stdin: testCase.input
          },
          abort.signal,
          {
            onWrite: (stream, text) => {
              written[stream] = cap(written[stream] + text);
              appendTerminal(stream, text);
            },
            onPrefetch: (loaded) => (prefetched = loaded),
            onInput: requestInput,
            interactive: verify
              ? false
              : runLanguage === 'python'
                ? interactive
                : true
          }
        );
        const passed =
          result.status === 'success' &&
          !result.truncated &&
          (testCase.expectedOutput === undefined ||
            normalizeBrowserTestOutput(result.stdout) ===
              normalizeBrowserTestOutput(testCase.expectedOutput));
        for (const stream of ['stdout', 'stderr'] as const) {
          const seen = written[stream];
          const produced = result[stream];
          if (produced.startsWith(seen))
            appendTerminal(stream, produced.slice(seen.length));
        }
        if (verify) {
          verificationResults = [
            ...verificationResults,
            {
              description: testCase.description,
              passed,
              expectedOutput: testCase.expectedOutput,
              actualOutput: result.stdout
            }
          ];
          if (!passed) break;
        }
      }
      if (result?.status === 'timeout') {
        issue =
          runLanguage === 'c'
            ? 'Eksekusi melebihi 20 detik; worker dihentikan.'
            : 'Eksekusi melebihi 10 detik; worker dihentikan.';
      }
      if (verify) {
        const verification = {
          passed:
            verificationResults.length === cases.length &&
            verificationResults.every((item) => item.passed),
          language: runLanguage,
          source: sourceSnapshot,
          results: verificationResults
        } satisfies BrowserVerificationResult;
        onVerification?.(verification);
      }
    } catch (error) {
      issue =
        error instanceof Error ? error.message : 'Preview belum tersedia.';
      onVerification?.(null);
    } finally {
      busy = false;
      inputWaiting = false;
      inputResolver?.('');
      inputResolver = undefined;
    }
  }

  async function runOnServer() {
    if (!serverRun || serverBusy || (language !== 'c' && language !== 'python'))
      return;
    serverBusy = true;
    serverIssue = '';
    serverResult = null;
    try {
      serverResult = await serverRun({ language, source, stdin });
    } catch (error) {
      serverIssue =
        error instanceof Error ? error.message : 'Fallback server gagal.';
    } finally {
      serverBusy = false;
    }
  }
</script>

<section class="stack" aria-label="Preview lokal di browser">
  <p class="muted">
    Preview berjalan di browser, bukan di server. Runtime hanya diunduh saat
    diperlukan; unduhan pertama compiler C sekitar 60 MB. Hasil ini bukan
    verifikasi penilaian server.
  </p>
  <div class="actions">
    <button
      class="secondary"
      onclick={run}
      disabled={disabled ||
        busy ||
        !preview ||
        !runtimeOrigin ||
        !source.trim() ||
        !['c', 'python'].includes(language)}
      >{busy
        ? 'Menyiapkan / menjalankan preview…'
        : verify
          ? 'Jalankan & uji di browser'
          : 'Jalankan di browser'}</button
    >
    {#if busy}<button class="secondary" onclick={() => abort?.abort()}
        >Hentikan preview</button
      >{/if}
    {#if serverRun}
      <button
        class="secondary"
        onclick={runOnServer}
        disabled={disabled || serverBusy || !source.trim()}
        >{serverBusy
          ? 'Menjalankan fallback…'
          : 'Fallback Glot (preview)'}</button
      >
    {/if}
  </div>
  {#if language === 'python'}
    <label class="muted">
      <input
        type="checkbox"
        bind:checked={interactive}
        disabled={disabled || busy}
      />
      Input interaktif — program meminta input saat berjalan
    </label>
    {#if interactive}
      <p class="muted">
        Input standar diabaikan. Mode ini mendukung input() pada kode tingkat
        atas; untuk input di fungsi sinkron, nonaktifkan mode ini dan isi input
        standar sebelum Run.
      </p>
    {/if}
  {/if}
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if busy && prefetched > 0 && prefetched < 2}
    <p class="muted">Menyiapkan runtime C: {prefetched}/2 bagian.</p>
  {/if}
  {#if verificationResults.length > 0}
    <section class="panel stack" aria-label="Hasil testcase browser">
      <h3>Hasil testcase browser</h3>
      <ul>
        {#each verificationResults as testCase}
          <li>
            {testCase.description}:
            <strong>{testCase.passed ? 'Lulus' : 'Belum sesuai'}</strong>
          </li>
        {/each}
      </ul>
    </section>
  {/if}
  {#if busy || result}
    <div class="panel stack">
      <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
      <div
        class="terminal-scrollback"
        data-testid="terminal-scrollback"
        role="log"
        aria-live="polite"
        tabindex="0"
        bind:this={scrollback}
      >
        {#each terminal as line}
          <div class="term-line" data-kind={line.kind}>{line.text}</div>
        {/each}
        {#if inputWaiting}
          <div class="term-line term-prompt" data-kind="prompt">
            {inputPrompt}<span class="term-caret" aria-hidden="true">▍</span>
          </div>
        {/if}
      </div>
      {#if result}
        {#if result.images.length}
          <div class="terminal-images">
            {#each result.images as image, index}
              <img
                class="preview-image"
                src={image}
                alt={`Grafik hasil preview ${index + 1}`}
              />
            {/each}
          </div>
        {/if}
        {#if result.truncated}<p class="terminal-notice">
            Output dipotong sesuai batas ukuran.
          </p>{/if}
        <p
          class="terminal-status status-{result.status}"
          data-testid="terminal-status"
        >
          Preview browser: {result.status}
        </p>
        <p class="muted" data-testid="preview-duration">
          {result.durationMs} ms · hasil lokal, tidak dikirim sebagai nilai
        </p>
      {/if}
      {#if inputWaiting}
        <form class="terminal-input" onsubmit={submitInput}>
          <label>
            {inputPrompt || 'Program meminta input'}
            <input
              bind:value={inputValue}
              maxlength="16383"
              autocomplete="off"
              use:focusInput
            />
          </label>
          <button type="submit">Kirim input</button>
        </form>
      {/if}
    </div>
  {/if}
  {#if serverResult}
    <div
      class="panel stack"
      aria-live="polite"
      aria-label="Hasil fallback server"
    >
      <h3>Fallback server: {serverResult.status}</h3>
      <p class="muted">
        {serverResult.duration_ms} ms · provider {serverResult.provider} · bukan bukti
        penilaian
      </p>
      <h4>stdout fallback</h4>
      <pre
        class="runner-output"
        aria-label="stdout fallback server">{serverResult.stdout ||
          '(kosong)'}</pre>
      <h4>stderr fallback</h4>
      <pre
        class="runner-output"
        aria-label="stderr fallback server">{serverResult.stderr ||
          '(kosong)'}</pre>
      {#if serverResult.truncated}<p>
          Output dipotong sesuai batas ukuran.
        </p>{/if}
    </div>
  {/if}
  {#if serverIssue}<p class="notice error" role="alert">{serverIssue}</p>{/if}
</section>

<style>
  .preview-image {
    display: block;
    max-width: 100%;
    height: auto;
    margin: 0.5rem 0;
  }

  .terminal-scrollback {
    max-height: 20rem;
    overflow: auto;
    padding: 0.8rem;
    border-radius: var(--ah-console-radius-badge);
    background: var(--ah-console-sunken);
    color: var(--ah-console-text);
    font-family: var(--ah-font-mono, ui-monospace, monospace);
    font-size: 0.875rem;
    line-height: 1.5;
  }

  .term-line {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .term-line[data-kind='stderr'] {
    color: var(--ah-console-danger);
  }

  .term-line[data-kind='input'] {
    color: var(--ah-console-accent-ink);
  }

  .term-line[data-kind='system'] {
    color: var(--ah-console-muted);
  }

  .term-prompt {
    color: var(--ah-console-muted);
  }

  .term-caret {
    color: var(--ah-console-accent);
  }

  .terminal-images {
    display: grid;
    gap: var(--ah-space-2, 0.5rem);
  }

  .terminal-notice {
    color: var(--ah-console-warning);
  }

  .terminal-input label {
    display: grid;
    gap: 0.35rem;
  }

  :global(.terminal-status) {
    font-weight: 600;
  }

  :global(.terminal-status.status-success) {
    color: var(--ah-console-accent);
  }

  :global(.terminal-status.status-timeout) {
    color: var(--ah-console-warning);
  }

  :global(.terminal-status.status-compile_error),
  :global(.terminal-status.status-runtime_error) {
    color: var(--ah-console-danger);
  }
</style>
