<script lang="ts">
  import './tokens.css';
  import type { ApiError } from '@algohub/auth-client';
  export let error: ApiError | null = null;
  export let onRetry: (() => void) | undefined = undefined;
  export let returnTo = '/';
</script>

{#if error}
  <div class="request-error" role="alert">
    <strong
      >{error.status === 401
        ? 'Sesi berakhir. Silakan masuk kembali.'
        : error.status === 409
          ? 'Data akses telah berubah. Muat ulang sebelum menyimpan.'
          : error.message}</strong
    >
    {#if error.requestId}<p class="request-id">
        Request ID: {error.requestId}
      </p>{/if}
    {#if error.status === 401}
      <a
        href={`/api/auth/start?${new URLSearchParams({ return_to: returnTo })}`}
        >Masuk kembali</a
      >
    {:else if onRetry}
      <button type="button" onclick={onRetry}>Muat ulang</button>
    {/if}
  </div>
{/if}

<style>
  .request-error {
    margin: 1rem 0;
    padding: 1rem;
    border: 1px solid #dab8a7;
    border-radius: 0.5rem;
    color: #7d3925;
    background: #fff5ef;
    overflow-wrap: anywhere;
  }
  .request-id {
    font-size: 0.8rem;
  }
  a,
  button {
    display: inline-flex;
    align-items: center;
    min-height: 44px;
    color: #245b5a;
    font: inherit;
  }
  button {
    padding: 0.4rem 0.8rem;
    border: 1px solid #a7bcb9;
    border-radius: 0.4rem;
    background: white;
    cursor: pointer;
  }
</style>
