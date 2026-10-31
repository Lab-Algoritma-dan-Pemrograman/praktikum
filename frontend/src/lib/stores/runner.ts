import { writable } from 'svelte/store';
import { browser } from '$app/environment';

export const pyodideWorkerStore = writable<Worker | null>(null);
export const isPyodideLoadingStore = writable<boolean>(true);

export const cWorkerStore = writable<MessagePort | null>(null);
export const isCLoadingStore = writable<boolean>(true);

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

	// Load Clang WASM Worker
	isCLoadingStore.set(true);
	try {
		console.log('Initializing Clang WASM Worker from static path...');
		const worker = new Worker('/clang-worker.js?v=7');
		const channel = new MessageChannel();
		const port = channel.port1;

		port.onmessage = (e) => {
			if (e.data.id === 'init_done') {
				console.log('Clang WASM Compiler initialized successfully');
				cWorkerStore.set(port);
				isCLoadingStore.set(false);
			} else if (e.data.id === 'init_error') {
				console.error('Failed to initialize Clang WASM Compiler:', e.data.error);
				isCLoadingStore.set(false);
			}
		};

		worker.postMessage({ id: 'constructor', data: channel.port2 }, [channel.port2]);
	} catch (e) {
		console.error('Failed to start Clang WASM Worker:', e);
		isCLoadingStore.set(false);
	}
}
