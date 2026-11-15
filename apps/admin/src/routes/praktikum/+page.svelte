<script lang="ts">
  import { onMount } from 'svelte';
  import type { AppIdentityData } from '@algohub/auth-client';
  import {
    PracticumClient,
    permitted,
    reusableCommand,
    errorMessage,
    type Command,
    type Scope,
    type Session,
    type SessionDetail
  } from '@algohub/practicum-client';
  import Authoring from './Authoring.svelte';
  import Activations from './Activations.svelte';
  import Monitoring from './Monitoring.svelte';
  import Rekap from './Rekap.svelte';
  export let data: AppIdentityData;
  const client = new PracticumClient();
  let sessions: Session[] = [];
  let detail: SessionDetail | null = null;
  let selected = '';
  let tab = 'activations';
  let offset = 0;
  let loading = false;
  let issue = '';
  let title = '';
  let sequence = 1;
  let institution = '';
  let division = '';
  let createBusy = false;
  let pending: Command | null = null;
  $: scope = {
    domain: 'praktikum',
    ...(institution ? { institution } : {}),
    ...(division ? { division } : {})
  } satisfies Scope;
  $: canCreate = permitted(data.principal, 'practicum.session.manage', scope);
  $: canDefine =
    !!detail &&
    permitted(data.principal, 'practicum.session.manage', {
      ...detail.session.scope,
      session_id: detail.session.id
    });
  $: canQuestions =
    !!detail &&
    permitted(data.principal, 'question.manage', {
      ...detail.session.scope,
      session_id: detail.session.id
    });
  $: canAudit =
    !!detail &&
    permitted(data.principal, 'audit.read', {
      ...detail.session.scope,
      session_id: detail.session.id
    });
  async function load(next = offset) {
    loading = true;
    issue = '';
    try {
      sessions = await client.get<Session[]>(
        `/admin/praktikum/sesi?limit=25&offset=${next}`
      );
      offset = next;
      if (!selected && sessions.length) await select(sessions[0].id);
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loading = false;
    }
  }
  async function select(id: string) {
    selected = id;
    detail = null;
    issue = '';
    loading = true;
    try {
      const value = await client.get<SessionDetail>(
        `/admin/praktikum/sesi/${encodeURIComponent(id)}`
      );
      if (id === selected) detail = value;
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loading = false;
    }
  }
  async function refresh() {
    if (!selected) return;
    const value = await client.get<SessionDetail>(
      `/admin/praktikum/sesi/${encodeURIComponent(selected)}`
    );
    detail = value;
  }
  async function create() {
    if (!canCreate) return;
    createBusy = true;
    issue = '';
    pending = reusableCommand(pending, '/admin/praktikum/sesi', 'POST', {
      title,
      sequence,
      scope
    });
    try {
      const session = await client.execute<Session>(pending);
      pending = null;
      title = '';
      await load();
      await select(session.id);
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      createBusy = false;
    }
  }
  onMount(() => {
    void load();
  });
  $: canRekap =
    !!detail &&
    detail.activations.some((a) =>
      permitted(data.principal, 'practicum.grade.write', a.scope)
    );
</script>

<svelte:head><title>Admin Praktikum — Algohub</title></svelte:head>
<section class="practicum stack">
  <h1>Konfigurasi praktikum</h1>
  <p>
    Definisi, activation, monitoring, dan penilaian mengikuti permission serta
    scope akun. Snapshot soal dan roster tidak dapat diubah setelah activation.
  </p>
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  <div class="panel stack">
    <label
      >Sesi<select
        value={selected}
        onchange={(event) => select(event.currentTarget.value)}
        disabled={loading}
        ><option value="" disabled>Pilih sesi</option
        >{#each sessions as session}<option value={session.id}
            >{session.sequence}. {session.title}</option
          >{/each}</select
      ></label
    >
    <div class="actions">
      <button class="secondary" onclick={() => load()} disabled={loading}
        >Perbarui sesi</button
      ><button
        class="secondary"
        disabled={offset === 0 || loading}
        onclick={() => load(offset - 25)}>Sesi sebelumnya</button
      ><button
        class="secondary"
        disabled={sessions.length < 25 || loading}
        onclick={() => load(offset + 25)}>Sesi berikutnya</button
      >
    </div>
    {#if !loading && sessions.length === 0}<p class="notice">
        Tidak ada sesi dalam scope Anda.
      </p>{/if}
  </div>
  {#if data.principal?.grants.some((g) => g.permission === 'practicum.session.manage' && !g.scope.class_id && !g.scope.shift && !g.scope.session_id)}
    <details class="panel">
      <summary>Buat sesi baru</summary>
      <form
        class="stack"
        onsubmit={(event) => {
          event.preventDefault();
          void create();
        }}
      >
        <label
          >Judul sesi baru<input
            bind:value={title}
            required
            maxlength="200"
          /></label
        ><label
          >Urutan sesi<input
            type="number"
            bind:value={sequence}
            required
            min="1"
          /></label
        >
        <div class="grid">
          <label
            >Institusi (opsional)<input
              bind:value={institution}
              maxlength="100"
            /></label
          ><label
            >Divisi (opsional)<input
              bind:value={division}
              maxlength="100"
            /></label
          >
        </div>
        <button disabled={!canCreate || createBusy}
          >{createBusy ? 'Membuat…' : 'Buat sesi'}</button
        >{#if !canCreate}<p class="notice">
            Scope definisi ini tidak tercakup grant Anda.
          </p>{/if}
      </form>
    </details>
  {/if}
  {#if loading}<p role="status">Memuat Praktikum…</p>{/if}
  {#if detail}
    <nav aria-label="Panel Admin Praktikum">
      <button
        class:secondary={tab !== 'activations'}
        aria-pressed={tab === 'activations'}
        onclick={() => (tab = 'activations')}>Activation</button
      ><button
        class:secondary={tab !== 'monitoring'}
        aria-pressed={tab === 'monitoring'}
        onclick={() => (tab = 'monitoring')}>Monitoring dan penilaian</button
      >{#if canDefine || canQuestions}<button
          class:secondary={tab !== 'authoring'}
          aria-pressed={tab === 'authoring'}
          onclick={() => (tab = 'authoring')}>Definisi dan soal</button
        >{/if}{#if canRekap}<button
          class:secondary={tab !== 'rekap'}
          aria-pressed={tab === 'rekap'}
          onclick={() => (tab = 'rekap')}>Rekap nilai</button
        >{/if}{#if canAudit}<a href="/auditlog">Audit sesuai scope</a>{/if}
    </nav>
    {#key detail.session.id}
      {#if tab === 'authoring' && (canDefine || canQuestions)}<Authoring
          {detail}
          principal={data.principal}
          {refresh}
        />
      {:else if tab === 'monitoring'}<Monitoring
          {detail}
          principal={data.principal}
        />
      {:else if tab === 'rekap' && canRekap}<Rekap
          {detail}
          principal={data.principal}
        />
      {:else}<Activations {detail} principal={data.principal} {refresh} />{/if}
    {/key}
  {/if}
</section>
