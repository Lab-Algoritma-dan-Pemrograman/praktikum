<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import CodeEditor from '$lib/components/CodeEditor.svelte';
	import { initRunner } from '$lib/stores/runner';
	import { Code } from 'lucide-svelte';

	// starter snippets per language
	const STARTER = {
		c: `#include <stdio.h>\n\nint main() {\n    printf("Hello, Praktikum!\\n");\n    return 0;\n}\n`,
		python: `print("Hello, Praktikum!")\n`
	};

	let code = $state(STARTER.c);
	let language = $state<'c' | 'python'>('c');
	let editorKey = $state(0); // force re-mount on language switch

	// run on mount to ensure workers init even if user only watches
	onMount(() => {
		initRunner();
	});

	function switchLang(lang: 'c' | 'python') {
		language = lang;
		code = STARTER[lang];
		// remount CodeEditor so Monaco reloads with the right language
		editorKey += 1;
	}

	function reset() {
		code = '';
	}
</script>

<svelte:head>
	<title>Live Code — Info Praktikum</title>
</svelte:head>

<div class="min-h-screen bg-gradient-to-br from-slate-50 via-blue-50/40 to-indigo-50/60 pb-12">
	<div class="max-w-6xl mx-auto px-4 pt-8">
		<!-- header -->
		<div class="mb-6 pb-4 border-b border-gray-200">
			<h1 class="text-3xl font-bold text-gray-800 flex items-center gap-3">
				<Code size={28} class="text-blue-600" />
				Live Code Editor
			</h1>
			<p class="text-sm text-gray-600 mt-1">
				Tuliskan dan jalankan kode langsung di sini — Monaco + Pyodide (Python) / Clang→WASM (C).
				Soal praktikum asli tetap memakai Edra di halaman praktikum.
			</p>
		</div>

		<!-- toolbar -->
		<div class="flex items-center justify-between mb-4 gap-3">
			<div class="flex gap-2">
				<button
					class={`px-4 py-2 rounded-lg text-sm font-medium transition ${language === 'c' ? 'bg-blue-600 text-white shadow' : 'bg-gray-100 hover:bg-gray-200'}`}
					onclick={() => switchLang('c')}>C</button>
				<button
					class={`px-4 py-2 rounded-lg text-sm font-medium transition ${language === 'python' ? 'bg-blue-600 text-white shadow' : 'bg-gray-100 hover:bg-gray-200'}`}
					onclick={() => switchLang('python')}>Python</button>
			</div>
			<div class="flex gap-2">
				<button
					class="px-3 py-2 rounded-lg bg-gray-100 hover:bg-gray-200 text-sm font-medium"
					onclick={reset}>Reset</button>
			</div>
		</div>

		<!-- editor (runnable wires its internal terminal + run button) -->
		<div class="border border-gray-200 rounded-xl overflow-hidden shadow bg-white">
			<CodeEditor
				key={editorKey}
				bind:value={code}
				language={language}
				height="500px"
				runnable
				readonly={false}
			/>
		</div>
	</div>
</div>
