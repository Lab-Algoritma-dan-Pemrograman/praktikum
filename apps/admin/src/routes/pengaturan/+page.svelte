<script lang="ts">
  import { onMount } from 'svelte';
  import {
    AuthClient,
    asApiError,
    type ApiError,
    type AccessUpdate,
    type DivisionCode,
    type PermissionDefinition,
    type Principal,
    type RoleCode,
    type Scope,
    type UserAccess,
    type UserProfile
  } from '@algohub/auth-client';
  import { RequestError } from '@algohub/ui';
  import ScopeFields from '../../lib/ScopeFields.svelte';
  export let data: {
    principal: Principal;
    users: UserProfile[];
    catalog: PermissionDefinition[];
  };
  type Assignment = {
    role: RoleCode | '';
    division: DivisionCode | '';
    scope: Scope;
  };
  type Grant = Assignment & { permission: string };
  const api = new AuthClient();
  const roles: RoleCode[] = ['koordinator', 'asisten', 'mahasiswa', 'peminjam'];
  const divisions: DivisionCode[] = [
    'sekretaris',
    'bendahara',
    'k3',
    'labops',
    'pendidikan',
    'pdd'
  ];
  let users: UserProfile[] = [];
  let offset = 0;
  let selectedId = 0;
  let version: number | null = null;
  let assignments: Assignment[] = [];
  let grants: Grant[] = [];
  let busy = true;
  onMount(() => {
    busy = false;
  });
  let stale = false;
  let issue: ApiError | null = null;
  let notice = '';
  $: {
    users = data.users;
    offset = 0;
  }
  $: self = selectedId === data.principal.user.id;
  $: selected = users.find((user) => user.id === selectedId);

  function clearEditor() {
    version = null;
    assignments = [];
    grants = [];
    stale = false;
    notice = '';
    issue = null;
  }
  function useAccess(access: UserAccess) {
    version = access.version;
    assignments = access.assignments
      .filter((assignment) => assignment.active)
      .map((assignment) => ({
        role: assignment.role,
        division: assignment.division ?? '',
        scope: { ...assignment.scope }
      }));
    grants = access.grants.map((grant) => ({
      permission: grant.permission,
      role: grant.role,
      division: grant.division ?? '',
      scope: { ...grant.scope }
    }));
    stale = false;
  }
  async function loadAccess() {
    if (!selectedId) return;
    busy = true;
    issue = null;
    notice = '';
    try {
      useAccess(await api.userAccess(selectedId));
    } catch (error) {
      issue = asApiError(error);
    } finally {
      busy = false;
    }
  }
  async function loadUsers(next: number) {
    busy = true;
    issue = null;
    try {
      users = await api.users(next);
      offset = next;
      selectedId = 0;
      clearEditor();
    } catch (error) {
      issue = asApiError(error);
    } finally {
      busy = false;
    }
  }
  function cleanAssignment(row: Assignment) {
    return {
      role: row.role as RoleCode,
      ...(row.division ? { division: row.division } : {}),
      scope: Object.fromEntries(
        Object.entries(row.scope).filter(
          ([, value]) => value !== '' && value !== undefined && value !== 0
        )
      ) as Scope
    };
  }
  async function save() {
    if (version === null || self || stale) return;
    busy = true;
    issue = null;
    notice = '';
    const update: AccessUpdate = {
      version,
      assignments: assignments.map(cleanAssignment),
      grants: grants.map((grant) => ({
        ...cleanAssignment(grant),
        permission: grant.permission
      }))
    };
    try {
      useAccess(await api.updateAccess(selectedId, update));
      notice =
        'Akses berhasil disimpan. Permission terbaru diperiksa pada permintaan berikutnya.';
    } catch (error) {
      issue = asApiError(error);
      stale = issue.status === 409;
    } finally {
      busy = false;
    }
  }
</script>

