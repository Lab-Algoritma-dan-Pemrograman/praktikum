<script lang="ts">
  import type { AppIdentityData } from '@algohub/auth-client';
  import BrowserPreview from '@algohub/ui/BrowserPreview.svelte';
  import { warmupPreview, type PlaygroundFallbackResult } from '@algohub/ui';
  import {
    ElearningClient,
    command,
    type LearningLanguage
  } from '@algohub/elearning-client';

  export let data: AppIdentityData;

  const client = new ElearningClient();
  let language: LearningLanguage = 'c';
  let source = '#include <stdio.h>\nint main(void) {\n  return 0;\n}\n';
  let stdin = '';

  $: runtimeOrigin =
    data.applications.find((app) => app.name === 'portal')?.origin ?? '';
</script>

<svelte:head><title>Playground — E-learning</title></svelte:head>
<section class="stack" aria-labelledby="playground-title">
  <h1 id="playground-title">Playground C & Python</h1>
  <p>
    Eksperimen utama berjalan langsung di browser melalui C/WASM atau Pyodide.
    Glot, bila tersedia, hanya fallback preview non-authoritative dan tidak
    memberi XP atau nilai akademik.
  </p>
  <div class="panel stack">
    <label
      >Bahasa playground<select bind:value={language}>
        <option value="c">C</option>
        <option value="python">Python</option>
      </select></label
    >
    <label
      >Kode playground<textarea
        class="code"
        rows="10"
        spellcheck="false"
        maxlength="65536"
        bind:value={source}
        onfocusin={() => warmupPreview(runtimeOrigin, language)}
      ></textarea></label
    >
    <label
      >Input standar playground<textarea
        class="code"
        rows="3"
        maxlength="8192"
        bind:value={stdin}
      ></textarea></label
    >
    <BrowserPreview
      {runtimeOrigin}
      {language}
      {source}
      {stdin}
      serverRun={(input) =>
        client.execute<PlaygroundFallbackResult>(
          command('/playground/run', 'POST', input)
        )}
    />
  </div>
</section>
