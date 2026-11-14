<script lang="ts">
  import { onMount } from 'svelte';
  import { hasPermission, type AppIdentityData } from '@algohub/auth-client';
  import '@algohub/siakad-client/styles.css';
  import {
    SiakadClient,
    emptyPage,
    loadResourcePage,
    mergeResources,
    errorMessage,
    type AcademicClass
  } from '@algohub/siakad-client';
  import SchedulesAttendanceAdmin from './SchedulesAttendanceAdmin.svelte';
  import InventoryRentalsAdmin from './InventoryRentalsAdmin.svelte';
  import ClassesAdmin from './ClassesAdmin.svelte';
  import ReportsAdmin from './ReportsAdmin.svelte';
  export let data: AppIdentityData;
  const client = new SiakadClient();
  let classes: AcademicClass[] = [];
  let scheduleClassId = '';
  let classPage = emptyPage<AcademicClass>();
  let tab =
    hasPermission(data.principal?.grants ?? [], 'schedule.write', 'siakad') ||
    hasPermission(data.principal?.grants ?? [], 'attendance.read', 'siakad')
      ? 'schedules'
      : ['inventory.manage', 'rental.approve', 'finance.manage'].some(
            (permission) =>
              hasPermission(data.principal?.grants ?? [], permission, 'siakad')
          )
        ? 'inventory'
        : hasPermission(data.principal?.grants ?? [], 'class.manage', 'siakad')
          ? 'classes'
          : 'reports';
  let ready = false;
  let schedulesMounted = false;
  let inventoryMounted = false;
  let loading = false;

  $: if (ready && tab === 'schedules') schedulesMounted = true;
  $: if (ready && tab === 'inventory') inventoryMounted = true;
  let issue = '';

  $: grants = data.principal?.grants ?? [];
  $: canClasses = hasPermission(grants, 'class.manage', 'siakad');
  $: canSchedules =
    hasPermission(grants, 'schedule.write', 'siakad') ||
    hasPermission(grants, 'attendance.read', 'siakad');
  $: canInventory = [
    'inventory.manage',
    'rental.approve',
    'finance.manage'
  ].some((permission) => hasPermission(grants, permission, 'siakad'));
  $: canAudit = hasPermission(grants, 'audit.read', 'siakad');

  async function loadClasses(more = false, retained: AcademicClass[] = []) {
    if (loading || (!canSchedules && !canClasses)) return;
    loading = true;
    issue = '';
    const selected = classes.find((row) => row.id === scheduleClassId);
    try {
      classPage = await loadResourcePage(
        client,
        '/admin/siakad/kelas',
        classPage,
        (row) => row.id,
        more,
        mergeResources(selected ? [selected] : [], retained, (row) => row.id)
      );
      classes = classPage.rows;
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    ready = true;
    void loadClasses();
  });
</script>

<svelte:head><title>Admin SIAKAD — Algohub</title></svelte:head>
<section class="siakad stack">
  <h1>Konfigurasi SIAKAD & laboratorium</h1>
  <p>
    Pengelolaan kelas, jadwal, presensi QR, inventaris, penyewaan alat, dan
    laporan berjalan sesuai permission divisi serta scope akun.
  </p>
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if loading}<p role="status">Memuat Admin SIAKAD…</p>{/if}

  <nav aria-label="Panel Admin SIAKAD">
    {#if canSchedules}<button
        disabled={!ready}
        class:secondary={tab !== 'schedules'}
        aria-pressed={tab === 'schedules'}
        onclick={() => (tab = 'schedules')}>Jadwal & Presensi</button
      >{/if}
    {#if canInventory}<button
        disabled={!ready}
        class:secondary={tab !== 'inventory'}
        aria-pressed={tab === 'inventory'}
        onclick={() => (tab = 'inventory')}>Inventaris & Penyewaan</button
      >{/if}
    {#if canClasses}<button
        disabled={!ready}
        class:secondary={tab !== 'classes'}
        aria-pressed={tab === 'classes'}
        onclick={() => (tab = 'classes')}>Kelas</button
      >{/if}
    <button
      disabled={!ready}
      class:secondary={tab !== 'reports'}
      aria-pressed={tab === 'reports'}
      onclick={() => (tab = 'reports')}>Laporan & Export</button
    >
    {#if canAudit}<a href="/auditlog">Audit sesuai scope</a>{/if}
  </nav>

  {#if (tab === 'classes' || tab === 'schedules') && classPage.hasMore}
    <button
      class="secondary"
      disabled={loading}
      onclick={() => loadClasses(true)}>Muat kelas lainnya</button
    >
  {/if}

  {#if canSchedules && schedulesMounted}
    <div
      hidden={tab !== 'schedules'}
      style:display={tab === 'schedules' ? 'contents' : 'none'}
    >
      <SchedulesAttendanceAdmin
        {classes}
        bind:classId={scheduleClassId}
        principal={data.principal}
        active={tab === 'schedules'}
      />
    </div>
  {/if}
  {#if canInventory && inventoryMounted}
    <div
      hidden={tab !== 'inventory'}
      style:display={tab === 'inventory' ? 'contents' : 'none'}
    >
      <InventoryRentalsAdmin
        principal={data.principal}
        active={tab === 'inventory'}
      />
    </div>
  {/if}
  {#if canClasses}
    <div
      hidden={tab !== 'classes'}
      style:display={tab === 'classes' ? 'contents' : 'none'}
    >
      <ClassesAdmin
        bind:classes
        principal={data.principal}
        refresh={(saved) => loadClasses(false, [saved])}
      />
    </div>
  {/if}
  {#if tab === 'reports'}
    <ReportsAdmin principal={data.principal} />
  {/if}
</section>
