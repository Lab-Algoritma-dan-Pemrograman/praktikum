export type PreviewStatus =
  'success' | 'compile_error' | 'runtime_error' | 'timeout';

export interface BrowserRunResult {
  status: PreviewStatus;
  stdout: string;
  stderr: string;
  truncated: boolean;
  images: string[];
  durationMs: number;
}

export interface BrowserVerificationCase {
  description: string;
  input: string;
  expectedOutput?: string;
}

export interface BrowserVerificationResult {
  passed: boolean;
  language: 'c' | 'python';
  source: string;
  results: Array<{
    description: string;
    passed: boolean;
    expectedOutput?: string;
    actualOutput: string;
  }>;
}

export function normalizeBrowserTestOutput(value: string): string {
  return value.replaceAll('\r\n', '\n').replace(/\n+$/, '');
}

export interface PlaygroundFallbackResult {
  status:
    | 'success'
    | 'compile_error'
    | 'runtime_error'
    | 'timeout'
    | 'provider_error';
  stdout: string;
  stderr: string;
  truncated: boolean;
  duration_ms: number;
  provider: 'glot';
  authoritative: false;
}

export function runtimeFrameURL(origin: string): string {
  const url = new URL(origin);
  const local =
    url.hostname === 'localhost' ||
    url.hostname.endsWith('.localhost') ||
    url.hostname === '127.0.0.1';
  if (
    (url.protocol !== 'https:' && !(url.protocol === 'http:' && local)) ||
    url.username ||
    url.password ||
    url.search ||
    url.hash ||
    url.pathname !== '/'
  )
    throw new Error('Origin runtime tidak valid.');
  return new URL('/runtimes/v3/frame.html', url).href;
}

export function boundedPreview(value: unknown): BrowserRunResult {
  if (!value || typeof value !== 'object')
    throw new Error('Hasil preview tidak valid.');
  const result = value as Record<string, unknown>;
  if (
    !['success', 'compile_error', 'runtime_error', 'timeout'].includes(
      String(result.status)
    ) ||
    typeof result.stdout !== 'string' ||
    typeof result.stderr !== 'string' ||
    typeof result.durationMs !== 'number' ||
    !Number.isFinite(result.durationMs) ||
    result.durationMs < 0
  )
    throw new Error('Hasil preview tidak valid.');
  const boundedText = (text: string) => {
    const bytes = new TextEncoder().encode(text.slice(0, 16385));
    return new TextDecoder().decode(bytes.subarray(0, 16384), { stream: true });
  };
  const stdout = boundedText(result.stdout);
  const stderr = boundedText(result.stderr);
  const images: string[] = [];
  let imagesTruncated = false;
  if (result.images !== undefined) {
    if (!Array.isArray(result.images))
      throw new Error('Hasil preview tidak valid.');
    for (const image of result.images.slice(0, 4)) {
      if (
        typeof image !== 'string' ||
        !image.startsWith('data:image/png;base64,') ||
        image.length > 524288
      ) {
        imagesTruncated = true;
        continue;
      }
      images.push(image);
    }
    if (result.images.length > 4) imagesTruncated = true;
  }
  return {
    status: result.status as PreviewStatus,
    stdout,
    stderr,
    truncated:
      result.truncated === true ||
      imagesTruncated ||
      stdout !== result.stdout ||
      stderr !== result.stderr,
    images,
    durationMs: Math.min(45000, Math.round(result.durationMs))
  };
}

type PreviewInput = {
  language: 'c' | 'python';
  source: string;
  stdin: string;
};

type PreviewOptions = {
  onWrite?: (stream: 'stdout' | 'stderr', text: string) => void;
  onPrefetch?: (loaded: number, total: number) => void;
  onInput?: (prompt?: string) => Promise<string>;
  interactive?: boolean;
};

type PendingRun = {
  id: string;
  input: PreviewInput;
  resolve: (result: BrowserRunResult) => void;
  reject: (error: Error) => void;
  cleanup: () => void;
  options: PreviewOptions;
  waitingInput?: boolean;
};

const idleDisposeMS = 10 * 60 * 1000;
const bootstrapMS = 60000;
const backstopMS = 45000;
const packageBackstopMS = 125000;

class BrowserPreviewClient {
  private frame?: HTMLIFrameElement;
  private port?: MessagePort;
  private connected = false;
  private origin = '';
  private frameTimer?: ReturnType<typeof setTimeout>;
  private runTimer?: ReturnType<typeof setTimeout>;
  private idleTimer?: ReturnType<typeof setTimeout>;
  private pending?: PendingRun;
  private warmupLanguage = '';
  private sequence = 0;

