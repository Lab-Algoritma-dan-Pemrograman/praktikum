<script lang="ts">
  import { onMount } from 'svelte';
  import { hasPermission, type Principal } from '@algohub/auth-client';
  import {
    SiakadClient,
    permitted,
    reusableCommand,
    versionedCommand,
    PendingCommands,
    emptyPage,
    loadResourcePage,
    mergeResources,
    errorMessage,
    jakarta,
    rupiah,
    type InventoryItem,
    type InventoryRental,
    type InventoryStatus,
    type RentalDecision
  } from '@algohub/siakad-client';
  export let principal: Principal | null;
  export let active = true;
  let mounted = false;
  $: if (mounted && active) void load();
  const client = new SiakadClient();
  let items: InventoryItem[] = [];
  let rentals: InventoryRental[] = [];
  let itemPage = emptyPage<InventoryItem>();
  let rentalPage = emptyPage<InventoryRental>();
  let loadingItems = false;
  let loadingRentals = false;

  let itemId = '';
  let itemVersion = 0;
  let itemCode = '';
  let itemName = '';
  let itemCategory = 'Elektronika';
  let totalQty = 5;
  let availableQty = 5;
  let rentalFee = 25000;
  let itemStatus: InventoryStatus = 'available';
  let institution = '';

  let busy = false;
  let issue = '';
  let notice = '';
  const pending = new PendingCommands();

  $: scope = items.find((item) => item.id === itemId)?.scope ?? {
    domain: 'siakad',
    ...(institution.trim() ? { institution: institution.trim() } : {})
  };
  $: hasInventoryAccess = hasPermission(
    principal?.grants ?? [],
    'inventory.manage',
    'siakad'
  );
  $: canManageInventory = permitted(principal, 'inventory.manage', scope);

  function chooseItem(id: string) {
    const item = items.find((i) => i.id === id);
    itemId = id;
    itemVersion = item?.version ?? 0;
    itemCode = item?.code ?? '';
    itemName = item?.name ?? '';
    itemCategory = item?.category ?? 'Elektronika';
    totalQty = item?.total_quantity ?? 5;
    availableQty = item?.available_quantity ?? 5;
    rentalFee = item?.rental_fee ?? 25000;
    itemStatus = item?.status ?? 'available';
  }

  async function loadItems(more = false) {
    if (loadingItems) return;
    loadingItems = true;
    issue = '';
    const selected = items.find((item) => item.id === itemId);
    try {
      itemPage = await loadResourcePage(
        client,
        '/inventory/items',
        itemPage,
        (row) => row.id,
        more,
        selected ? [selected] : []
      );
      items = itemPage.rows;
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loadingItems = false;
    }
  }

  async function loadRentals(more = false) {
    if (loadingRentals) return;
    loadingRentals = true;
    issue = '';
    const unresolved = rentals.filter(
      (rental) =>
        !!pending.get(
          `/admin/siakad/penyewaan/${encodeURIComponent(rental.id)}`,
          'PATCH'
        )
    );
    try {
      rentalPage = await loadResourcePage(
        client,
        '/admin/siakad/penyewaan',
        rentalPage,
        (row) => row.id,
        more,
        unresolved
      );
      rentals = rentalPage.rows;
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loadingRentals = false;
    }
  }

  async function load() {
    await Promise.all([loadItems(), loadRentals()]);
  }

  async function saveItem() {
    if (busy || !canManageInventory) return;
    busy = true;
    issue = '';
    notice = '';
    const path = `/admin/siakad/inventaris${itemId ? `/${encodeURIComponent(itemId)}` : ''}`;
    const method = itemId ? 'PATCH' : 'POST';
    const mutation = pending.put(
      reusableCommand(pending.get(path, method), path, method, {
        ...(itemId ? { version: itemVersion } : { code: itemCode, scope }),
        name: itemName,
        category: itemCategory,
        total_quantity: totalQty,
        available_quantity: availableQty,
        rental_fee: rentalFee,
        status: itemStatus
      })
    );
    try {
      const saved = await client.execute<InventoryItem>(mutation);
      pending.complete(mutation);
      items = mergeResources(items, [saved], (row) => row.id);
      itemPage = { ...itemPage, rows: items };
      chooseItem(saved.id);
      notice = 'Item inventaris berhasil disimpan.';
    } catch (error) {
      pending.failed(mutation, error);
      issue = errorMessage(error);
      if (!pending.get(path, method) && itemId) {
        issue +=
          ' Muat ulang halaman, pilih kembali alat ini, lalu periksa stok terbaru sebelum memasukkan ulang perubahan Anda.';
      }
    } finally {
      busy = false;
    }
  }

  async function decide(rental: InventoryRental, action: RentalDecision) {
    if (
      busy ||
      !permitted(
        principal,
        action === 'verify_payment' ? 'finance.manage' : 'rental.approve',
        rental.scope
      )
    )
      return;
    busy = true;
    issue = '';
    notice = '';
    const path = `/admin/siakad/penyewaan/${encodeURIComponent(rental.id)}`;
    const mutation = pending.put(
      versionedCommand(pending.get(path, 'PATCH'), path, {
        version: rental.version,
        action
      })
    );
    try {
      const updated = await client.execute<InventoryRental>(mutation);
      pending.complete(mutation);
      rentals = mergeResources(rentals, [updated], (row) => row.id);
      rentalPage = { ...rentalPage, rows: rentals };
      notice = `Penyewaan ${updated.item_name} diperbarui (${updated.status}).`;
      await loadItems();
    } catch (error) {
      pending.failed(mutation, error);
      issue = errorMessage(error);
    } finally {
      busy = false;
    }
  }

  onMount(() => {
    mounted = true;
  });
