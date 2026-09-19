<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { api } from '$lib/api';
	import { browser } from '$app/environment';
	import { Play, Square, RefreshCw } from 'lucide-svelte';
	import {
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
		oninput
	} = $props();

	let el: HTMLDivElement;
	let termEl: HTMLDivElement;
	let editor: any = null;
	let monacoRef: any = null;
	let term: any = null;
	let fitAddon: any = null;

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
	let running = $state(false);
	let fallbackMode = $state(false);

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

		// Initialize Monaco
		const loader = (await import('@monaco-editor/loader')).default;
		const monaco = await loader.init();
		monacoRef = monaco;
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
		editor.onDidChangeModelContent(() => {
			value = editor.getValue();
			oninput?.();
		});

		editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter, () => {
			runCode();
		});

		// Initialize xterm.js if runnable
		if (runnable && termEl) {
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

			const resizeObserver = new ResizeObserver(() => fitAddon?.fit());
			resizeObserver.observe(termEl);

			return () => {
				resizeObserver.disconnect();
				term?.dispose();
			};
		}
	});

	$effect(() => {
		if (editor && value !== editor.getValue()) {
			editor.setValue(value ?? '');
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

		term.clear();
		outputText = '';
		running = true;
		fallbackMode = false;

		if (runLang === 'python') {
			if (!$pyodideWorkerStore) {
				runServerFallback();
				return;
			}
			runPythonInteractive();
		} else {
			if (!$cWorkerStore) {
				runServerFallback();
				return;
			}
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
			wl('\x1b[33m⏳ Worker masih dimuat. Tunggu sebentar lalu klik Run lagi.\x1b[0m');
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
		clearTerminal();
		wl('\x1b[1;31m⚠ Eksekusi dihentikan oleh pengguna.\x1b[0m');
	}

	onDestroy(() => {
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

		<!-- Spacer/Status area empty -->
		<div></div>
	</div>

	<!-- Monaco Editor -->
	<div bind:this={el} style="height: {height};" class="overflow-hidden bg-[#1e1e1e]"></div>
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
