let language;
let base;
let pyodide;
let modules;
let sysroot;
let busy = false;
let stdout = '';
let stderr = '';
let truncated = false;
let stdoutBytes = 0;
let stderrBytes = 0;
const encoder = new TextEncoder();
const decoder = new TextDecoder();

function append(target, text) {
  const available = 16384 - (target === 'stdout' ? stdoutBytes : stderrBytes);
  if (available <= 0) {
    truncated = true;
    return;
  }
  const bytes = encoder.encode(String(text).slice(0, 16385));
  const value = decoder.decode(bytes.subarray(0, available), { stream: true });
  decoder.decode();
  truncated = truncated || bytes.length > available;
  const size = encoder.encode(value).length;
  if (target === 'stdout') {
    stdout += value;
    stdoutBytes += size;
  } else {
    stderr += value;
    stderrBytes += size;
  }
}

async function read(name) {
  const response = await fetch(new URL(`clang/${name}`, base), {
    credentials: 'omit'
  });
  if (!response.ok) throw new Error('Runtime asset unavailable');
  return response.arrayBuffer();
}

async function initialize(message) {
  language = message.language;
  base = message.base;
  if (language === 'python') {
    importScripts('https://cdn.jsdelivr.net/pyodide/v0.25.0/full/pyodide.js');
    pyodide = await loadPyodide({
      indexURL: 'https://cdn.jsdelivr.net/pyodide/v0.25.0/full/'
    });
  } else {
    self.requestAnimationFrame = () => {
      throw new Error('Preview C hanya mendukung eksekusi sinkron.');
    };
    importScripts(new URL('clang/shared.js', base).href);
    const names = ['clang', 'lld', 'memfs'];
    const compiled = await Promise.all(
      names.map(async (name) => [
        name,
        await WebAssembly.compile(await read(name))
      ])
    );
    modules = new Map(compiled);
    sysroot = await read('sysroot.tar');
  }
  postMessage({ type: 'ready' });
}

async function run(message) {
  stdout = '';
  stderr = '';
  stdoutBytes = 0;
  stderrBytes = 0;
  truncated = false;
  const started = performance.now();
  let status = 'success';
  let phase = 'run';
  try {
    if (language === 'python') {
      const lines = message.stdin.split('\n');
      if (lines.at(-1) === '') lines.pop();
      pyodide.setStdin({
        stdin: () => (lines.length ? lines.shift() : undefined)
      });
      pyodide.setStdout({ batched: (text) => append('stdout', `${text}\n`) });
      pyodide.setStderr({ batched: (text) => append('stderr', `${text}\n`) });
      const globals = pyodide.runPython('dict(__name__="__main__")');
      try {
        await pyodide.runPythonAsync(message.source, { globals });
      } finally {
        globals.destroy();
      }
    } else {
      phase = 'compile';
      const api = new API({
        readBuffer: async () => sysroot,
        compileStreaming: async (name) => modules.get(name),
        hostWrite: (text) =>
          append(phase === 'compile' ? 'stderr' : 'stdout', text)
      });
      api.hostLog = () => {};
      api.hostLogAsync = async (_message, promise) => promise;
      await api.ready;
      await api.compile({
        input: 'main.c',
        contents: message.source,
        obj: 'main.o'
      });
      await api.link('main.o', 'main.wasm');
      const program = await WebAssembly.compile(
        api.memfs.getFileContents('main.wasm')
      );
      api.memfs.setStdinStr(message.stdin);
      phase = 'run';
      await api.run(program, 'main.wasm');
    }
  } catch (error) {
    status = phase === 'compile' ? 'compile_error' : 'runtime_error';
    append('stderr', String(error?.message ?? 'Program gagal.'));
  }
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
}

onmessage = async ({ data }) => {
  if (busy) return;
  busy = true;
  try {
    if (data?.type === 'init') await initialize(data);
    else if (data?.type === 'run') await run(data);
  } catch {
    postMessage({ type: 'init-error' });
  } finally {
    busy = false;
  }
};
