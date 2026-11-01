<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { Play, Square } from 'lucide-svelte';
	import { sendCInput, cancelCInput } from '$lib/cInput';
	import { cWorkerStore, cWorkerRefStore, initRunner, resetCWorker, isCLoadingStore } from '$lib/stores/runner';
	import { get } from 'svelte/store';

	type Status = 'init' | 'loading' | 'ready' | 'running' | 'unsupported';

	let status = $state<Status>('init');
	let output = $state('');
	let needInput = $state(false);
	let inputValue = $state('');
	let inputEl: HTMLInputElement | undefined = $state();

	let code = $state(`#include <stdio.h>
int main(void) {
    char nama[64];
    printf("Siapa namamu? ");
    scanf("%63s", nama);
    int n;
    printf("Masukkan sebuah angka: ");
    scanf("%d", &n);
    printf("Halo %s, kuadratnya adalah %d\n", nama, n * n);
    return 0;
}
`);

	// Timer timeout run C (40 detik, pause saat need-input).
	let cRunTimer: ReturnType<typeof setTimeout> | undefined;
	const C_RUN_TIMEOUT_MS = 40000;

	function append(text: string) {
		output += text;
	}

	function injectStdoutUnbuffering(s: string): string {
		const mainRegex = /\bmain\s*\([^)]*\s*\)\s*\{/;
		if (!mainRegex.test(s)) return s;
		const suntikan = '$&\n    setbuf(stdout, NULL);\n    setbuf(stdin, NULL);';
		if (/#\s*include\s*[<"]stdio\.h[>"]/i.test(s)) {
			return s.replace(mainRegex, suntikan);
		}
		return `#include <stdio.h>\n${s.replace(mainRegex, suntikan)}`;
	}

	function setupWorker() {
		// SharedArrayBuffer butuh cross-origin isolation (header COOP/COEP).
		if (typeof SharedArrayBuffer === 'undefined' || !self.crossOriginIsolated) {
			status = 'unsupported';
			return;
		}
		status = 'loading';
		// initRunner membuat worker + SAB + retry; kita tunggu port siap.
		initRunner();
		const unsub = cWorkerStore.subscribe((port) => {
			if (port && status === 'loading') {
				status = 'ready';
				unsub();
			}
		});
		// Kalau init gagal (retry habis), isLoading jadi false & port tetap null.
		const unsubLoading = isCLoadingStore.subscribe((loading) => {
			if (!loading && status === 'loading') {
				status = 'unsupported';
				unsubLoading();
			}
		});
	}

	async function run() {
		if (status !== 'ready') return;
		const port = get(cWorkerStore);
		if (!port) return;

		output = '';
		needInput = false;
		status = 'running';

		const responseId = Math.floor(Math.random() * 1000000);
		let outBuffer = '';

		const startRunTimer = () => {
			clearTimeout(cRunTimer);
			cRunTimer = setTimeout(() => {
				needInput = false;
				append('\n[Program tidak merespons dalam 40 detik (kemungkinan infinite loop) — compiler di-reset.]\n');
				try {
					get(cWorkerRefStore)?.terminate();
				} catch { /* abaikan */ }
				resetCWorker();
				status = 'ready';
			}, C_RUN_TIMEOUT_MS);
		};
		const pauseRunTimer = () => clearTimeout(cRunTimer);

		const needInputHandler = (e: MessageEvent) => {
			if (e.data && e.data.id === 'need-input') {
				pauseRunTimer();
				needInput = true;
				queueMicrotask(() => inputEl?.focus());
			}
		};

		const compileHandler = (e: MessageEvent) => {
			const { id, data } = e.data;
			if (id === 'write') {
				outBuffer += data;
			} else if (id === 'runAsync' && e.data.responseId === responseId) {
				port.removeEventListener('message', compileHandler);

				if (!data.success) {
					port.removeEventListener('message', needInputHandler);
					append(outBuffer || data.error || 'Kompilasi gagal.');
					status = 'ready';
					return;
				}

				outBuffer = '';
				const runResponseId = responseId + 1;

				const runHandler = (e2: MessageEvent) => {
					const { id: id2, data: data2 } = e2.data;
					if (id2 === 'write') {
						append(data2);
						outBuffer += data2;
					} else if (id2 === 'runAsync' && e2.data.responseId === runResponseId) {
						port.removeEventListener('message', runHandler);
						port.removeEventListener('message', needInputHandler);

						if (data2.sabBlocked) {
							needInput = true;
							queueMicrotask(() => inputEl?.focus());
							return;
						}

						pauseRunTimer();
						needInput = false;

						if (data2.error) append('\n⚠ ' + data2.error + '\n');
						if (data2.waitingForInput) {
							needInput = true;
							queueMicrotask(() => inputEl?.focus());
							return;
						}
						if (!outBuffer.endsWith('\n') && outBuffer.length > 0) append('\n');
						append('\n✓ Program selesai!\n');
						status = 'ready';
					}
				};

				port.addEventListener('message', runHandler);
				startRunTimer();
				port.postMessage({ id: 'run', responseId: runResponseId, data: '' });
			}
		};

		port.addEventListener('message', needInputHandler);
		port.addEventListener('message', compileHandler);
		port.postMessage({ id: 'compile', responseId, data: injectStdoutUnbuffering(code), legacy: false });
	}

	function submitInput() {
		if (!needInput) return;
		const line = inputValue;
		append(line + '\n'); // echo ke terminal
		const ok = sendCInput(line);
		if (!ok) {
			needInput = false;
			append('\n[Mode input interaktif tidak tersedia (SAB mati).]\n');
		}
		inputValue = '';
		needInput = false;
	}

	function stop() {
		// Batalkan worker yang sedang Atomics.wait (mode SAB).
		cancelCInput();
		clearTimeout(cRunTimer);
		// Kalau hang, terminate + reset → re-init.
		if (status === 'running') {
			try {
				get(cWorkerRefStore)?.terminate();
			} catch { /* abaikan */ }
			resetCWorker();
		}
		needInput = false;
		status = 'init';
		setupWorker();
	}

	onMount(setupWorker);
	onDestroy(() => {
		clearTimeout(cRunTimer);
		cancelCInput();
		get(cWorkerRefStore)?.terminate();
	});
