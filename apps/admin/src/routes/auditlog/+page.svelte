<script lang="ts">
  import { onMount } from 'svelte';
  import {
    AuthClient,
    asApiError,
    scopeLabel,
    type ApiError,
    type AuditEvent
  } from '@algohub/auth-client';
  import { RequestError } from '@algohub/ui';
  export let data: { audit: { events: AuditEvent[]; nextBefore: number } };
  const api = new AuthClient();
  let events: AuditEvent[] = [];
  let before: number | undefined;
  let history: Array<number | undefined> = [];
  let busy = true;
  onMount(() => {
    busy = false;
  });
  let issue: ApiError | null = null;
  let nextBefore = 0;
  $: {
    events = data.audit.events;
    nextBefore = data.audit.nextBefore;
    before = undefined;
    history = [];
  }
  const date = (value: string) =>
    new Date(value).toLocaleString('id-ID', { timeZone: 'UTC' });

  async function loadPage(direction: 'next' | 'previous' | 'retry') {
    const cursor =
      direction === 'next'
        ? nextBefore
        : direction === 'previous'
          ? history[history.length - 1]
          : before;
    busy = true;
    issue = null;
    try {
      const result = await api.audit(cursor);
      if (direction === 'next') history = [...history, before];
      if (direction === 'previous') history = history.slice(0, -1);
      events = result.events;
      nextBefore = result.nextBefore;
      before = cursor;
    } catch (error) {
      issue = asApiError(error);
    } finally {
      busy = false;
    }
  }
</script>

<svelte:head><title>Audit log — Algohub Admin</title></svelte:head>
<section class="identity-page">
  <p class="eyebrow">IDENTITY / AUDIT</p>
  <h1>Jejak perubahan</h1>
  <p>
    Hanya event pada scope audit yang diizinkan oleh server. Waktu ditampilkan
    dalam UTC.
  </p>
  <RequestError
    error={issue}
    onRetry={() => loadPage('retry')}
    returnTo="/auditlog"
  />
  <div class="identity-panel" aria-busy={busy}>
    {#if busy}<p role="status">Memuat jejak audit…</p>{/if}
    {#if !events.length}<p>
        Belum ada event audit dalam scope yang diberikan.
      </p>{/if}
    <ol class="audit-list">
      {#each events as event}
        <li class="audit-event">
          <p class="eyebrow">
            {event.app} /
            <time datetime={event.created_at}>{date(event.created_at)} UTC</time
            >
          </p>
          <h2>{event.action}</h2>
          <p>
            {event.actor_id ? `Pengguna #${event.actor_id}` : 'Sistem'} · {event.subject_type}
            {event.subject_id}
          </p>
          <p class="muted">{scopeLabel(event.scope)}</p>
          <p class="muted">
            Request ID: <code>{event.request_id}</code><br />Operasi:
            <code>{event.operation_id}</code>
          </p>
          {#if event.detail && Object.keys(event.detail).length}<details>
              <summary>Ringkasan perubahan</summary>
              <pre>{JSON.stringify(event.detail, null, 2)}</pre>
            </details>{/if}
        </li>
      {/each}
    </ol>
    <nav class="actions" aria-label="Halaman audit">
      <button
        class="secondary"
        type="button"
        disabled={busy || !history.length}
        onclick={() => loadPage('previous')}>Sebelumnya</button
      ><button
        class="secondary"
        type="button"
        disabled={busy || !nextBefore}
        onclick={() => loadPage('next')}>Lebih lama</button
      >
    </nav>
  </div>
</section>

<style>
  .audit-list {
    padding: 0;
    list-style: none;
  }
</style>
