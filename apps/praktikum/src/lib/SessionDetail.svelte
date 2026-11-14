<script lang="ts">
  import { goto } from '$app/navigation';
  export let id: string;
  import { onMount } from 'svelte';
  import {
    PracticumClient,
    reusableCommand,
    errorMessage,
    courseLabels,
    jakarta,
    type Attempt,
    type Command,
    type SessionDetail
  } from '@algohub/practicum-client';
  let detail: SessionDetail | null = null;
  let loading = true;
  let issue = '';
  let starting = '';
  let tokens: Record<string, string> = {};
  let pending: Command | null = null;
  const client = new PracticumClient();
  async function load() {
    loading = true;
    issue = '';
    try {
      detail = await client.detail(id);
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loading = false;
    }
  }
  async function start(id: string) {
    starting = id;
    issue = '';
    pending = reusableCommand(pending, '/practicum/attempts', 'POST', {
      activation_id: id,
      ...(tokens[id] ? { access_token: tokens[id] } : {})
    });
    try {
      const attempt = await client.execute<Attempt>(pending);
      tokens = {};
      pending = null;
      await goto(`/attempts/${encodeURIComponent(attempt.attempt_id)}`);
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      starting = '';
    }
  }
  onMount(() => {
    void load();
  });
</script>

<svelte:head><title>Detail sesi — Praktikum</title></svelte:head>
{#if loading}<p role="status">Memuat detail sesi…</p>{/if}
{#if issue}<div class="notice error" role="alert">
    {issue}
    <div class="actions">
      <button class="secondary" onclick={load} disabled={loading || !!starting}
        >Muat ulang detail</button
      ><a href="/login">Masuk kembali</a>
    </div>
  </div>{/if}
{#if detail}
  <h1>{detail.session.title}</h1>
  <p>
    Setiap activation memiliki satu attempt. Melanjutkan tidak mengulang waktu.
    Sesi ujian memerlukan persetujuan masuk dari pengajar.
  </p>
  <div class="stack">
    {#each detail.courses as course}
      <section class="panel">
        <h2>{course.title}</h2>
        <p>{courseLabels[course.kind]} · {course.duration_minutes} menit</p>
        {#each detail.activations.filter((a) => a.course_id === course.id) as activation}
          <form
            class="stack panel"
            data-activation-id={activation.id}
            onsubmit={(event) => {
              event.preventDefault();
              void start(activation.id);
            }}
          >
            <p>
              {activation.is_open ? 'Terbuka' : 'Ditutup'} · Kelas {activation
                .scope.class_id} / Shift {activation.scope.shift} · {activation.question_count}
              soal
            </p>
            <p class="muted">
              Timer {activation.timer_mode === 'shared'
                ? 'bersama, dimulai peserta pertama'
                : 'individual'}<br />Batas penutupan: {jakarta(
                activation.closes_at
              )}{#if activation.deadline_at}<br />Deadline: {jakarta(
                  activation.deadline_at
                )}{/if}
            </p>
            {#if course.token_required}<label
                >Token akses<input
                  type="password"
                  autocomplete="off"
                  maxlength="128"
                  bind:value={tokens[activation.id]}
                  disabled={!!starting}
                /></label
              >{/if}
            <button
              disabled={!activation.is_open ||
                !detail.session.active ||
                !!starting}
              >{starting === activation.id
                ? 'Membuka…'
                : 'Mulai / lanjutkan attempt'}</button
            >
            {#if !activation.is_open}<p class="muted">
                Activation ditutup. Hasil attempt yang dikumpulkan tersedia di
                Nilai saya.
              </p>{/if}
          </form>
        {:else}<p class="notice">
            Belum ada activation untuk roster Anda.
          </p>{/each}
      </section>
    {:else}<p class="notice">Belum ada course.</p>{/each}
  </div>
{/if}
