<script lang="ts">
  import { onMount } from 'svelte';
  import {
    SiakadClient,
    financeCommand,
    errorMessage,
    rupiah,
    jakarta,
    permitted,
    type AcademicFinanceMutation,
    type Command
  } from '@algohub/siakad-client';
  import type { AppIdentityData } from '@algohub/auth-client';
  export let data: AppIdentityData;
  const client = new SiakadClient();
  let saldo = 0;
  let income = 0;
  let expense = 0;
  let adjustment = 0;
  let mutations: AcademicFinanceMutation[] = [];
  let loading = false;
  let busy = false;
  let issue = '';
  let notice = '';
  let targetUserId = 1;
  let type: AcademicFinanceMutation['type'] = 'income';
  let note = '';
  let amount = 0;
  let category = 'Kas';
  let pending: Command | null = null;

  $: canManage = permitted(data.principal, 'finance.manage', {
    domain: 'siakad'
  });

  async function load() {
    if (loading) return;
    loading = true;
    issue = '';
    try {
      const balance = await client.get<{
        saldo: number;
        income: number;
        expense: number;
        adjustment: number;
        mutations: AcademicFinanceMutation[];
      }>('/academic/keuangan?limit=25&offset=0');
      saldo = balance.saldo;
      income = balance.income;
      expense = balance.expense;
      adjustment = balance.adjustment;
      mutations = balance.mutations;
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loading = false;
    }
  }

  async function post() {
    if (busy || !note.trim() || amount <= 0) return;
    busy = true;
    issue = '';
    notice = '';
    pending = financeCommand(pending, {
      target_user_id: targetUserId,
      type,
      note: note.trim(),
      amount,
      category: category.trim() || 'Kas'
    });
    try {
      const created = await client.execute<AcademicFinanceMutation>(pending);
      pending = null;
      note = '';
      amount = 0;
      mutations = [created, ...mutations];
      if (created.type === 'income') income += created.amount;
      else if (created.type === 'expense') expense += created.amount;
      else adjustment += created.amount;
      saldo = income - expense + adjustment;
      notice = `Mutasi ${created.type} ${rupiah(created.amount)} tercatat untuk ${created.user_name}.`;
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      busy = false;
    }
  }

  onMount(() => {
    void load();
  });
</script>

<svelte:head><title>Keuangan — SIAKAD</title></svelte:head>
<section class="stack" aria-labelledby="keuangan-title">
  <h2 id="keuangan-title">Keuangan mahasiswa</h2>
  {#if loading}<p role="status">Memuat saldo…</p>{/if}
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if notice}<p class="notice success" role="status">{notice}</p>{/if}

  <div class="grid">
    <article class="panel">
      <p class="muted">Saldo</p>
      <p><strong>{rupiah(saldo)}</strong></p>
    </article>
    <article class="panel">
      <p class="muted">Total masuk</p>
      <p>{rupiah(income)}</p>
    </article>
    <article class="panel">
      <p class="muted">Total keluar</p>
      <p>{rupiah(expense)}</p>
    </article>
    <article class="panel">
      <p class="muted">Penyesuaian</p>
      <p>{rupiah(adjustment)}</p>
    </article>
  </div>

  <section class="stack" aria-label="Riwayat mutasi saya">
    <h3>Riwayat mutasi saya</h3>
    {#each mutations as row (row.id)}
      <article class="panel" data-mutation-id={row.id}>
        <span class="badge"
          >{row.type.toUpperCase()} · {rupiah(row.amount)}</span
        >
        <p>{row.note} ({row.category})</p>
        <p class="muted">{jakarta(row.created_at)}</p>
      </article>
    {:else}
      {#if !loading}<p class="notice">Belum ada mutasi tercatat.</p>{/if}
    {/each}
  </section>

  {#if canManage}
    <form
      class="panel stack"
      onsubmit={(e) => {
        e.preventDefault();
        void post();
      }}
    >
      <h3>Catat mutasi (admin/bendahara)</h3>
      <div class="grid">
        <label
          >ID mahasiswa<input
            type="number"
            min="1"
            bind:value={targetUserId}
            required
            disabled={busy}
          /></label
        >
        <label
          >Jenis<select bind:value={type} disabled={busy}>
            <option value="income">Masuk</option>
            <option value="expense">Keluar</option>
            <option value="adjustment">Penyesuaian saldo</option>
          </select></label
        >
        <label
          >Nominal (Rp)<input
            type="number"
            min="1"
            bind:value={amount}
            required
            disabled={busy}
          /></label
        >
        <label
          >Kategori<input
            bind:value={category}
            maxlength="64"
            disabled={busy}
          /></label
        >
      </div>
      <label
        >Keterangan<input
          bind:value={note}
          required
          maxlength="500"
          placeholder="Contoh: Deposit kas, pembayaran sewa alat"
          disabled={busy}
        /></label
      >
      <button disabled={busy || !note.trim() || amount <= 0}
        >{busy ? 'Menyimpan…' : 'Catat mutasi'}</button
      >
    </form>
  {/if}
</section>
