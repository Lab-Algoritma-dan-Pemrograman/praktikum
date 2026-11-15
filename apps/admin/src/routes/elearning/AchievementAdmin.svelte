<script lang="ts">
  import { onMount } from 'svelte';
  import type { Principal } from '@algohub/auth-client';
  import {
    ElearningClient,
    permitted,
    reusableCommand,
    errorMessage,
    type Command,
    type LearningAchievementCatalogItem,
    type LearningAchievementInput,
    type Scope
  } from '@algohub/elearning-client';
  export let principal: Principal | null;
  const client = new ElearningClient();
  let items: LearningAchievementCatalogItem[] = [];
  let selectedId = '';
  let version = 0;
  let title = '';
  let description = '';
  let requirementType: LearningAchievementInput['requirement_type'] = 'manual';
  let requirementValue = '';
  let xpReward = 0;
  let active = true;
  let institution = 'lab';
  let division = 'pendidikan';
  let busy = false;
  let issue = '';
  let notice = '';
  let pending: Command | null = null;

  $: scope = {
    domain: 'elearning',
    ...(institution ? { institution } : {}),
    ...(division ? { division } : {})
  } satisfies Scope;
  $: canSave = permitted(principal, 'learning.content.manage', scope);

  function choose(id: string) {
    const item = items.find((entry) => entry.id === id);
    selectedId = id;
    version = item?.version ?? 0;
    title = item?.title ?? '';
    description = item?.description ?? '';
    requirementType = item?.requirement_type ?? 'manual';
    requirementValue = item?.requirement_value ?? '';
    xpReward = item?.xp_reward ?? 0;
    active = item?.active ?? true;
    institution = item?.scope.institution ?? 'lab';
    division = item?.scope.division ?? 'pendidikan';
  }

  async function load() {
    issue = '';
    try {
      items = await client.get<LearningAchievementCatalogItem[]>(
        '/admin/elearning/achievements'
      );
    } catch (error) {
      issue = errorMessage(error);
    }
  }

  async function save() {
    if (!canSave) return;
    busy = true;
    issue = '';
    notice = '';
    const path = `/admin/elearning/achievements${selectedId ? `/${encodeURIComponent(selectedId)}` : ''}`;
    const method = selectedId ? 'PATCH' : 'POST';
    pending = reusableCommand(pending, path, method, {
      ...(selectedId ? { version } : {}),
      title,
      description,
      requirement_type: requirementType,
      requirement_value: requirementValue,
      xp_reward: xpReward,
      active,
      scope
    });
    try {
      const saved =
        await client.execute<LearningAchievementCatalogItem>(pending);
      pending = null;
      await load();
      choose(saved.id);
      notice = 'Achievement berhasil disimpan.';
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

<section class="panel stack">
  <h2>Kelola katalog achievement</h2>
  <p class="muted">
    Achievement manual ditetapkan koordinator; sisanya dievaluasi server
    otomatis.
  </p>
  {#if issue}<p class="notice error" role="alert">{issue}</p>{/if}
  {#if notice}<p class="notice" role="status">{notice}</p>{/if}
  <label
    >Achievement untuk diedit<select
      value={selectedId}
      onchange={(e) => choose(e.currentTarget.value)}
    >
      <option value="">Achievement baru</option>
      {#each items as item}<option value={item.id}
          >{item.title} ({item.requirement_type} · {item.active
            ? 'aktif'
            : 'nonaktif'})</option
        >{/each}
    </select></label
  >
  <form
    class="stack"
    onsubmit={(e) => {
      e.preventDefault();
      void save();
    }}
  >
    <div class="grid">
      <label
        >Judul achievement<input
          bind:value={title}
          required
          maxlength="200"
        /></label
      >
      <label
        >Jenis syarat<select bind:value={requirementType}>
          <option value="manual">Manual</option>
          <option value="xp">Total XP</option>
          <option value="lesson_count">Jumlah pelajaran selesai</option>
          <option value="streak">Streak harian</option>
          <option value="game_score">Skor Bug Hunt sempurna</option>
          <option value="level_completed">Level selesai</option>
        </select></label
      >
      <label
        >Nilai syarat<input
          bind:value={requirementValue}
          maxlength="128"
          placeholder="mis. 100 (XP) atau level-id"
        /></label
      >
      <label
        >Hadiah XP<input
          type="number"
          min="0"
          max="10000"
          bind:value={xpReward}
          required
        /></label
      >
    </div>
    <label
      >Deskripsi<textarea rows="3" bind:value={description} maxlength="2000"
      ></textarea></label
    >
    <label class="check"
      ><input type="checkbox" bind:checked={active} />Achievement aktif</label
    >
    <button disabled={!canSave || busy}
      >{selectedId ? 'Simpan achievement' : 'Buat achievement'}</button
    >
  </form>
</section>