  run(
    origin: string,
    input: PreviewInput,
    signal: AbortSignal,
    options: PreviewOptions = {}
  ): Promise<BrowserRunResult> {
    if (this.pending)
      return Promise.reject(
        new Error(
          'Preview lain masih berjalan. Hentikan atau tunggu hingga selesai.'
        )
      );
    if (signal.aborted) return Promise.reject(new Error('Preview dibatalkan.'));
    const encoder = new TextEncoder();
    if (
      !['c', 'python'].includes(input.language) ||
      !input.source.trim() ||
      encoder.encode(input.source).length > 65536 ||
      encoder.encode(input.stdin).length > 16384
    )
      return Promise.reject(
        new Error('Kode maksimal 64 KiB dan input maksimal 16 KiB.')
      );
    try {
      this.ensureFrame(origin);
    } catch {
      return Promise.reject(new Error('Origin runtime tidak valid.'));
    }
    this.cancelIdle();
    return new Promise((resolve, reject) => {
      const abort = () => this.fail(new Error('Preview dibatalkan.'));
      this.pending = {
        id: String(++this.sequence),
        input,
        resolve,
        reject,
        cleanup: () => signal.removeEventListener('abort', abort),
        options
      };
      signal.addEventListener('abort', abort, { once: true });
      if (this.connected) this.dispatchRun();
      else this.armBootstrap();
    });
  }

  warmup(origin: string, language: string) {
    if (language !== 'c' && language !== 'python') return;
    if (this.pending) return;
    try {
      this.ensureFrame(origin);
    } catch {
      return;
    }
    this.cancelIdle();
    this.warmupLanguage = language;
    if (this.connected) this.sendWarmup();
    else if (!this.frameTimer) this.armBootstrap();
  }

  private ensureFrame(origin: string) {
    const url = runtimeFrameURL(origin);
    if (this.frame && this.origin !== origin) this.dispose();
    if (this.frame) return;
    this.origin = origin;
    const frame = document.createElement('iframe');
    frame.title = 'Isolated browser code preview';
    frame.hidden = true;
    frame.setAttribute('sandbox', 'allow-scripts');
    frame.referrerPolicy = 'no-referrer';
    frame.src = url;
    frame.onload = () => {
      const channel = new MessageChannel();
      this.port = channel.port1;
      this.port.onmessage = (event) => this.receive(event.data);
      frame.contentWindow?.postMessage({ type: 'connect' }, '*', [
        channel.port2
      ]);
    };
    this.frame = frame;
    document.body.append(frame);
  }

  private armBootstrap() {
    clearTimeout(this.frameTimer);
    this.frameTimer = setTimeout(() => this.bootstrapFailed(), bootstrapMS);
  }

  private bootstrapFailed() {
    clearTimeout(this.frameTimer);
    this.frameTimer = undefined;
    if (this.pending) {
      this.fail(
        new Error(
          'Runtime belum berhasil dimuat dalam 60 detik. Coba lagi secara manual.'
        )
      );
      return;
    }
    if (this.frame) {
      this.port?.close();
      this.frame.remove();
      this.port = undefined;
      this.frame = undefined;
      this.connected = false;
    }
  }

  private dispatchRun() {
    if (!this.connected || !this.pending) return;
    clearTimeout(this.frameTimer);
    this.frameTimer = undefined;
    this.port?.postMessage({
      type: 'run',
      id: this.pending.id,
      ...this.pending.input,
      interactive: this.pending.options.interactive === true
    });
    this.armBackstop();
  }

  private armBackstop(timeout = backstopMS) {
    clearTimeout(this.runTimer);
    this.runTimer = setTimeout(
      () =>
        this.fail(
          new Error('Preview tidak merespons. Worker dihentikan; coba lagi.')
        ),
      timeout
    );
  }

  private sendWarmup() {
    if (!this.connected || !this.warmupLanguage) return;
    this.port?.postMessage({ type: 'warmup', language: this.warmupLanguage });
  }