<svelte:head><title>Pengaturan akses — Algohub Admin</title></svelte:head>
<section class="identity-page">
  <p class="eyebrow">IDENTITY / PENGATURAN</p>
  <h1>Pengaturan akses</h1>
  <p>
    Hanya koordinator global dapat mengubah penugasan dan grant eksplisit.
    Pengguna baru tidak otomatis memiliki role.
  </p>
  <section class="identity-panel" aria-labelledby="users-title">
    <h2 id="users-title">Pilih pengguna</h2>
    <label for="access-user"
      >Pengguna<select
        id="access-user"
        bind:value={selectedId}
        disabled={busy}
        onchange={clearEditor}
        ><option value={0}>Pilih pengguna</option>{#each users as user}<option
            value={user.id}
            >{user.name} — {user.email}{user.id === data.principal.user.id
              ? ' (Anda)'
              : ''}</option
          >{/each}</select
      ></label
    >
    {#if !users.length}<p>Belum ada pengguna pada halaman ini.</p>{/if}
    <div class="actions">
      <button type="button" disabled={!selectedId || busy} onclick={loadAccess}
        >{busy ? 'Memuat…' : 'Muat akses'}</button
      ><button
        class="secondary"
        type="button"
        disabled={busy || offset === 0}
        onclick={() => loadUsers(Math.max(0, offset - 25))}
        >Pengguna sebelumnya</button
      ><button
        class="secondary"
        type="button"
        disabled={busy || users.length < 25}
        onclick={() => loadUsers(offset + 25)}>Pengguna berikutnya</button
      >
    </div>
  </section>
  <RequestError
    error={issue}
    onRetry={selectedId ? loadAccess : () => loadUsers(offset)}
    returnTo="/pengaturan"
  />
  {#if notice}<p class="notice" role="status">{notice}</p>{/if}
  {#if version !== null}
    <section
      class="identity-panel"
      aria-labelledby="editor-title"
      aria-busy={busy}
    >
      <h2 id="editor-title">Akses {selected?.name}</h2>
      <p class="muted">
        Versi akses: {version}. Scope kosong berarti tidak dibatasi pada level
        tersebut. Gunakan identifier scope yang sudah disepakati; halaman ini
        tidak membuat resource domain.
      </p>
      {#if self}<p class="notice">
          Anda tidak dapat mengubah akses sendiri untuk mencegah kehilangan
          akses.
        </p>{/if}
      <form
        onsubmit={(event) => {
          event.preventDefault();
          void save();
        }}
      >
        <fieldset class="editor" disabled={busy || self}>
          <legend>Penugasan role</legend>
          {#if !assignments.length}<p>Belum ada penugasan role.</p>{/if}
          {#each assignments as assignment, index}
            <fieldset>
              <legend>Penugasan {index + 1}</legend>
              <div class="form-grid">
                <label for={`assignment-role-${index}`}
                  >Role<select
                    id={`assignment-role-${index}`}
                    bind:value={assignment.role}
                    required
                    onchange={() => (assignment.division = '')}
                    ><option value="">Pilih role</option
                    >{#each roles as role}<option value={role}>{role}</option
                      >{/each}</select
                  ></label
                ><label for={`assignment-division-${index}`}
                  >Divisi asisten<select
                    id={`assignment-division-${index}`}
                    bind:value={assignment.division}
                    disabled={assignment.role !== 'asisten'}
                    ><option value="">Tanpa divisi</option
                    >{#each divisions as division}<option value={division}
                        >{division}</option
                      >{/each}</select
                  ></label
                >
              </div>
              <ScopeFields
                bind:scope={assignment.scope}
                idPrefix={`assignment-${index}`}
              />
              <div class="actions">
                <button
                  class="danger"
                  type="button"
                  aria-label={`Hapus penugasan ${index + 1}`}
                  onclick={() =>
                    (assignments = assignments.filter((_, at) => at !== index))}
                  >Hapus penugasan</button
                >
              </div>
            </fieldset>
          {/each}
          <button
            class="secondary"
            type="button"
            onclick={() =>
              (assignments = [
                ...assignments,
                { role: '', division: '', scope: {} }
              ])}>Tambah penugasan</button
          >
        </fieldset>
        <fieldset class="editor" disabled={busy || self}>
          <legend>Grant permission eksplisit</legend>
          <p class="muted">
            Role dan divisi grant harus sesuai dengan penugasan pengguna. Server
            memvalidasi batas permission dan scope.
          </p>
          {#if !grants.length}<p>Belum ada grant eksplisit.</p>{/if}
          {#each grants as grant, index}
            <fieldset>
              <legend>Grant {index + 1}</legend>
              <label for={`grant-permission-${index}`}
                >Permission<select
                  id={`grant-permission-${index}`}
                  bind:value={grant.permission}
                  required
                  ><option value="">Pilih permission</option
                  >{#each data.catalog as permission}<option
                      value={permission.code}
                      >{permission.code} — {permission.description}</option
                    >{/each}</select
                ></label
              >
              <div class="form-grid">
                <label for={`grant-role-${index}`}
                  >Role grant<select
                    id={`grant-role-${index}`}
                    bind:value={grant.role}
                    required
                    onchange={() => (grant.division = '')}
                    ><option value="">Pilih role</option
                    >{#each roles as role}<option value={role}>{role}</option
                      >{/each}</select
                  ></label
                ><label for={`grant-division-${index}`}
                  >Divisi grant<select
                    id={`grant-division-${index}`}
                    bind:value={grant.division}
                    disabled={grant.role !== 'asisten'}
                    ><option value="">Tanpa divisi</option
                    >{#each divisions as division}<option value={division}
                        >{division}</option
                      >{/each}</select
                  ></label
                >
              </div>
              <ScopeFields
                bind:scope={grant.scope}
                idPrefix={`grant-${index}`}
              />
              <div class="actions">
                <button
                  class="danger"
                  type="button"
                  aria-label={`Hapus grant ${index + 1}`}
                  onclick={() =>
                    (grants = grants.filter((_, at) => at !== index))}
                  >Hapus grant</button
                >
              </div>
            </fieldset>
          {/each}
          <button
            class="secondary"
            type="button"
            onclick={() =>
              (grants = [
                ...grants,
                { permission: '', role: '', division: '', scope: {} }
              ])}>Tambah grant</button
          >
        </fieldset>
        <div class="actions">
          <button type="submit" disabled={busy || stale || self}
            >Simpan perubahan akses</button
          ><button
            class="secondary"
            type="button"
            disabled={busy}
            onclick={loadAccess}>Muat ulang akses</button
          >
        </div>
      </form>
    </section>
  {/if}
</section>
