<script lang="ts">
  import { onMount } from 'svelte';
  import {
    SiakadClient,
    rentalCommand,
    emptyPage,
    loadResourcePage,
    mergeResources,
    errorMessage,
    jakarta,
    rupiah,
    type RentalCommand,
    type InventoryItem,
    type InventoryRental
  } from '@algohub/siakad-client';
  const client = new SiakadClient();
  let items: InventoryItem[] = [];
  let rentals: InventoryRental[] = [];
  let itemPage = emptyPage<InventoryItem>();
  let rentalPage = emptyPage<InventoryRental>();
  let itemId = '';
  let quantity = 1;
  let purpose = '';
  let dueDays = 3;
  let loading = false;
  let busy = false;
  let issue = '';
  let notice = '';
  let pending: RentalCommand | null = null;

  $: availableItems = items.filter(
    (item) => item.status === 'available' && item.available_quantity > 0
  );

  async function load(kind: 'all' | 'items' | 'rentals' = 'all') {
    if (loading) return;
    loading = true;
    issue = '';
    const selected = items.find((item) => item.id === itemId);
    try {
      const [inv, ownRentals] = await Promise.all([
        kind === 'all' || kind === 'items'
          ? loadResourcePage(
              client,
              '/inventory/items',
              itemPage,
              (row) => row.id,
              kind !== 'all',
              selected ? [selected] : []
            )
          : itemPage,
        kind === 'all' || kind === 'rentals'
          ? loadResourcePage(
              client,
              '/inventory/rentals',
              rentalPage,
              (row) => row.id,
              kind !== 'all'
            )
          : rentalPage
      ]);
      itemPage = inv;
      rentalPage = ownRentals;
      items = inv.rows;
      rentals = ownRentals.rows;
      if (!itemId && !pending)
        itemId =
          items.find(
            (item) => item.status === 'available' && item.available_quantity > 0
          )?.id ?? '';
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loading = false;
    }
  }

  async function submitRental() {
    if (busy || loading || !itemId || !purpose.trim()) return;
    busy = true;
    issue = '';
    notice = '';
    pending = rentalCommand(pending, {
      item_id: itemId,
      quantity,
      purpose: purpose.trim(),
      due_days: dueDays
    });
    try {
      const created = await client.execute<InventoryRental>(pending);
      pending = null;
      purpose = '';
      rentals = mergeResources(rentals, [created], (row) => row.id);
      rentalPage = { ...rentalPage, rows: rentals };
      notice = `Permohonan sewa ${created.item_name} (${created.quantity} unit) berhasil diajukan.`;
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

<svelte:head><title>Inventaris & Penyewaan — SIAKAD</title></svelte:head>
<section class="stack" aria-labelledby="rental-title">
  <h2 id="rental-title">Inventaris dan penyewaan alat</h2>
  {#if loading}<p role="status">Memuat inventaris…</p>{/if}
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if notice}<p class="notice success" role="status">{notice}</p>{/if}

  <div class="grid">
    {#each items as item}
      <article class="panel stack" data-item-id={item.id}>
        <span class="badge">{item.category} · {item.status}</span>
        <h3>{item.name} ({item.code})</h3>
        <p>
          Stok tersedia: <strong
            >{item.available_quantity} / {item.total_quantity}</strong
          >
          · Biaya sewa: <strong>{rupiah(item.rental_fee)}</strong> / unit
        </p>
      </article>
    {/each}
  </div>
  {#if itemPage.hasMore}
    <button
      class="secondary"
      disabled={busy || loading}
      onclick={() => load('items')}>Muat alat lainnya</button
    >
  {/if}

  <form
    class="panel stack"
    onsubmit={(e) => {
      e.preventDefault();
      void submitRental();
    }}
  >
    <h3>Ajukan peminjaman / penyewaan alat</h3>
    <div class="grid">
      <label
        >Alat laboratorium<select
          bind:value={itemId}
          required
          disabled={busy || loading}
        >
          {#if itemId && !availableItems.some((item) => item.id === itemId)}
            <option value={itemId}>Alat dipilih tidak tersedia: {itemId}</option
            >
          {/if}
          {#each availableItems as item}<option value={item.id}
              >{item.name} (tersedia {item.available_quantity})</option
            >{/each}
        </select></label
      >
      <label
        >Jumlah pinjam<input
          type="number"
          min="1"
          max={Math.min(
            1000,
            availableItems.find((item) => item.id === itemId)
              ?.available_quantity ?? 0
          )}
          bind:value={quantity}
          required
        /></label
      >
      <label
        >Durasi pinjam (hari)<input
          type="number"
          min="1"
          max="30"
          bind:value={dueDays}
          required
        /></label
      >
    </div>
    <label
      >Keperluan peminjaman<input
        bind:value={purpose}
        required
        maxlength="200"
      /></label
    >
    <button disabled={busy || loading || !itemId || !purpose.trim()}
      >{busy ? 'Mengirim…' : 'Ajukan peminjaman'}</button
    >
  </form>

  <section class="stack" aria-label="Riwayat peminjaman saya">
    <h3>Permohonan peminjaman saya</h3>
    {#each rentals as rental}
      <article class="panel" data-rental-id={rental.id}>
        <span class="badge"
          >{rental.status.toUpperCase()} · {rental.payment_verified
            ? 'Lunas / Terverifikasi'
            : 'Menunggu verifikasi pembayaran'}</span
        >
        <h4>{rental.item_name} × {rental.quantity} unit</h4>
        <p>
          Keperluan: {rental.purpose} · Total biaya:
          <strong>{rupiah(rental.total_fee)}</strong>
        </p>
        <p class="muted">
          Diajukan: {jakarta(rental.requested_at)} · Batas kembali: {jakarta(
            rental.due_at
          )}
        </p>
      </article>
    {:else}
      {#if !loading}<p class="notice">Belum ada permohonan peminjaman.</p>{/if}
    {/each}
    {#if rentalPage.hasMore}
      <button
        class="secondary"
        disabled={busy || loading}
        onclick={() => load('rentals')}>Muat riwayat penyewaan lainnya</button
      >
    {/if}
  </section>
</section>
