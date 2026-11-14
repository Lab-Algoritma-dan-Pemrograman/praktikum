<script lang="ts">
  import type { Scope } from '@algohub/auth-client';
  export let scope: Scope;
  export let idPrefix: string;
  const fields = [
    { key: 'domain', label: 'Domain' },
    { key: 'institution', label: 'Institusi' },
    { key: 'division', label: 'Divisi scope' },
    { key: 'class_id', label: 'ID kelas' },
    { key: 'session_id', label: 'ID sesi' }
  ] as const;
</script>

<div class="scope-fields">
  {#each fields as field}<label for={`${idPrefix}-${field.key}`}
      >{field.label}<input
        id={`${idPrefix}-${field.key}`}
        bind:value={scope[field.key]}
        maxlength="100"
        placeholder="Tidak dibatasi"
      /></label
    >{/each}
  <label for={`${idPrefix}-shift`}
    >Shift<input
      id={`${idPrefix}-shift`}
      type="number"
      min="0"
      max="100"
      step="1"
      bind:value={scope.shift}
      placeholder="0 / tidak dibatasi"
    /></label
  >
</div>

<style>
  .scope-fields {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.8rem;
    margin-top: 1rem;
  }
  @media (max-width: 900px) {
    .scope-fields {
      grid-template-columns: 1fr;
    }
  }
</style>
