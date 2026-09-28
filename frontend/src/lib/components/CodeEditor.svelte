<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { api } from '$lib/api';
	import { browser } from '$app/environment';
	import { Play, Square, RefreshCw } from 'lucide-svelte';
	import {
		initRunner,
		pyodideWorkerStore,
		isPyodideLoadingStore,
		cWorkerStore,
		isCLoadingStore
	} from '$lib/stores/runner';

	let {
		value = $bindable(''),
		language = 'c',
		readonly = false,
		height = '350px',
		runnable = false,
		oninput = undefined
	} = $props();

	let el: HTMLDivElement;
	let termEl: HTMLDivElement;
	let editor: any = null;
	let monacoRef: any = null;
	let term: any = null;
	let fitAddon: any = null;
	let resizeObserver: ResizeObserver | null = null;

	// Mirror output teks ke DOM: xterm menggambar ke canvas (tak terbaca assistive tech),
	// jadi tulis juga ke live-region agar output bisa dibaca & diverifikasi.
	let outputText = $state('');
	function stripAnsi(str: string): string {
		return str.replace(/\x1b\[[0-9;?]*[A-Za-z]/g, '');
	}
	function w(str: string) {
		term?.write(str);
		outputText += stripAnsi(str);
	}
	function wl(str: string) {
		term?.writeln(str);
		outputText += stripAnsi(str) + '\n';
	}

	// Run state
	let runLang = $state('c');
	let loadError = $state('');
	let running = $state(false);
	let fallbackMode = $state(false);
	let statusText = $state('');

	// Worker WASM itu berat (clang ~30 MB + lld ~19 MB + sysroot ~9 MB). Jangan
	// diunduh begitu halaman dibuka — mulai hanya saat pengguna menyentuh editor
	// atau menekan Run, supaya halaman publik tidak membebani pengunjung.
	let runnerStarted = false;

	function ensureRunner() {
		if (runnerStarted || !browser) return;
		runnerStarted = true;
		initRunner();
	}

	function workerReady(lang: string): boolean {
		return lang === 'python' ? $pyodideWorkerStore !== null : $cWorkerStore !== null;
	}

	// Tunggu worker selesai dimuat (unduhan pertama bisa lama) tanpa memblokir UI.
	function tungguWorker(lang: string, ms = 180000): Promise<boolean> {
		return new Promise((resolve) => {
			if (workerReady(lang)) return resolve(true);
			const mulai = Date.now();
			const timer = setInterval(() => {
				if (workerReady(lang)) {
					clearInterval(timer);
					resolve(true);
				} else if (Date.now() - mulai > ms) {
					clearInterval(timer);
					resolve(false);
				}
			}, 400);
		});
	}

	// Python interactive input state
	let inputBuffer = '';
	let waitingForInput = $state(false);

	// C iterative input state
	let cStdinMode = false;
	let cStdinLines: string[] = [];
	let cStdinBuffer = '';
	let cStdinOffsets: number[] = [];

	function injectStdoutUnbuffering(code: string): string {
		const mainRegex = /\bmain\s*\([^)]*\)\s*\{/;
		if (mainRegex.test(code)) {
			return code.replace(mainRegex, '$&\n    setbuf(stdout, NULL);\n    setbuf(stdin, NULL);');
		}
		return code;
	}

	onMount(async () => {
		if (language === 'python') runLang = 'python';

		// Initialize Monaco (diunduh dari CDN saat runtime). Kalau CDN/loader
		// gagal, tampilkan fallback error yang jelas supaya pengguna tahu kenapa
		// editor kosong (bukan diam tanpa penjelasan).
		const loader = (await import('@monaco-editor/loader')).default;
		let monaco: any;
		try {
			monaco = await loader.init();
		} catch {
			loadError = 'Editor kode gagal dimuat (CDN terblokir di jaringan/ISP Anda). Coba muat ulang halaman atau gunakan koneksi lain.';
			return;
		}
		if (!monaco?.editor) {
			loadError = 'Editor kode gagal dimuat. Coba muat ulang halaman.';
			return;
		}
		monacoRef = monaco;
		try {
			editor = monaco.editor.create(el, {
				value,
				language: runLang,
				readOnly: readonly,
				automaticLayout: true,
				minimap: { enabled: false },
				fontSize: 14,
				scrollBeyondLastLine: false,
				theme: 'vs-dark'
			});
		} catch {
			loadError = 'Editor kode gagal dibuat. Coba muat ulang halaman.';
			return;
		}
		editor.onDidChangeModelContent(() => {
			value = editor.getValue();
			oninput?.();
		});

		editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter, () => {
			runCode();
		});

		// Terminal diinisialisasi terpisah dari Monaco: kegagalan xterm atau
		// return dini karena CDN Monaco gagal tidak boleh membunuh inisialisasi
		// terminal. Keduanya jalan sendiri apa pun yang terjadi pada yang lain.
		await initTerminal();
	});

	// Inisialisasi terminal dipisah supaya tidak ikut mati bila Monaco gagal.
	async function initTerminal() {
		if (!runnable || !termEl) return;
		try {
			const [{ Terminal }, { FitAddon }] = await Promise.all([
				import('@xterm/xterm'),
				import('@xterm/addon-fit')
			]);
			await import('@xterm/xterm/css/xterm.css');
			term = new Terminal({
				theme: {
					background: '#18181b',
					foreground: '#e4e4e7',
					cursor: '#a1a1aa',
					cursorAccent: '#18181b',
					selectionBackground: '#3f3f4680',
					green: '#4ade80',
					red: '#f87171',
					yellow: '#facc15',
					blue: '#60a5fa',
					magenta: '#c084fc',
					cyan: '#22d3ee'
				},
				fontSize: 13,
				fontFamily: '"JetBrains Mono", "Fira Code", monospace',
				cursorBlink: true,
				cursorStyle: 'bar',
				convertEol: true,
				disableStdin: false,
				scrollback: 5000
			});

			fitAddon = new FitAddon();
			term.loadAddon(fitAddon);
			term.open(termEl);
			fitAddon.fit();

			wl('\x1b[2mKlik RUN untuk menguji kode Anda...\x1b[0m');

			term.onKey(({ key, domEvent }) => {
				const keyCode = domEvent.keyCode;
				if (!term || !running) return;

				if (runLang === 'python' && waitingForInput) {
					if (keyCode === 13) { // Enter
						waitingForInput = false;
						w('\r\n');
						const inputValue = inputBuffer;
						inputBuffer = '';
						if ($pyodideWorkerStore) {
							$pyodideWorkerStore.postMessage({ type: 'INPUT_RESPONSE', value: inputValue });
						}
					} else if (keyCode === 8) { // Backspace
						if (inputBuffer.length > 0) {
							inputBuffer = inputBuffer.slice(0, -1);
							w('\b \b');
						}
					} else if (!domEvent.ctrlKey && !domEvent.altKey && !domEvent.metaKey) {
						inputBuffer += key;
						w(key);
					}
				} else if (runLang === 'c' && cStdinMode) {
					if (keyCode === 13) { // Enter
						w('\r\n');
						cStdinLines.push(cStdinBuffer);
						cStdinBuffer = '';
						const accumulatedStdin = cStdinLines.join('\n') + '\n';
						executeCCode(accumulatedStdin);
					} else if (keyCode === 8) { // Backspace
						if (cStdinBuffer.length > 0) {
							cStdinBuffer = cStdinBuffer.slice(0, -1);
							w('\b \b');
						}
					} else if (!domEvent.ctrlKey && !domEvent.altKey && !domEvent.metaKey) {
						cStdinBuffer += key;
						w(key);
					}
				}
			});

			const resizeObserverLocal = new ResizeObserver(() => fitAddon?.fit());
			resizeObserverLocal.observe(termEl);
			resizeObserver = resizeObserverLocal;

			// Mulai unduh compiler begitu pengguna menyentuh editor/terminal, supaya
			// klik Run pertama tidak terasa menggantung. Dipasang di sini (bukan di
			// $effect) karena dijalankan sekali setelah elemen benar-benar ada.
			const mulai = () => ensureRunner();
			el?.addEventListener('pointerdown', mulai, { once: true });
			el?.addEventListener('keydown', mulai, { once: true });
			termEl?.addEventListener('pointerdown', mulai, { once: true });
		} catch (e) {
			console.error('Terminal gagal diinisialisasi:', e);
		}
	}

	$effect(() => {
		// `value` WAJIB dibaca tanpa syarat di baris pertama: kalau dibaca setelah
		// guard `editor &&` (yang bernilai false saat init), Svelte tidak pernah
		// mencatatnya sebagai dependency dan effect ini tak akan jalan lagi —
		// akibatnya ganti bahasa di halaman induk tidak mengganti isi editor.
		const v = value;
		if (editor && v !== editor.getValue()) {
			editor.setValue(v ?? '');
		}
	});

	// `readOnly` hanya dipasang sekali saat editor dibuat, jadi kunci yang dilepas
	// asisten (mis. course dibuka ulang) tidak berpengaruh sampai halaman dimuat
	// ulang. Sinkronkan tiap prop berubah.
	$effect(() => {
		const ro = readonly;
		if (editor) editor.updateOptions({ readOnly: ro });
	});

	// Prop `language` dari halaman induk wajib menggerakkan mode editor + pilihan
	// bahasa toolbar. Tanpa sinkronisasi ini halaman bisa menampilkan kode Python
	// sementara kompilatornya tetap C — gejalanya error "C++ requires a type
	// specifier" saat Run, tepat saat pengguna menekan tombol Python.
	//
	// Hanya bereaksi saat PROP berubah, bukan saat pengguna mengubah select
	// internal: halaman praktikum mengirim language="c" tetap, jadi tanpa penjaga
	// ini pilihan Python dari select akan langsung dipaksa balik ke C.
	let lastLangProp: string | undefined = undefined;
	$effect(() => {
		if (language === lastLangProp) return;
		lastLangProp = language;
		const want = language === 'python' ? 'python' : 'c';
		if (want !== runLang) {
			runLang = want;
			onLangChange();
		}
	});

	function onLangChange() {
		if (monacoRef && editor) {
			monacoRef.editor.setModelLanguage(editor.getModel(), runLang);
		}
		clearTerminal();
	}

	function clearTerminal() {
		term?.clear();
		outputText = '';
		wl('\x1b[2mKlik RUN untuk menguji kode Anda...\x1b[0m');
		running = false;
		waitingForInput = false;
		cStdinMode = false;
		inputBuffer = '';
		cStdinBuffer = '';
		cStdinLines = [];
		cStdinOffsets = [];
	}

	async function runCode() {
		if (!term || running) return;

		// Kompilator baru diunduh saat pertama kali dibutuhkan.
		ensureRunner();

		term.clear();
		outputText = '';
		running = true;
		fallbackMode = false;
		statusText = '';

		const lang = runLang;
		if (!workerReady(lang)) {
			const info = lang === 'python' ? 'Python (Pyodide)' : 'compiler C (clang+wasi)';
			statusText = `Menyiapkan ${info}… unduhan pertama bisa sampai ±1 menit.`;
			await tungguWorker(lang).then(async (siap) => {
				statusText = '';
				if (siap) {
					running = false;
					await runCode();
				} else {
					running = false;
					runServerFallback();
				}
			});
			return;
		}

		if (runLang === 'python') {
			runPythonInteractive();
		} else {
			cStdinLines = [];
			cStdinBuffer = '';
			cStdinOffsets = [];
			executeCCode('');
		}
	}

	async function runPythonInteractive() {
		if (!term || !$pyodideWorkerStore) return;

		let lastOutputLength = 0;
		const id = Date.now().toString() + Math.random().toString();

		const handler = (e: MessageEvent) => {
			if (e.data.id !== id || !term) return;

			if (e.data.type === 'INPUT_REQUEST') {
				const outputVal = e.data.output || '';
				if (outputVal.length > lastOutputLength) {
					w(outputVal.slice(lastOutputLength));
					lastOutputLength = outputVal.length;
				}
				const prompt = e.data.prompt || '';
				if (prompt) {
					w(prompt);
				}
				waitingForInput = true;
				inputBuffer = '';
			} else if (e.data.type === 'RUN_DONE') {
				$pyodideWorkerStore.removeEventListener('message', handler);
				const outputVal = e.data.output || '';
				if (outputVal.length > lastOutputLength) {
					w(outputVal.slice(lastOutputLength));
				}
				wl('');
				wl('\r\n\x1b[1;32m✓ Program selesai!\x1b[0m');
				running = false;
			} else if (e.data.type === 'RUN_ERROR') {
				$pyodideWorkerStore.removeEventListener('message', handler);
				wl(`\r\n\x1b[1;31m❌ Error: ${e.data.error}\x1b[0m`);
				running = false;
			} else if (e.data.type === 'RUN_CANCELLED') {
				$pyodideWorkerStore.removeEventListener('message', handler);
				wl('\r\n\x1b[1;33m⚠ Program dibatalkan.\x1b[0m');
				running = false;
			}
		};

		$pyodideWorkerStore.addEventListener('message', handler);
		$pyodideWorkerStore.postMessage({ type: 'RUN_INTERACTIVE', code: value, id });

		// Attach cleanup function to terminal for cancel actions
		(term as any).__cleanup = () => {
			$pyodideWorkerStore.removeEventListener('message', handler);
			$pyodideWorkerStore.postMessage({ type: 'CANCEL' });
		};
	}

	async function executeCCode(accumulatedStdin: string) {
		if (!term || !$cWorkerStore) return;

		const responseId = Math.floor(Math.random() * 1000000);
		let compileOutputBuffer = '';

		const compileHandler = (e: MessageEvent) => {
			const { id, data } = e.data;
			if (!term) return;

			if (id === 'write') {
				compileOutputBuffer += data;
			} else if (id === 'runAsync' && e.data.responseId === responseId) {
				$cWorkerStore.removeEventListener('message', compileHandler);

				if (!data.success) {
					cStdinMode = false;
					term.clear();
		outputText = '';
					wl('❌ \x1b[1;31mCompilation Error:\x1b[0m');
					w(`\x1b[31m${compileOutputBuffer || data.error || 'Gagal melakukan kompilasi.'}\x1b[0m\r\n`);
					running = false;
					return;
				}

				// Compile success, run the executable
				compileOutputBuffer = '';
				const runResponseId = responseId + 1;

				const runHandler = (e2: MessageEvent) => {
					const { id: id2, data: data2 } = e2.data;
					if (!term) return;

					if (id2 === 'write') {
						compileOutputBuffer += data2;
					} else if (id2 === 'runAsync' && e2.data.responseId === runResponseId) {
						$cWorkerStore.removeEventListener('message', runHandler);

						term.clear();
		outputText = '';

						if (data2.waitingForInput && data2.stdoutLenAtInputRequest !== undefined) {
							const newOffset = data2.stdoutLenAtInputRequest;
							if (!cStdinOffsets.includes(newOffset)) {
								cStdinOffsets.push(newOffset);
							}
						}

						let out = compileOutputBuffer || '';
						if (data2.waitingForInput && data2.stdoutLenAtInputRequest !== undefined) {
							out = out.slice(0, data2.stdoutLenAtInputRequest);
						}

						let displayOutput = '';
						let lastOffset = 0;
						for (let i = 0; i < cStdinOffsets.length; i++) {
							const offset = cStdinOffsets[i];
							displayOutput += out.slice(lastOffset, offset);
							if (i < cStdinLines.length) {
								displayOutput += cStdinLines[i] + '\n';
							}
							lastOffset = offset;
						}
						displayOutput += out.slice(lastOffset);

						w(displayOutput);

						if (data2.error) {
							w(`\r\n\x1b[33m⚠ ${data2.error}\x1b[0m\r\n`);
						}

						if (data2.waitingForInput) {
							cStdinMode = true;
							cStdinBuffer = '';
						} else {
							cStdinMode = false;
							if (!out.endsWith('\n') && out.length > 0) wl('');
							wl('\r\n\x1b[1;32m✓ Program selesai!\x1b[0m');
							running = false;
						}
					}
				};

				$cWorkerStore.addEventListener('message', runHandler);
				$cWorkerStore.postMessage({ id: 'run', responseId: runResponseId, data: accumulatedStdin });
			}
		};

		$cWorkerStore.addEventListener('message', compileHandler);
		$cWorkerStore.postMessage({ id: 'compile', responseId, data: injectStdoutUnbuffering(value) });
	}

	async function runServerFallback() {
		// Hanya untuk pemakai yang sudah login. /api/praktikum/run butuh Bearer token,
		// dan api.ts akan melempar 401 ke /praktikum/login — jangan sampai itu terjadi
		// pada pengunjung publik di /info/praktikum.
		if (!browser || !localStorage.getItem('token')) {
			statusText = '';
			wl('\x1b[33m⚠ Compiler belum siap di browser ini. Coba klik Run sekali lagi, atau muat ulang halaman.\x1b[0m');
			running = false;
			return;
		}
		if (!term) return;
		fallbackMode = true;

		try {
			const result = await api.post<{ stdout: string; stderr: string; error: string }>(
				'/api/praktikum/run',
				{ language: runLang, source: value, stdin: '' }
			);

			if (result.stdout) w(result.stdout);
			if (result.stderr) w(`\x1b[31m${result.stderr}\x1b[0m`);
			if (result.error) w(`\r\n\x1b[1;31m❌ Error: ${result.error}\x1b[0m`);
			wl('\r\n\x1b[1;32m✓ Program selesai!\x1b[0m');
		} catch (e) {
			wl(`\r\n\x1b[1;31m❌ Connection Error: ${(e as Error).message}\x1b[0m`);
		} finally {
			running = false;
		}
	}

	function stopCode() {
		if (term && (term as any).__cleanup) {
			(term as any).__cleanup();
			(term as any).__cleanup = null;
		}
		statusText = '';
		clearTerminal();
		wl('\x1b[1;31m⚠ Eksekusi dihentikan oleh pengguna.\x1b[0m');
	}

	onDestroy(() => {
		resizeObserver?.disconnect();
		term?.dispose();
		editor?.dispose();
	});
