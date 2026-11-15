<script lang="ts">
  import { onMount } from 'svelte';
  import type { Principal } from '@algohub/auth-client';
  import {
    ElearningClient,
    permitted,
    reusableCommand,
    errorMessage,
    jakarta,
    type Command,
    type LearningProfile
  } from '@algohub/elearning-client';
  export let principal: Principal | null;
  const client = new ElearningClient();
  let userID = 1;
  let xp = 0;
  let reason = '';
  let busy = false;
  let issue = '';
  let notice = '';
  let profile: LearningProfile | null = null;
  let pending: Command | null = null;

  $: canManage = !!principal?.grants.some(
    (g) => g.permission === 'learning.xp.manage'
  );

  async function load() {
    issue = '';
    try {
      const rows = await client.get<LearningProfile[]>(
        '/admin/elearning/monitoring?limit=100&offset=0'
      );
      profile = rows.find((row) => row.user_id === userID) ?? null;
    } catch (error) {
      issue = errorMessage(error);
    }
  }

  async function mutate(path: string, body: unknown) {
    if (!canManage) return;
    busy = true;
    issue = '';
    notice = '';
    pending = reusableCommand(pending, path, 'POST', body);
    try {
      const updated = await client.execute<LearningProfile>(pending);
      pending = null;
      profile = updated;
      notice = `XP mahasiswa ${updated.user_name} kini ${updated.xp} (Level ${updated.level}).`;
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      busy = false;
    }
  }

  async function setXP(event: SubmitEvent) {
    event.preventDefault();
    await mutate('/admin/elearning/xp/set', {
      user_id: userID,
      xp,
      reason
    });
  }

  async function resetXP(event: SubmitEvent) {
    event.preventDefault();
    await mutate('/admin/elearning/xp/reset', {
      user_id: userID,
      reason
    });
  }
  onMount(() => {
    void load();
  });
</script>

<section class="panel stack">
  <h2>Setel XP mahasiswa</h2>
  <p class="muted">
    Perubahan dicatat pada audit log dengan alasan wajib. Reset mengembalikan XP
    ke nol tanpa menghapus riwayat prestige.
  </p>
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if notice}<p class="notice" role="status">{notice}</p>{/if}
  <form class="stack" onsubmit={setXP}>
    <div class="grid">
      <label
        >ID mahasiswa<input
          type="number"
          min="1"
          bind:value={userID}
          required
          oninput={() => (profile = null)}
        /></label
      >
      <label
        >XP baru<input
          type="number"
          min="0"
          max="1000000"
          bind:value={xp}
          required
        /></label
      >
    </div>
    <label
      >Alasan penyesuaian<textarea
        rows="2"
        bind:value={reason}
        required
        minlength="1"
        maxlength="500"
        placeholder="mis. koreksi XP semester lalu"
      ></textarea></label
    >
    <button disabled={!canManage || busy}>
      {busy ? 'Menyimpan…' : 'Setel XP'}
    </button>
  </form>
  <form class="stack" onsubmit={resetXP}>
    <button class="secondary" disabled={!canManage || busy}>
      Reset XP ke nol
    </button>
  </form>
  {#if profile}
    <p class="muted" data-testid="xp-admin-profile">
      Profil terakhir dimuat: {profile.user_name} · {profile.xp} XP · Level{' '}
      {profile.level} · {jakarta(profile.updated_at)}
    </p>
  {/if}
</section>