</script>

<div class="stack">
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if notice}<p class="notice" role="status">{notice}</p>{/if}

  {#if hasInventoryAccess}
    <section class="panel stack">
      <h2>Kelola inventaris laboratorium (K3 / Labops)</h2>
      <label
        >Pilih alat untuk diedit<select
          value={itemId}
          disabled={busy}
          onchange={(e) => chooseItem(e.currentTarget.value)}
        >
          <option value="">Alat baru</option>
          {#each items as item}<option value={item.id}
              >{item.code} — {item.name} ({item.available_quantity}/{item.total_quantity})</option
            >{/each}
        </select></label
      >
      {#if itemPage.hasMore}
        <button
          class="secondary"
          disabled={busy || loadingItems}
          onclick={() => loadItems(true)}>Muat alat lainnya</button
        >
      {/if}
      <form
        class="stack"
        onsubmit={(e) => {
          e.preventDefault();
          void saveItem();
        }}
      >
        <div class="grid">
          {#if !itemId}
            <label
              >Institusi alat (opsional)<input
                bind:value={institution}
                maxlength="100"
              /></label
            >
          {/if}
          <label
            >Kode alat<input
              bind:value={itemCode}
              disabled={!!itemId}
              required
              maxlength="200"
            /></label
          >
          <label
            >Nama alat<input
              bind:value={itemName}
              required
              maxlength="200"
            /></label
          >
          <label
            >Kategori alat<input
              bind:value={itemCategory}
              required
              maxlength="200"
            /></label
          >
          <label
            >Stok total<input
              type="number"
              min="0"
              max="10000"
              bind:value={totalQty}
              required
            /></label
          >
          <label
            >Stok tersedia<input
              type="number"
              min="0"
              max={totalQty}
              bind:value={availableQty}
              required
            /></label
          >
          <label
            >Biaya sewa (Rp)<input
              type="number"
              min="0"
              max="100000000"
              bind:value={rentalFee}
              required
            /></label
          >
          <label
            >Status alat<select bind:value={itemStatus}>
              <option value="available">Tersedia (available)</option>
              <option value="maintenance">Perawatan (maintenance)</option>
              <option value="retired">Nonaktif (retired)</option>
            </select></label
          >
        </div>
        <button disabled={busy || !canManageInventory}
          >{itemId ? 'Simpan alat' : 'Buat alat'}</button
        >
        {#if !canManageInventory}<p class="notice">
            Scope alat ini tidak tercakup grant Anda.
          </p>{/if}
      </form>
    </section>
  {/if}

  <section class="stack" aria-label="Daftar permohonan penyewaan">
    <div class="actions">
      <h2>Permohonan penyewaan alat</h2>
      <button
        class="secondary"
        onclick={load}
        disabled={busy || loadingItems || loadingRentals}
        >Perbarui penyewaan</button
      >
    </div>
    {#each rentals as rental}
      <article class="panel stack" data-admin-rental={rental.id}>
        <span class="badge"
          >{rental.status.toUpperCase()} · {rental.payment_verified
            ? 'Pembayaran terverifikasi'
            : 'Belum diverifikasi'}</span
        >
        <h3>
          {rental.borrower_name} — {rental.item_name} × {rental.quantity} unit
        </h3>
        <p>
          Keperluan: {rental.purpose} · Biaya:
          <strong>{rupiah(rental.total_fee)}</strong> · Batas kembali: {jakarta(
            rental.due_at
          )}
        </p>
        <div class="actions">
          {#if permitted(principal, 'rental.approve', rental.scope) && rental.status === 'requested'}
            <button disabled={busy} onclick={() => decide(rental, 'approve')}
              >Setujui sewa {rental.borrower_name}</button
            >
            <button
              class="danger"
              disabled={busy}
              onclick={() => decide(rental, 'reject')}
              >Tolak sewa {rental.borrower_name}</button
            >
          {/if}
          {#if permitted(principal, 'rental.approve', rental.scope) && rental.status === 'approved'}
            <button
              class="secondary"
              disabled={busy}
              onclick={() => decide(rental, 'return')}
              >Tandai dikembalikan</button
            >
          {/if}
          {#if permitted(principal, 'finance.manage', rental.scope) && !rental.payment_verified && ['approved', 'returned'].includes(rental.status)}
            <button
              class="secondary"
              disabled={busy}
              onclick={() => decide(rental, 'verify_payment')}
              >Verifikasi pembayaran</button
            >
          {/if}
        </div>
      </article>
    {:else}
      <p class="notice">Belum ada permohonan penyewaan alat.</p>
    {/each}
    {#if rentalPage.hasMore}
      <button
        class="secondary"
        disabled={busy || loadingRentals}
        onclick={() => loadRentals(true)}>Muat penyewaan lainnya</button
      >
    {/if}
  </section>
</div>
