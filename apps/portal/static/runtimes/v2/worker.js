const COMPILER_VERSION = 'clang-8.0.1-wasm-r2';
const OUTPUT_LIMIT = 16384;
const FLUSH_MS = 16;
const FLUSH_BYTES = 16384;
const encoder = new TextEncoder();
const decoder = new TextDecoder();

let language = '';
let base = '';
let pyodide;
let sysroot;
let api;
const modules = new Map();
const compileCache = new Map();

let runId = null;
let phase = 'run';
let interactive = false;
let sab = null;
let sabMeta = null;
let sabData = null;
let busy = false;

let stdout = '';
let stderr = '';
let stdoutBytes = 0;
let stderrBytes = 0;
let truncated = false;
let buffers = { stdout: '', stderr: '' };
let flushTimer = null;

function flushStream(stream) {
  const text = buffers[stream];
  if (!text) return;
  buffers[stream] = '';
  postMessage({ type: 'write', id: runId, stream, text });
}

function flushAll() {
  clearTimeout(flushTimer);
  flushTimer = null;
  flushStream('stdout');
  flushStream('stderr');
}

function scheduleFlush() {
  if (flushTimer) return;
  flushTimer = setTimeout(() => {
    flushTimer = null;
    flushStream('stdout');
    flushStream('stderr');
  }, FLUSH_MS);
}

function append(stream, text) {
  if (!text) return;
  const used = stream === 'stdout' ? stdoutBytes : stderrBytes;
  const room = OUTPUT_LIMIT - used;
  if (room <= 0) {
    truncated = true;
    return;
  }
  const bytes = encoder.encode(String(text));
  const value = decoder.decode(bytes.subarray(0, room), { stream: true });
  decoder.decode();
  const size = encoder.encode(value).length;
  if (size === 0) {
    if (bytes.length) truncated = true;
    return;
  }
  if (stream === 'stdout') {
    stdout += value;
    stdoutBytes += size;
  } else {
    stderr += value;
    stderrBytes += size;
  }
  if (bytes.length > room) truncated = true;
  buffers[stream] += value;
  if (encoder.encode(buffers[stream]).length >= FLUSH_BYTES)
    flushStream(stream);
  else scheduleFlush();
}

function setPhase(next) {
  phase = next;
  postMessage({ type: 'phase', id: runId, phase: next });
}

function cacheKey(value) {
  let a = 0x811c9dc5;
  let b = 0x01000193;
  for (let i = 0; i < value.length; i++) {
    const code = value.charCodeAt(i);
    a = Math.imul(a ^ code, 16777619) >>> 0;
    b = Math.imul(b ^ (code + i), 2246822519) >>> 0;
  }
  return `${COMPILER_VERSION}:${value.length}:${a.toString(16)}${b.toString(16)}`;
}

async function fetchModule(name) {
  const response = await fetch(new URL(name, base), { credentials: 'omit' });
  if (!response.ok) throw new Error('Runtime asset unavailable');
  const type = (response.headers.get('content-type') ?? '')
    .split(';')[0]
    .trim();
  if (type === 'application/wasm') {
    try {
      return await WebAssembly.compileStreaming(response.clone());
    } catch {
      // Fall through to the buffered path when the MIME type lies.
    }
  }
  return WebAssembly.compile(await response.arrayBuffer());
}

function loadModule(name) {
  let pendingModule = modules.get(name);
  if (!pendingModule) {
    pendingModule = fetchModule(name);
    modules.set(name, pendingModule);
  }
  return pendingModule;
}

async function readAsset(name) {
  const response = await fetch(new URL(name, base), { credentials: 'omit' });
  if (!response.ok) throw new Error('Runtime asset unavailable');
  return response.arrayBuffer();
}

function createAPI() {
  const instance = new API({
    readBuffer: async () => sysroot,
    compileStreaming: (name) => loadModule(name),
    hostWrite: (text) =>
      append(phase === 'compile' ? 'stderr' : 'stdout', text),
    sab
  });
  instance.hostLog = () => {};
  instance.hostLogAsync = async (_message, promise) => promise;
  return instance;
}

function applySAB(instance) {
  const memfs = instance.memfs;
  memfs.sab = interactive ? sab : null;
  memfs.sabMeta = memfs.sab ? new Int32Array(memfs.sab, 0, 2) : null;
  memfs.sabData = memfs.sab ? new Uint8Array(memfs.sab, 8) : null;
  memfs.sabMax = memfs.sab ? memfs.sab.byteLength - 8 : 0;
  memfs.sabActive = Boolean(memfs.sab);
  memfs.sabPending = null;
  memfs.sabPendingPos = 0;
  memfs.sabWaiting = false;
  memfs.sabBlocked = false;
}

function recall(key) {
  const module = compileCache.get(key);
  if (module) {
    compileCache.delete(key);
    compileCache.set(key, module);
  }
  return module;
}

