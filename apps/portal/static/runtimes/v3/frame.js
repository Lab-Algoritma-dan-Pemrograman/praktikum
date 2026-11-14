const base = new URL('.', location.href).href;
const assetBase = new URL('/runtimes/v1/clang/', location.origin).href;
const policy = document.createElement('meta');
policy.httpEquiv = 'Content-Security-Policy';
policy.content = `default-src 'none'; script-src 'wasm-unsafe-eval' ${base} ${assetBase} https://cdn.jsdelivr.net/pyodide/v0.25.0/full/; connect-src ${base} ${assetBase} https://cdn.jsdelivr.net/pyodide/v0.25.0/full/; worker-src blob:; base-uri 'none'; form-action 'none'`;
document.head.append(policy);

const budgets = { c: 20000, python: 10000 };
const recycleAfter = { c: 200, python: 50 };
const idleLimit = 10 * 60 * 1000;
const outputLimit = 16384;
const imageLimit = 524288;
const imageCount = 4;
const loadingLimit = 120000;
const encoder = new TextEncoder();

let port;
let worker;
let language = '';
let ready = false;
let pending = null;
let waitingInput = false;
let watchdog;
let loadingTimer;
let idleTimer;
let runs = 0;
let startedAt = 0;
let sab = null;
let sabMeta = null;
let sabData = null;
let collected;

function emptyCollected() {
  return { stdout: '', stderr: '', truncated: false, images: [] };
}

collected = emptyCollected();

function resetSAB() {
  if (!sabMeta) return;
  Atomics.store(sabMeta, 0, 0);
  Atomics.store(sabMeta, 1, 0);
}

function stopWorker() {
  clearTimeout(watchdog);
  clearTimeout(loadingTimer);
  clearTimeout(idleTimer);
  watchdog = undefined;
  loadingTimer = undefined;
  idleTimer = undefined;
  worker?.terminate();
  worker = undefined;
  ready = false;
  runs = 0;
  resetSAB();
}

function scheduleIdle() {
  clearTimeout(idleTimer);
  idleTimer = setTimeout(() => stopWorker(), idleLimit);
}

function armWatchdog() {
  clearTimeout(watchdog);
  if (!pending) return;
  watchdog = setTimeout(onTimeout, budgets[language] ?? 10000);
}

function onTimeout() {
  if (!pending) return;
  const id = pending.id;
  const durationMs = Date.now() - startedAt;
  pending = null;
  waitingInput = false;
  clearTimeout(loadingTimer);
  loadingTimer = undefined;
  stopWorker();
  port.postMessage({
    type: 'result',
    id,
    result: {
      status: 'timeout',
      stdout: collected.stdout,
      stderr: collected.stderr,
      truncated: collected.truncated,
      images: collected.images,
      durationMs: Math.max(0, durationMs)
    }
  });
}

function collect(stream, text) {
  const value = String(text);
  if (!value) return;
  const used = encoder.encode(collected[stream]).length;
  const room = outputLimit - used;
  if (room <= 0) {
    collected.truncated = true;
    return;
  }
  const bytes = encoder.encode(value);
  const chunk = new TextDecoder().decode(bytes.subarray(0, room));
  collected[stream] += chunk;
  if (bytes.length > room) collected.truncated = true;
}

function collectImage(value) {
  const data = String(value ?? '');
  if (
    !data.startsWith('data:image/png;base64,') ||
    data.length > imageLimit ||
    collected.images.length >= imageCount
  ) {
    collected.truncated = true;
    return;
  }
  collected.images.push(data);
}

function handleWorkerMessage(data) {
  if (!data || typeof data !== 'object') return;
  if (data.type === 'ready') {
    ready = true;
    clearTimeout(idleTimer);
    if (pending) sendCurrentRun();
    else scheduleIdle();
    return;
  }
  if (data.type === 'prefetch') {
    port.postMessage({
      type: 'prefetch',
      loaded: Number(data.loaded) || 0,
      total: Number(data.total) || 0
    });
    return;
  }
  if (data.type === 'init-error') {
    const id = pending?.id;
    pending = null;
    waitingInput = false;
    stopWorker();
    if (id) port.postMessage({ type: 'failed', id });
    return;
  }
  if (!pending) return;
  if (data.type === 'phase' && data.id === pending.id) {
    port.postMessage({ type: 'phase', id: data.id, phase: data.phase });
    return;
  }
  if (data.type === 'loading' && data.id === pending.id) {
    clearTimeout(watchdog);
    clearTimeout(loadingTimer);
    loadingTimer = setTimeout(onTimeout, loadingLimit);
    port.postMessage({ type: 'phase', id: data.id, phase: 'packages' });
    return;
  }
  if (data.type === 'loaded' && data.id === pending.id) {
    clearTimeout(loadingTimer);
    loadingTimer = undefined;
    armWatchdog();
    port.postMessage({ type: 'phase', id: data.id, phase: 'run' });
    return;
  }
  if (data.type === 'write' && data.id === pending.id) {
    if (typeof data.text === 'string') {
      collect(data.stream === 'stderr' ? 'stderr' : 'stdout', data.text);
      port.postMessage({
        type: 'write',
        id: data.id,
        stream: data.stream === 'stderr' ? 'stderr' : 'stdout',
        text: data.text
      });
    }
    return;
  }
  if (data.type === 'image' && data.id === pending.id) {
    collectImage(data.data);
    return;
  }
  if (data.type === 'input-request' && data.id === pending.id) {
    if (pending.language !== 'python' && !sabMeta) return;
    waitingInput = true;
    clearTimeout(watchdog);
    port.postMessage({
      type: 'input-request',
      id: data.id,
      stdout: collected.stdout,
      prompt: typeof data.prompt === 'string' ? data.prompt : ''
    });
    return;
  }
  if (data.type !== 'result' || data.id !== pending.id) return;
  const id = pending.id;
  pending = null;
  waitingInput = false;
  clearTimeout(watchdog);
  clearTimeout(loadingTimer);
  port.postMessage({
    type: 'result',
    id,
    result: { ...data.result, images: collected.images }
  });
  runs += 1;
  if (
    data.result?.status !== 'success' ||
    runs >= (recycleAfter[language] ?? 200)
  )
    stopWorker();
  else scheduleIdle();
}

