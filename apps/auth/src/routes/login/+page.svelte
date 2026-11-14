<script lang="ts">
  export let data: {
    loginToken: string;
    returnTo: string | null;
    fixtures: boolean;
    oauthAvailable: boolean;
  };
  export let form: { message?: string; requestId?: string } | null = null;
</script>

<svelte:head
  ><title>Login — Algohub</title><meta
    name="referrer"
    content="same-origin"
  /></svelte:head
>
<main class="auth-page">
  <a class="brand" href="/">Algohub / Auth Center</a>
  <p class="eyebrow">IDENTITY / SINGLE SIGN-ON</p>
  <h1>Masuk dengan identitas Algohub.</h1>
  <p>Satu login, sesi terpisah untuk setiap aplikasi.</p>
  {#if data.returnTo}<p class="destination">
      Tujuan: <strong>{data.returnTo}</strong>
    </p>{/if}
  {#if form?.message}<div class="error" role="alert">
      <strong>{form.message}</strong>{#if form.requestId}<p>
          Request ID: {form.requestId}
        </p>{/if}
    </div>{/if}
  <form method="POST" action="?/password">
    <input type="hidden" name="login_token" value={data.loginToken} />
    <label for="email">Email</label>
    <input
      id="email"
      name="email"
      type="email"
      autocomplete="username"
      required
      maxlength="254"
    />
    <label for="password">Password</label>
    <input
      id="password"
      name="password"
      type="password"
      autocomplete="current-password"
      required
      maxlength="1024"
    />
    <button type="submit">Masuk</button>
  </form>
  {#if data.oauthAvailable}
    <form method="POST" action="?/oauth">
      <input type="hidden" name="login_token" value={data.loginToken} /><button
        class="secondary"
        type="submit">Masuk dengan OAuth</button
      >
    </form>
  {/if}
  {#if data.fixtures}<p class="note">
      Environment fixture lokal. Gunakan akun pengujian yang disediakan.
    </p>{/if}
  <p class="note">
    Password dan token provider tidak disimpan di browser storage.
  </p>
</main>

<style>
  .auth-page {
    width: min(32rem, 90vw);
    margin: 0 auto;
    padding: clamp(2rem, 8vw, 5rem) 0;
    color: #17324d;
  }
  .brand {
    display: inline-flex;
    min-height: 44px;
    align-items: center;
    color: #245b5a;
    font-weight: 800;
    text-decoration: none;
  }
  .eyebrow {
    margin-top: 2rem;
    color: #326b68;
    font-size: 0.72rem;
    font-weight: 800;
    letter-spacing: 0.12em;
  }
  h1 {
    color: #102a43;
    font-size: clamp(2rem, 6vw, 3.2rem);
    line-height: 1.08;
    letter-spacing: -0.05em;
  }
  p {
    color: #4d6872;
    line-height: 1.65;
  }
  .destination,
  .error {
    overflow-wrap: anywhere;
    font-size: 0.9rem;
  }
  .error {
    padding: 1rem;
    border: 1px solid #d2a08c;
    border-radius: 0.5rem;
    color: #7d3925;
    background: #fff5ef;
  }
  .error p {
    margin-bottom: 0;
    font-size: 0.8rem;
  }
  form {
    display: grid;
    gap: 0.7rem;
    margin-top: 1.5rem;
  }
  label {
    font-weight: 700;
  }
  input {
    width: 100%;
    min-height: 44px;
    padding: 0.7rem;
    border: 1px solid #a7bcb9;
    border-radius: 0.45rem;
    color: #17324d;
    background: white;
    font: inherit;
  }
  button {
    min-height: 44px;
    margin-top: 0.5rem;
    padding: 0.7rem 1rem;
    border: 0;
    border-radius: 0.45rem;
    color: white;
    background: #245b5a;
    font: inherit;
    cursor: pointer;
  }
  .secondary {
    border: 1px solid #a7bcb9;
    color: #245b5a;
    background: white;
  }
  .note {
    font-size: 0.85rem;
  }
</style>