</script>

<!-- Editor Container with Toolbar integrated at the top -->
<div class="flex flex-col border border-zinc-800 rounded-xl overflow-hidden shadow-sm mb-6">
	<!-- Editor Toolbar -->
	<div class="flex flex-wrap items-center justify-between gap-3 p-3 bg-zinc-900 border-b border-zinc-800 select-none">
		<div class="flex items-center gap-2">
			{#if runnable}
				<select bind:value={runLang} onchange={onLangChange} class="h-8 rounded-lg bg-zinc-800 border border-zinc-700 text-zinc-100 px-2.5 py-1 text-xs font-bold font-mono outline-none cursor-pointer focus:border-zinc-500">
					<option value="c">main.c</option>
					<option value="python">main.py</option>
				</select>
				
				{#if !running}
					<button class="h-8 bg-primary hover:bg-primary/95 text-white px-3 py-1 rounded-lg text-xs font-bold transition-all flex items-center gap-1.5 shadow-sm" onclick={runCode}>
						<Play size={12} /> Run <span class="text-[9px] text-white/50 font-normal">Ctrl+Enter</span>
					</button>
				{:else}
					<button class="h-8 bg-red-600 hover:bg-red-700 text-white px-3 py-1 rounded-lg text-xs font-bold transition-all flex items-center gap-1.5 shadow-sm" onclick={stopCode}>
						<Square size={12} /> Stop
					</button>
				{/if}
				
				<button class="h-8 bg-zinc-800 hover:bg-zinc-700 text-zinc-300 px-3 py-1 rounded-lg text-xs font-bold transition-all flex items-center gap-1.5 border border-zinc-700" onclick={clearTerminal}>
					<RefreshCw size={11} /> Reset Terminal
				</button>
			{/if}
		</div>

		<!-- Status: jelaskan kenapa Run belum kelihatan bereaksi (worker WASM berat) -->
		<div class="text-[11px] font-mono text-zinc-400">
			{#if statusText}
				{statusText}
			{/if}
		</div>
	</div>

	<!-- Monaco Editor (dengan fallback error bila CDN gagal) -->
	{#if loadError}
		<div class="p-6 text-center" style="height: {height};">
			<p class="text-sm font-bold text-red-400">{loadError}</p>
			<button class="mt-3 h-8 bg-zinc-800 hover:bg-zinc-700 text-zinc-200 px-4 py-1 rounded-lg text-xs font-bold border border-zinc-700" onclick={() => location.reload()}>
				Muat Ulang
			</button>
		</div>
	{:else}
		<div bind:this={el} style="height: {height};" class="overflow-hidden bg-[#1e1e1e]"></div>
	{/if}
</div>

<!-- Standalone Mac-style Terminal (below the editor) -->
{#if runnable}
	<div class="flex flex-col overflow-hidden rounded-xl border border-zinc-800 bg-[#18181b] shadow-lg flex-grow min-h-0">
		<!-- macOS window control bar -->
		<div class="flex items-center justify-between px-4 py-2 bg-[#141414] border-b border-zinc-850 select-none flex-shrink-0">
			<div class="flex items-center gap-1.5">
				<span class="w-2.5 h-2.5 rounded-full bg-[#ff5f56] border border-[#e0443e]"></span>
				<span class="w-2.5 h-2.5 rounded-full bg-[#ffbd2e] border border-[#dfa123]"></span>
				<span class="w-2.5 h-2.5 rounded-full bg-[#27c93f] border border-[#1aab29]"></span>
			</div>
			<span class="text-[10px] font-mono text-zinc-500 font-bold tracking-wider">TERMINAL</span>
			<div class="w-12"></div> <!-- Spacer for symmetry -->
		</div>

		<!-- Terminal contents -->
		<div class="p-3 bg-[#18181b] flex-grow min-h-[220px]">
			<div bind:this={termEl} class="w-full h-full min-h-[200px]"></div>
		<div class="sr-only whitespace-pre-wrap" aria-live="polite" aria-atomic="true">{outputText}</div>
		</div>
	</div>
{/if}
