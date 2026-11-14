const COMPILER_VERSION = 'clang-8.0.1-wasm-r2';
const OUTPUT_LIMIT = 16384;
const IMAGE_LIMIT = 524288;
const IMAGE_COUNT = 4;
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
let busy = false;
let inputResolver = null;
let inputModuleReady = false;

let stdout = '';
let stderr = '';
let stdoutBytes = 0;
let stderrBytes = 0;
let truncated = false;
let images = [];
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
  memfs.sab = sab;
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

const PYTHON_PLOTS = `
import io as _plot_io
import base64 as _plot_b64
import __algohub_io as _terminal
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as _plt
_plt.close('all')
_original_show = _plt.show

def _show(*args, **kwargs):
    for number in _plt.get_fignums():
        fig = _plt.figure(number)
        try:
            buf = _plot_io.BytesIO()
            fig.savefig(buf, format='png', bbox_inches='tight')
            _terminal.image('data:image/png;base64,' + _plot_b64.b64encode(buf.getvalue()).decode('ascii'))
        finally:
            _plt.close(fig)

_plt.show = _show
`;

const PYTHON_INTERACTIVE = `
import ast as _ast
import builtins as _builtins
import inspect as _inspect
import __algohub_io as _terminal

class _InputCalls(_ast.NodeTransformer):
    def visit_FunctionDef(self, node):
        return node
    def visit_Lambda(self, node):
        return node
    def visit_ClassDef(self, node):
        return node
    def visit_Call(self, node):
        self.generic_visit(node)
        if isinstance(node.func, _ast.Name) and node.func.id == 'input':
            call = _ast.Call(func=_ast.Name(id='_async_input', ctx=_ast.Load()), args=[node.func, *node.args], keywords=node.keywords)
            return _ast.copy_location(_ast.Await(value=call), node)
        return node

async def _async_input(function, *args, **kwargs):
    if function is not _builtins.input:
        return function(*args, **kwargs)
    if kwargs or len(args) > 1:
        raise TypeError('input() accepts at most one positional prompt')
    return await _terminal.readLine(str(args[0]) if args else '')

_tree = _InputCalls().visit(_ast.parse(_source, filename='preview.py'))
_code = compile(_ast.fix_missing_locations(_tree), 'preview.py', 'exec', flags=_ast.PyCF_ALLOW_TOP_LEVEL_AWAIT, dont_inherit=True)
_result = eval(_code, globals())
if _inspect.isawaitable(_result):
    await _result
`;

function ensurePythonIO() {
  if (inputModuleReady) return;
  pyodide.registerJsModule('__algohub_io', {
    readLine: (prompt = '') =>
      new Promise((resolve) => {
        flushAll();
        inputResolver = resolve;
        postMessage({
          type: 'input-request',
          id: runId,
          prompt: String(prompt ?? '')
        });
      }),
    image: (value) => {
      const url = String(value ?? '');
      if (
        !url.startsWith('data:image/png;base64,') ||
        url.length > IMAGE_LIMIT ||
        images.length >= IMAGE_COUNT
      ) {
        truncated = true;
        return;
      }
      images.push(url);
      postMessage({ type: 'image', id: runId, data: url });
    }
  });
  inputModuleReady = true;
}

async function runPython(message) {
  postMessage({ type: 'loading', id: runId });
  const globals = pyodide.runPython('dict(__name__="__main__")');
  const plots = pyodide.runPython('dict()');
  let plotsReady = false;
  try {
    await pyodide.loadPackagesFromImports(message.source);
    ensurePythonIO();
    if (Object.hasOwn(pyodide.loadedPackages, 'matplotlib')) {
      pyodide.runPython(PYTHON_PLOTS, { globals: plots });
      plotsReady = true;
    }
    postMessage({ type: 'loaded', id: runId });
    pyodide.setStdout({ batched: (text) => append('stdout', `${text}\n`) });
    pyodide.setStderr({ batched: (text) => append('stderr', `${text}\n`) });
    const lines = message.stdin.split('\n');
    if (lines.at(-1) === '') lines.pop();
    pyodide.setStdin({
      stdin: () => {
        if (interactive)
          throw new Error(
            'Input interaktif hanya untuk kode tingkat atas. Untuk fungsi sinkron, nonaktifkan mode interaktif dan isi input standar.'
          );
        return lines.length ? lines.shift() : undefined;
      }
    });
    if (interactive) globals.set('_source', message.source);
    await pyodide.runPythonAsync(
      interactive ? PYTHON_INTERACTIVE : message.source,
      { globals }
    );
    if (plotsReady) pyodide.runPython('_show()', { globals: plots });
  } finally {
    try {
      if (plotsReady)
        pyodide.runPython("_plt.close('all')\n_plt.show = _original_show", {
          globals: plots
        });
    } finally {
      globals.destroy();
      plots.destroy();
      inputResolver = null;
    }
  }
}

async function run(message) {
  stdout = '';
  stderr = '';
  stdoutBytes = 0;
  stderrBytes = 0;
  truncated = false;
  images = [];
  inputResolver = null;
  buffers = { stdout: '', stderr: '' };
  clearTimeout(flushTimer);
  flushTimer = null;
  runId = message.id;
  phase = 'run';
  interactive = message.interactive === true;
  sab =
    typeof SharedArrayBuffer === 'function' &&
    message.sab instanceof SharedArrayBuffer
      ? message.sab
      : null;
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
      images,
      durationMs: Math.round(performance.now() - started)
    }
  });
  runId = null;
}

onmessage = async ({ data }) => {
  if (data?.type === 'input') {
    if (inputResolver && data.id === runId) {
      const resolve = inputResolver;
      inputResolver = null;
      resolve(String(data.value ?? '').replace(/\r?\n$/, ''));
    }
    return;
  }
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
