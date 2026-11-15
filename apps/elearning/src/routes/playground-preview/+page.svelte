<script lang="ts">
  // Jalur preview C/Glot non-interaktif. Dipisah dari /playground yang kini
  // memakai worker SAB interaktif (input() blocking), mengikuti legacy.
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

<svelte:head><title>Playground preview — E-learning</title></svelte:head>
<section class="stack" aria-labelledby="playground-preview-title">
  <h1 id="playground-preview-title">Playground C &amp; Python (preview)</h1>
  <p class="muted">
    Jalankan kode lewat iframe runtime terisolasi. Tidak butuh
    SharedArrayBuffer; cocok untuk browser lama atau bila COOP/COEP tak
    tersedia. Untuk pengalaman interaktif <code>input()</code> live, buka
    <a href="/playground">playground utama</a>.
  </p>
  <p class="muted">
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