function remember(key, module) {
  compileCache.set(key, module);
  while (compileCache.size > 4)
    compileCache.delete(compileCache.keys().next().value);
}

async function prefetch() {
  const names = ['clang', 'lld'];
  let loaded = 0;
  for (const name of names) {
    try {
      await loadModule(name);
    } catch {
      modules.delete(name);
    }
    loaded += 1;
    postMessage({ type: 'prefetch', loaded, total: names.length });
  }
}

async function initialize(message) {
  language = message.language;
  base = message.base;
  if (language === 'python') {
    importScripts('https://cdn.jsdelivr.net/pyodide/v0.25.0/full/pyodide.js');
    pyodide = await loadPyodide({
      indexURL: 'https://cdn.jsdelivr.net/pyodide/v0.25.0/full/'
    });
    postMessage({ type: 'ready' });
    return;
  }
  self.requestAnimationFrame = () => {
    throw new Error('Preview C hanya mendukung eksekusi sinkron.');
  };
  self.port = {
    postMessage: (data) => {
      if (data?.id === 'need-input' && runId !== null)
        postMessage({ type: 'input-request', id: runId });
    }
  };
  importScripts(new URL('shared.js', base).href);
  await loadModule('memfs');
  sysroot = await readAsset('sysroot.tar');
  api = createAPI();
  await api.ready;
  postMessage({ type: 'ready' });
  void prefetch();
}

async function runC(message) {
  const key = cacheKey(message.source);
  let module = recall(key);
  if (!module) {
    api = createAPI();
    applySAB(api);
    await api.ready;
    setPhase('compile');
    await api.compile({
      input: 'main.c',
      contents: message.source,
      obj: 'main.o'
    });
    await api.link('main.o', 'main.wasm');
    module = await WebAssembly.compile(api.memfs.getFileContents('main.wasm'));
    remember(key, module);
  } else {
    applySAB(api);
  }
  api.memfs.setStdinStr(message.stdin);
  applySAB(api);
  setPhase('run');
  await api.run(module, 'main.wasm');
}

function readInteractiveStdin() {
  if (!sabMeta || !sabData) return undefined;
  while (true) {
    const flag = Atomics.load(sabMeta, 0);
    if (flag === 1) break;
    if (flag === 2) return undefined;
    Atomics.wait(sabMeta, 0, 0);
  }
  const length = Atomics.load(sabMeta, 1);
  const bytes = sabData.slice(0, length);
  Atomics.store(sabMeta, 0, 0);
  Atomics.store(sabMeta, 1, 0);
  return decoder.decode(bytes);
}

async function runPython(message) {
  const lines = message.stdin.split('\n');
  if (lines.at(-1) === '') lines.pop();
  pyodide.setStdin({
    stdin: () =>
      interactive
        ? readInteractiveStdin()
        : lines.length
          ? lines.shift()
          : undefined
  });
  pyodide.setStdout({ batched: (text) => append('stdout', `${text}\n`) });
  pyodide.setStderr({ batched: (text) => append('stderr', `${text}\n`) });
  const globals = pyodide.runPython('dict(__name__="__main__")');
  try {
    await pyodide.runPythonAsync(message.source, { globals });
  } finally {
    globals.destroy();
  }
}

async function run(message) {
  stdout = '';
  stderr = '';
  stdoutBytes = 0;
  stderrBytes = 0;
  truncated = false;
  buffers = { stdout: '', stderr: '' };
  clearTimeout(flushTimer);
  flushTimer = null;
  runId = message.id;
  phase = 'run';
  interactive =
    message.interactive === true && message.sab instanceof SharedArrayBuffer;
  sab = interactive ? message.sab : null;
  sabMeta = sab ? new Int32Array(sab, 0, 2) : null;
  sabData = sab ? new Uint8Array(sab, 8) : null;
  const started = performance.now();
  let status = 'success';
  try {
    if (language === 'python') {
      await runPython(message);
    } else {
      await runC(message);
    }
  } catch (error) {
    status = phase === 'compile' ? 'compile_error' : 'runtime_error';
    append('stderr', String(error?.message ?? 'Program gagal.'));
  }
  flushAll();
  postMessage({
    type: 'result',
    id: message.id,
    result: {
      status,
      stdout,
      stderr,
      truncated,
      durationMs: Math.round(performance.now() - started)
    }
  });
  runId = null;
}

onmessage = async ({ data }) => {
  if (busy) return;
  if (data?.type !== 'init' && data?.type !== 'run') return;
  if (data.type === 'init') {
    busy = true;
    try {
      await initialize(data);
    } catch {
      postMessage({ type: 'init-error' });
    } finally {
      busy = false;
    }
    return;
  }
  busy = true;
  try {
    await run(data);
  } finally {
    busy = false;
  }
};
