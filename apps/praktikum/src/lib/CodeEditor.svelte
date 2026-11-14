<script lang="ts">
  import BrowserPreview from '@algohub/ui/BrowserPreview.svelte';
  import { warmupPreview, type PlaygroundFallbackResult } from '@algohub/ui';
  export let runtimeOrigin: string;
  import {
    PracticumClient,
    command,
    reusableCommand,
    errorMessage,
    type Command,
    type RunnerInfo,
    type RunResult
  } from '@algohub/practicum-client';
  export let attemptId: string;
  export let questionId: string;
  export let number: number;
  export let value: string;
  export let language: string;
  export let disabled = false;
  export let offline = false;
  export let runner: RunnerInfo | null = null;
  export let update: (value: string, language: string) => void;
  let stdin = '';
  let selectedVersion = '';
  let pending: Command | null = null;
  let result: RunResult | null = null;
  let issue = '';
  let busy = false;
  $: choices =
    runner?.languages.filter((item) => item.language === language) ?? [];
  $: version = choices.some((item) => item.version === selectedVersion)
    ? selectedVersion
    : (choices[0]?.version ?? '');
  $: languages = [
    ...new Map(
      (runner?.languages ?? []).map((item) => [item.language, item.label])
    ).entries()
  ];
  async function run() {
    busy = true;
    issue = '';
    result = null;
    pending = reusableCommand(
      pending,
      `/practicum/attempts/${encodeURIComponent(attemptId)}/run`,
      'POST',
      { question_id: questionId, source: value, language, version, stdin }
    );
    try {
      result = await new PracticumClient().execute<RunResult>(pending);
      // Terminal results are cached by key, including provider_error. A new
      // explicit run is a new operation; only uncertain transport retries reuse.
      pending = null;
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      busy = false;
    }
  }
</script>

<div class="stack">
  <div class="grid">
    <label
      >Bahasa soal {number}<select
        value={language}
        onchange={(event) => update(value, event.currentTarget.value)}
        disabled={disabled || !runner?.available}
      >
        {#each languages as [id, label]}<option value={id}>{label}</option
          >{/each}
        {#if !languages.some(([id]) => id === language)}<option value={language}
            >{language || 'Belum tersedia'}</option
          >{/if}
      </select></label
    >
    {#if runner?.available}
      <label
        >Compiler soal {number}<select
          value={version}
          onchange={(event) => (selectedVersion = event.currentTarget.value)}
          {disabled}
          >{#each choices as choice}<option value={choice.version}
              >{choice.version}</option
            >{/each}</select
        ></label
      >
    {/if}
  </div>
  <label
    >Kode soal {number}<textarea
      class="code"
      rows="12"
      spellcheck="false"
      autocomplete="off"
      maxlength="65536"
      {value}
      readonly={disabled}
      oninput={(event) => update(event.currentTarget.value, language)}
      onfocusin={() => warmupPreview(runtimeOrigin, language)}
    ></textarea></label
  >
  <label
    >Input standar soal {number}<textarea
      class="code"
      rows="3"
      maxlength="8192"
      bind:value={stdin}
      {disabled}
    ></textarea></label
  >
  <BrowserPreview
    {runtimeOrigin}
    {language}
    source={value}
    {stdin}
    {disabled}
    serverRun={(input) =>
      new PracticumClient().execute<PlaygroundFallbackResult>(
        command('/playground/run', 'POST', input)
      )}
  />
  {#if runner?.simulated}<p class="notice">
      Runner simulasi — tidak ada eksekusi kode nyata. Output hanya fixture
      pengujian, bukan bukti program benar.
    </p>{/if}
  {#if runner?.available}
    <button
      class="secondary"
      onclick={run}
      disabled={disabled || offline || busy || !version}
      >{busy ? 'Memproses…' : 'Jalankan kode di server'}</button
    >
    {#if issue}<p class="notice error" role="alert">
        {issue} Tekan Jalankan kode untuk mencoba kembali permintaan yang sama.
      </p>{/if}
    {#if result}
      <section
        class="panel"
        aria-label={`Hasil runner soal ${number}`}
        aria-live="polite"
      >
        <h3>Hasil: {result.status}</h3>
        {#if result.simulated}<p class="notice">
            SIMULASI · Tidak ada eksekusi nyata.
          </p>{/if}
        <p class="muted">
          {result.duration_ms} ms · Exit {result.exit_code ?? '—'} · {result.runner_version}
        </p>
        <button class="secondary" onclick={() => (result = null)}
          >Bersihkan output soal {number}</button
        >
        <h4>stdout</h4>
        <!-- svelte-ignore a11y_no_noninteractive_tabindex (Scrollable output must be keyboard reachable.) -->
        <pre
          class="runner-output"
          tabindex="0"
          aria-label={`stdout soal ${number}`}>{result.stdout ||
            '(kosong)'}</pre>
        <h4>stderr</h4>
        <!-- svelte-ignore a11y_no_noninteractive_tabindex (Scrollable output must be keyboard reachable.) -->
        <pre
          class="runner-output"
          tabindex="0"
          aria-label={`stderr soal ${number}`}>{result.stderr ||
            '(kosong)'}</pre>
        {#if result.truncated}<p>Output dipotong sesuai batas ukuran.</p>{/if}
        <details>
          <summary>Metadata permintaan</summary>
          <p class="prompt">
            ID: {result.request_id}<br />Hash source: {result.source_hash}
          </p>
        </details>
        {#if result.status === 'provider_error'}<p>
            Provider gagal. Anda boleh mencoba lagi dengan input yang sama;
            jawaban tersimpan tidak berubah.
          </p>{/if}
      </section>
    {/if}
  {:else}
    <p class="notice">
      Preview browser hanya untuk latihan. Jawaban tersimpan tetap masuk ke alur
      penilaian manual dan hasil preview tidak menetapkan nilai akademik.
    </p>
  {/if}
</div>