</script>

<div class="mx-auto max-w-4xl">
	<h1 class="mb-1 text-2xl font-bold text-ink-heading">Playground C (Live)</h1>
	<p class="mb-5 text-sm text-ink-caption">
		Tulis kode C, di-compile ke WebAssembly langsung di browser (clang-wasm),
		lalu jalan — termasuk <code>scanf</code> yang interaktif (SAB blocking stdin).
	</p>

	{#if status === 'unsupported'}
		<div class="rounded-lg bg-state-warning-bg p-4 text-sm text-state-warning">
			Browser ini belum mendukung mode interaktif (butuh <em>cross-origin isolation</em> /
			SharedArrayBuffer). Coba Chrome/Edge/Firefox terbaru via HTTPS dengan COOP/COEP aktif.
		</div>
	{:else}
		<div class="grid gap-4">
			<div>
				<label class="label" for="code">Kode C</label>
				<textarea
					id="code"
					bind:value={code}
					class="input min-h-48 font-mono text-sm"
					spellcheck="false"
				></textarea>
			</div>

			<div class="flex items-center gap-2">
				<button class="btn-primary" onclick={run} disabled={status !== 'ready'}>
					<Play size={16} />
					{status === 'loading'
						? 'Memuat runtime…'
						: status === 'running'
							? 'Compile & jalan…'
							: 'Run'}
				</button>
				<button class="btn-outline" onclick={stop} disabled={status === 'loading' || status === 'init'}>
					<Square size={14} /> Stop / Reset
				</button>
			</div>

			<div>
				<p class="mb-1 text-xs font-medium text-ink-caption">Output / Terminal:</p>
				<pre class="min-h-40 overflow-auto whitespace-pre-wrap rounded-lg bg-gray-900 p-3 text-sm text-gray-100">{output || '(output akan muncul di sini)'}</pre>
				{#if needInput}
					<form class="mt-2 flex items-center gap-2" onsubmit={(e) => { e.preventDefault(); submitInput(); }}>
						<span class="font-mono text-sm text-ink-caption">&gt;</span>
						<input
							bind:this={inputEl}
							bind:value={inputValue}
							class="input flex-1 font-mono text-sm"
							placeholder="ketik input lalu Enter…"
							autocomplete="off"
						/>
						<button class="btn-primary py-1.5" type="submit">Kirim</button>
					</form>
				{/if}
			</div>
		</div>
	{/if}
</div>
