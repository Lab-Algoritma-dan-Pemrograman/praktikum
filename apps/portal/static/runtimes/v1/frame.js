const base = new URL('.', location.href).href;
const policy = document.createElement('meta');
policy.httpEquiv = 'Content-Security-Policy';
policy.content = `default-src 'none'; script-src 'wasm-unsafe-eval' ${base} https://cdn.jsdelivr.net/pyodide/v0.25.0/full/; connect-src ${base} https://cdn.jsdelivr.net/pyodide/v0.25.0/full/; worker-src blob:; base-uri 'none'; form-action 'none'`;
document.head.append(policy);
let port;
let worker;
let language;
let ready = false;
let pending;

function stop() {
  worker?.terminate();
  worker = undefined;
  ready = false;
  pending = undefined;
}

function execute() {
  if (!pending || !ready) return;
  port.postMessage({ type: 'executing', id: pending.id });
  worker.postMessage({ type: 'run', ...pending });
}

function start(nextLanguage) {
  const blob = new Blob(
    [`importScripts(${JSON.stringify(new URL('worker.js', base).href)});`],
    { type: 'text/javascript' }
  );
  const url = URL.createObjectURL(blob);
  worker = new Worker(url);
  URL.revokeObjectURL(url);
  language = nextLanguage;
  worker.onmessage = ({ data }) => {
    if (data?.type === 'ready') {
      ready = true;
      execute();
    } else if (pending && data?.type === 'result' && data.id === pending.id) {
      const id = pending.id;
      if (language === 'python') stop();
      else pending = undefined;
      port.postMessage({ type: 'result', id, result: data.result });
    } else if (data?.type === 'init-error' && pending) {
      port.postMessage({ type: 'failed', id: pending.id });
      stop();
    }
  };
  worker.onerror = () => {
    if (pending) port.postMessage({ type: 'failed', id: pending.id });
    stop();
  };
  worker.postMessage({ type: 'init', language, base });
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
    if (data?.type === 'cancel') {
      if (pending?.id === data.id) stop();
      return;
    }
    if (
      pending ||
      data?.type !== 'run' ||
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
      new TextEncoder().encode(data.source).length > 65536 ||
      new TextEncoder().encode(data.stdin).length > 16384
    )
      return;
    if (language !== data.language) stop();
    pending = data;
    if (!worker) start(data.language);
    else execute();
  };
  port.postMessage({ type: 'connected' });
});
