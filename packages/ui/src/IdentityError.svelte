<script lang="ts">
  import './tokens.css';
  import { safeRelativePath } from '@algohub/auth-client';
  export let status: number;
  export let message: string;
  export let path = '/';
  $: title =
    status === 401
      ? 'Sesi berakhir atau belum masuk'
      : status === 403
        ? 'Akses tidak diizinkan'
        : status === 404
          ? 'Halaman tidak tersedia'
          : status >= 500
            ? 'Layanan belum tersedia'
            : 'Permintaan login tidak valid';
</script>

<svelte:head
  ><title>{title} — Algohub</title><meta
    name="robots"
    content="noindex"
  /></svelte:head
>
<main class="error-page">
  <a class="brand" href="/">Algohub</a>
  <section aria-labelledby="error-title">
    <p class="code">{status} / IDENTITY</p>
    <h1 id="error-title">{title}</h1>
    <p>{message}</p>
    {#if status === 401 || status < 403}
      <a
        class="button"
        href={`/api/auth/start?${new URLSearchParams({ return_to: safeRelativePath(path) ?? '/' })}`}
        >Masuk kembali</a
      >
    {:else if status >= 500}
      <button class="button" type="button" onclick={() => location.reload()}
        >Coba lagi</button
      >
    {/if}
    <a class="secondary" href="/">Kembali ke beranda</a>
  </section>
</main>

<style>
  .error-page {
    width: min(44rem, 90vw);
    margin: 0 auto;
    padding: clamp(2rem, 8vw, 6rem) 0;
    color: #17324d;
    overflow-wrap: anywhere;
  }
  .brand {
    color: #245b5a;
    font-weight: 800;
  }
  section {
    margin-top: 3rem;
    padding: clamp(1rem, 4vw, 2rem);
    border: 1px solid #d6e1e3;
    border-radius: 0.8rem;
    background: #fff;
  }
  .code {
    color: #8b452e;
    font-size: 0.8rem;
    font-weight: 800;
    letter-spacing: 0.1em;
  }
  h1 {
    font-size: clamp(1.8rem, 5vw, 3rem);
    letter-spacing: -0.04em;
    line-height: 1.1;
  }
  p {
    line-height: 1.65;
  }
  .button,
  .secondary {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-height: 44px;
    margin: 0.7rem 0.7rem 0 0;
    padding: 0.7rem 1rem;
    border: 0;
    border-radius: 0.45rem;
    font: inherit;
    text-decoration: none;
    cursor: pointer;
  }
  .button {
    color: white;
    background: #245b5a;
  }
  .secondary {
    color: #245b5a;
    background: #eef7f5;
  }
</style>
