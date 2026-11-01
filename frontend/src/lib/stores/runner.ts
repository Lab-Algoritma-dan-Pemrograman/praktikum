import { writable } from 'svelte/store';
import { browser } from '$app/environment';

export const pyodideWorkerStore = writable<Worker | null>(null);
export const isPyodideLoadingStore = writable<boolean>(true);

export const cWorkerStore = writable<MessagePort | null>(null);
// Referensi worker asli (bukan port) — dipakai untuk terminate saat
// timeout/reset (satu-satunya cara menghentikan wasm yang infinite loop).
export const cWorkerRefStore = writable<Worker | null>(null);
// SharedArrayBuffer untuk blocking stdin interaktif (mode SAB).
// null kalau halaman tidak cross-origin isolated (fallback ke EOF lama).
export const cSabStore = writable<SharedArrayBuffer | null>(null);
export const isCLoadingStore = writable<boolean>(true);

// Query version untuk memaksa browser mengambil worker baru (cache immutable).
const CLANG_WORKER_VERSION = 'v=8';
const SAB_SIZE = 4096; // cukup untuk satu baris input C
const MAX_RETRIES = 2;
const RETRY_DELAY_MS = 1500;

// Dipanggil dari layout DAN dari CodeEditor. Tanpa penjaga, setiap panggilan
// membuat set Worker baru dan menimpa cWorkerStore; 'compile' lalu mendarat di
// worker lama sementara 'run' dikirim ke worker baru -> "Please compile first".
let runnerStarted = false;

export function initRunner() {
	if (!browser || runnerStarted) return;
	runnerStarted = true;

	// Load Pyodide Worker
	isPyodideLoadingStore.set(true);
	try {
		console.log('Initializing Pyodide Worker from static path...');
		const worker = new Worker('/pyodide.worker.js');
		worker.onmessage = (e) => {
			if (e.data.type === 'INIT_DONE') {
				console.log('Pyodide Worker initialized successfully');
				pyodideWorkerStore.set(worker);
				isPyodideLoadingStore.set(false);
			} else if (e.data.type === 'INIT_ERROR') {
				console.error('Failed to initialize Pyodide in worker:', e.data.error);
				isPyodideLoadingStore.set(false);
			}
		};
		worker.postMessage({ type: 'INIT' });
	} catch (e) {
		console.error('Failed to start Pyodide Worker:', e);
		isPyodideLoadingStore.set(false);
	}

	// Load Clang WASM Worker (dengan SAB + retry)
	isCLoadingStore.set(true);
	const tryInit = (attempt: number) => {
		// SAB hanya tersedia kalau halaman cross-origin isolated (COOP+COEP).
		// Kalau tidak, worker fallback ke mode EOF lama (legacy).
		let localSab: SharedArrayBuffer | null = null;
		try {
			if (typeof SharedArrayBuffer !== 'undefined' && self.crossOriginIsolated) {
				localSab = new SharedArrayBuffer(SAB_SIZE);
			}
		} catch {
			localSab = null;
		}
		cSabStore.set(localSab);

		console.log(`Initializing Clang WASM Worker from static path (attempt ${attempt + 1})...`);
		let worker: Worker;
		try {
			worker = new Worker(`/clang-worker.js?${CLANG_WORKER_VERSION}`);
		} catch (e) {
			console.error('Failed to construct Clang Worker:', e);
			isCLoadingStore.set(false);
			return;
		}

		const channel = new MessageChannel();
		const port = channel.port1;

		worker.onerror = (e) => {
			console.error('Clang Worker error:', e.message || e);
			worker?.terminate();
			if (attempt < MAX_RETRIES) {
				setTimeout(() => tryInit(attempt + 1), RETRY_DELAY_MS);
			} else {
				isCLoadingStore.set(false);
			}
		};

		port.onmessage = (e) => {
			if (e.data.id === 'init_done') {
				console.log('Clang WASM Compiler initialized successfully');
				cWorkerRefStore.set(worker);
				cWorkerStore.set(port);
				isCLoadingStore.set(false);
			} else if (e.data.id === 'init_error') {
				console.error('Failed to initialize Clang WASM Compiler:', e.data.error);
				worker?.terminate();
				if (attempt < MAX_RETRIES) {
					setTimeout(() => tryInit(attempt + 1), RETRY_DELAY_MS);
				} else {
					isCLoadingStore.set(false);
				}
			}
		};

		// SAB dikirim by value (bukan transfer list) — SharedArrayBuffer tidak
		// boleh masuk transfer list; hanya MessagePort yang di-transfer.
		if (localSab) {
			worker.postMessage({ id: 'constructor', data: channel.port2, sab: localSab }, [channel.port2]);
		} else {
			worker.postMessage({ id: 'constructor', data: channel.port2 }, [channel.port2]);
		}
	};

	tryInit(0);
}

// Reset worker (dipakai setelah terminate saat timeout/reset) supaya
// initializer bisa menciptakan worker baru. `runnerStarted` di-reset juga —
// tanpa itu, initRunner() tidak akan pernah membuat worker baru (guard).
export function resetCWorker() {
	cWorkerStore.set(null);
	cWorkerRefStore.set(null);
	cSabStore.set(null);
	isCLoadingStore.set(true);
	runnerStarted = false;
}
