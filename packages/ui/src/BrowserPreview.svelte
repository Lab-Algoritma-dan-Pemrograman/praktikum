<script lang="ts">
  import { onMount } from 'svelte';
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
  let live = { stdout: '', stderr: '' };
  let prefetched = 0;
  let serverBusy = false;
  let serverResult: PlaygroundFallbackResult | null = null;
  let serverIssue = '';
  let interactive = false;
  let inputPrompt = '';
  let inputValue = '';
  let inputWaiting = false;
  let inputResolver: ((value: string) => void) | undefined;

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
    live = { stdout: '', stderr: '' };
    prefetched = 0;
    try {
      const cases = verify
        ? verificationCases.length > 0
          ? verificationCases
          : [{ description: 'Program dapat dijalankan', input: stdin }]
        : [{ description: 'Preview', input: stdin }];
      for (const testCase of cases) {
        live = { stdout: '', stderr: '' };
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
              live = { ...live, [stream]: cap(live[stream] + text) };
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
      live = { stdout: '', stderr: '' };
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
  {#if inputWaiting}
    <form class="panel stack" onsubmit={submitInput}>
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
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if busy && prefetched > 0 && prefetched < 2}
    <p class="muted">Menyiapkan runtime C: {prefetched}/2 bagian.</p>
  {/if}
  {#if busy && (live.stdout || live.stderr)}
    <div class="panel stack" aria-label="Keluaran sementara">
      {#if live.stdout}<pre
          class="runner-output"
          aria-label="stdout sementara">{live.stdout}</pre>{/if}
      {#if live.stderr}<pre
          class="runner-output"
          aria-label="stderr sementara">{live.stderr}</pre>{/if}
    </div>
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
  {#if result}
    <div class="panel stack" aria-live="polite">
      <h3>Preview browser: {result.status}</h3>
      <p class="muted" data-testid="preview-duration">
        {result.durationMs} ms · hasil lokal, tidak dikirim sebagai nilai
      </p>
      <h4>stdout preview</h4>
      <pre
        class="runner-output"
        aria-label="stdout preview browser">{result.stdout || '(kosong)'}</pre>
      <h4>stderr preview</h4>
      <pre
        class="runner-output"
        aria-label="stderr preview browser">{result.stderr || '(kosong)'}</pre>
      {#if result.images.length}
        <h4>Grafik preview</h4>
        <div class="stack">
          {#each result.images as image, index}
            <img
              class="preview-image"
              src={image}
              alt={`Grafik hasil preview ${index + 1}`}
            />
          {/each}
        </div>
      {/if}
      {#if result.truncated}<p>Output dipotong sesuai batas ukuran.</p>{/if}
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
</style>