  private receive(data: { type?: string; id?: string; result?: unknown }) {
    if (!data || typeof data !== 'object') return;
    if (data.type === 'connected') {
      this.connected = true;
      clearTimeout(this.frameTimer);
      this.frameTimer = undefined;
      if (this.pending) this.dispatchRun();
      else this.sendWarmup();
      return;
    }
    if (data.type === 'prefetch') {
      const value = data as { loaded?: unknown; total?: unknown };
      if (this.pending)
        this.pending.options.onPrefetch?.(
          Number(value.loaded) || 0,
          Number(value.total) || 0
        );
      return;
    }
    if (!this.pending || data.id !== this.pending.id) return;
    if (data.type === 'phase') {
      const phase = (data as { phase?: unknown }).phase;
      if (phase === 'packages') this.armBackstop(packageBackstopMS);
      else if (phase === 'initializing') this.armBackstop(bootstrapMS + 5000);
      else if (phase === 'run') this.armBackstop();
      return;
    }
    if (data.type === 'write') {
      const message = data as { stream?: unknown; text?: unknown };
      if (typeof message.text === 'string')
        this.pending.options.onWrite?.(
          message.stream === 'stderr' ? 'stderr' : 'stdout',
          message.text
        );
      return;
    }
    if (data.type === 'input-request') {
      if (this.pending.waitingInput) return;
      this.pending.waitingInput = true;
      clearTimeout(this.runTimer);
      this.runTimer = undefined;
      const onInput = this.pending.options.onInput;
      if (!onInput) {
        this.fail(
          new Error(
            'Program meminta input interaktif, tetapi mode interaktif tidak tersedia.'
          )
        );
        return;
      }
      const id = this.pending.id;
      const prompt =
        typeof (data as { prompt?: unknown }).prompt === 'string'
          ? (data as { prompt: string }).prompt
          : '';
      void onInput(prompt)
        .then((value) => {
          if (!this.pending || this.pending.id !== id) return;
          if (new TextEncoder().encode(`${value}\n`).length > 16384)
            throw new Error('Input maksimal 16 KiB per baris.');
          this.pending.waitingInput = false;
          this.port?.postMessage({ type: 'input', id, value: `${value}\n` });
          this.armBackstop();
        })
        .catch((error) => {
          if (!this.pending || this.pending.id !== id) return;
          this.fail(
            error instanceof Error ? error : new Error('Input dibatalkan.')
          );
        });
      return;
    }
    if (data.type === 'result') {
      try {
        const result = boundedPreview(data.result);
        const pending = this.pending;
        this.finish();
        pending.resolve(result);
      } catch {
        this.fail(new Error('Hasil preview tidak valid.'));
      }
      return;
    }
    if (data.type === 'failed')
      this.fail(
        new Error(
          'Runtime browser gagal dimuat. Tidak ada fallback server otomatis.'
        )
      );
  }

  private finish() {
    clearTimeout(this.runTimer);
    clearTimeout(this.frameTimer);
    this.runTimer = undefined;
    this.frameTimer = undefined;
    this.pending?.cleanup();
    this.pending = undefined;
  }

  private fail(error: Error) {
    const pending = this.pending;
    if (!pending) return;
    this.port?.postMessage({ type: 'cancel', id: pending.id });
    this.finish();
    if (!this.connected) {
      this.port?.close();
      this.frame?.remove();
      this.port = undefined;
      this.frame = undefined;
    }
    pending.reject(error);
  }

  private cancelIdle() {
    clearTimeout(this.idleTimer);
    this.idleTimer = undefined;
  }

  scheduleIdle() {
    this.cancelIdle();
    this.idleTimer = setTimeout(() => this.dispose(), idleDisposeMS);
  }

  dispose() {
    this.fail(new Error('Preview ditutup.'));
    clearTimeout(this.idleTimer);
    clearTimeout(this.frameTimer);
    clearTimeout(this.runTimer);
    this.idleTimer = undefined;
    this.frameTimer = undefined;
    this.runTimer = undefined;
    this.port?.close();
    this.frame?.remove();
    this.port = undefined;
    this.frame = undefined;
    this.connected = false;
    this.warmupLanguage = '';
  }
}

let users = 0;
const client = new BrowserPreviewClient();
if (typeof window !== 'undefined')
  window.addEventListener('pagehide', () => client.dispose());

export function warmupPreview(origin: string, language: string) {
  if (origin) client.warmup(origin, language);
}

export function acquirePreview() {
  users += 1;
  let released = false;
  return {
    run: client.run.bind(client),
    warmup: client.warmup.bind(client),
    release() {
      if (released) return;
      released = true;
      if (--users === 0) client.scheduleIdle();
    }
  };
}