function startWorker(nextLanguage) {
  stopWorker();
  language = nextLanguage;
  resetSAB();
  const blob = new Blob(
    [`importScripts(${JSON.stringify(new URL('worker.js', base).href)});`],
    { type: 'text/javascript' }
  );
  const url = URL.createObjectURL(blob);
  worker = new Worker(url);
  URL.revokeObjectURL(url);
  worker.onmessage = ({ data }) => handleWorkerMessage(data);
  worker.onerror = () => {
    if (pending) {
      const id = pending.id;
      pending = null;
      port.postMessage({ type: 'failed', id });
    }
    stopWorker();
  };
  worker.postMessage({ type: 'init', language, base: assetBase });
}

function ensureWorker(nextLanguage) {
  if (worker && language === nextLanguage) return;
  startWorker(nextLanguage);
}

function sendCurrentRun() {
  if (!pending || !ready) return;
  clearTimeout(loadingTimer);
  armWatchdog();
  port.postMessage({ type: 'phase', id: pending.id, phase: 'run' });
  worker.postMessage({
    type: 'run',
    id: pending.id,
    language,
    source: pending.source,
    stdin: pending.stdin,
    interactive: pending.interactive === true,
    sab
  });
}

addEventListener('message', (event) => {
  if (
    event.source !== parent ||
    port ||
    event.data?.type !== 'connect' ||
    event.ports.length !== 1
  )
    return;
  port = event.ports[0];
  port.onmessage = ({ data }) => {
    if (!data || typeof data !== 'object') return;
    if (data.type === 'cancel') {
      if (pending?.id === data.id) {
        pending = null;
        waitingInput = false;
        stopWorker();
      }
      return;
    }
    if (data.type === 'warmup') {
      if (pending || !['c', 'python'].includes(data.language)) return;
      ensureWorker(data.language);
      return;
    }
    if (data.type === 'input') {
      if (!pending || !waitingInput || data.id !== pending.id) return;
      if (pending.language === 'python') {
        worker.postMessage({
          type: 'input',
          id: pending.id,
          value: String(data.value ?? '')
        });
        waitingInput = false;
        armWatchdog();
        return;
      }
      if (!sabMeta) {
        waitingInput = false;
        armWatchdog();
        return;
      }
      const bytes = encoder.encode(String(data.value ?? ''));
      const room = sabData.length - 1;
      const length = Math.min(room, bytes.length);
      sabData.set(bytes.subarray(0, length), 0);
      Atomics.store(sabMeta, 1, length);
      Atomics.store(sabMeta, 0, 1);
      Atomics.notify(sabMeta, 0);
      waitingInput = false;
      armWatchdog();
      return;
    }
    if (
      pending ||
      data.type !== 'run' ||
      !['c', 'python'].includes(data.language)
    )
      return;
    if (
      typeof data.id !== 'string' ||
      typeof data.source !== 'string' ||
      typeof data.stdin !== 'string'
    )
      return;
    if (
      encoder.encode(data.source).length > 65536 ||
      encoder.encode(data.stdin).length > 16384
    )
      return;
    if (data.language !== language && worker) stopWorker();
    pending = data;
    waitingInput = false;
    collected = emptyCollected();
    startedAt = Date.now();
    if (
      data.interactive === true &&
      crossOriginIsolated &&
      typeof SharedArrayBuffer === 'function'
    ) {
      if (!sab) {
        sab = new SharedArrayBuffer(65544);
        sabMeta = new Int32Array(sab, 0, 2);
        sabData = new Uint8Array(sab, 8);
      }
      resetSAB();
    } else {
      sab = null;
      sabMeta = null;
      sabData = null;
    }
    clearTimeout(idleTimer);
    ensureWorker(data.language);
    if (ready) sendCurrentRun();
    else {
      clearTimeout(loadingTimer);
      loadingTimer = setTimeout(onTimeout, 60000);
      port.postMessage({
        type: 'phase',
        id: pending.id,
        phase: 'initializing'
      });
    }
  };
  port.postMessage({ type: 'connected' });
});
