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

<main class="login-page">
  <div class="shell">
    <a class="brand" href="/"
      ><span class="brand-mark" aria-hidden="true"
        ><span></span><span></span><span></span><span></span></span
      ><span>Algohub <span class="brand-divider">/</span> Auth Center</span></a
    >

    <section class="card" aria-labelledby="login-title">
      <span class="medallion" aria-hidden="true">
        <svg
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.6"
          ><rect x="4.5" y="10" width="15" height="10" rx="2.5" /><path
            d="M8.5 10V7.5a3.5 3.5 0 0 1 7 0V10"
          /><circle
            cx="12"
            cy="15"
            r="1.2"
            fill="currentColor"
            stroke="none"
          /></svg
        >
      </span>

      <p class="eyebrow">Identity / single sign-on</p>
      <h1 id="login-title">Masuk dengan identitas Algohub.</h1>
      <p class="lead">Satu login, sesi terpisah untuk setiap aplikasi.</p>

      {#if data.returnTo}
        <p class="destination">Tujuan: <strong>{data.returnTo}</strong></p>
      {/if}

      {#if form?.message}
        <div class="error" role="alert">
          <strong>{form.message}</strong>
          {#if form.requestId}<p>Request ID: {form.requestId}</p>{/if}
        </div>
      {/if}

      <form method="POST" action="?/password">
        <input type="hidden" name="login_token" value={data.loginToken} />
        <div class="field">
          <label for="email">Email atau NIM</label>
          <div class="control">
            <span class="icon" aria-hidden="true">
              <svg
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="1.6"
                ><rect x="3" y="5.5" width="18" height="13" rx="2.5" /><path
                  d="m4 7 8 6 8-6"
                /></svg
              >
            </span>
            <input
              id="email"
              name="email"
              type="text"
              inputmode="email"
              autocomplete="username"
              placeholder="nama@algohub.web.id atau NIM"
              required
              maxlength="254"
            />
          </div>
        </div>

        <div class="field">
          <label for="password">Password</label>
          <div class="control">
            <span class="icon" aria-hidden="true">
              <svg
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="1.6"
                ><rect x="5" y="10.5" width="14" height="9" rx="2.2" /><path
                  d="M8.5 10.5V8a3.5 3.5 0 0 1 7 0v2.5"
                /></svg
              >
            </span>
            <input
              id="password"
              name="password"
              type="password"
              autocomplete="current-password"
              required
              maxlength="1024"
            />
          </div>
        </div>

        <button class="submit" type="submit">Masuk</button>
      </form>

      {#if data.oauthAvailable}
        <form method="POST" action="?/oauth">
          <input type="hidden" name="login_token" value={data.loginToken} />
          <button class="secondary" type="submit">Masuk dengan OAuth</button>
        </form>
      {/if}

      <div class="notes">
        {#if data.fixtures}
          <p>
            Environment fixture lokal. Gunakan akun pengujian yang disediakan.
          </p>
        {/if}
        <p>Password dan token provider tidak disimpan di browser storage.</p>
      </div>
    </section>

    <p class="back"><a href="/">Kembali ke portal</a></p>
  </div>
</main>

<style>
  .login-page {
    display: grid;
    min-height: 100vh;
    place-items: center;
    padding: clamp(1.5rem, 5vw, 3rem) 1.25rem;
    background:
      radial-gradient(
        60rem 30rem at 15% -10%,
        color-mix(in srgb, var(--ah-rust) 9%, transparent),
        transparent 70%
      ),
      var(--ah-cream);
  }

  .shell {
    width: min(26rem, 100%);
  }

  .brand {
    display: inline-flex;
    min-height: 44px;
    align-items: center;
    gap: 0.6rem;
    color: var(--ah-ink);
    font-weight: 600;
    letter-spacing: -0.02em;
    text-decoration: none;
  }

  .brand-mark {
    display: grid;
    width: 1.9rem;
    height: 2.2rem;
    grid-template-columns: repeat(2, 1fr);
    gap: 0.13rem;
    padding: 0.26rem;
    border-radius: 50% 50% 0.4rem 0.4rem / 60% 60% 0 0;
    background: var(--ah-rust);
  }

  .brand-mark span {
    border-radius: 50%;
    background: var(--ah-cream);
  }

  .brand-mark span:nth-child(2) {
    border-radius: 50% 50% 0 50%;
    background: var(--ah-ink);
  }

  .brand-mark span:nth-child(3) {
    border-radius: 50% 0 50% 50%;
    background: var(--ah-blue);
  }

  .brand-divider {
    color: var(--ah-muted);
    font-weight: 400;
  }

  .card {
    margin-top: 1.25rem;
    padding: clamp(1.5rem, 5vw, 2.25rem);
    border: 1px solid var(--ah-console-line);
    border-radius: var(--ah-console-radius-card);
    background: var(--ah-console-card);
    box-shadow: var(--ah-shadow-float);
    color: var(--ah-console-text);
  }

  .medallion {
    display: grid;
    width: 3.2rem;
    height: 3.2rem;
    margin-bottom: 1.35rem;
    place-items: center;
    border-radius: var(--ah-console-radius-card);
    color: var(--ah-console-accent);
    background: var(--ah-console-accent-soft);
  }

  .medallion svg {
    width: 1.6rem;
    height: 1.6rem;
  }

  .eyebrow {
    margin: 0 0 0.6rem;
    color: var(--ah-console-accent);
    font-size: 0.75rem;
    font-weight: 700;
    letter-spacing: 0.14em;
    text-transform: uppercase;
  }

  h1 {
    margin: 0;
    color: var(--ah-console-heading);
    font-size: clamp(1.7rem, 4.6vw, 2.1rem);
    font-weight: 500;
    line-height: 1.1;
    letter-spacing: -0.035em;
  }

  .lead {
    margin: 0.7rem 0 0;
    color: var(--ah-console-muted);
    font-size: 0.95rem;
    line-height: 1.55;
  }

  .destination {
    margin: 1rem 0 0;
    padding: 0.55rem 0.75rem;
    border-radius: var(--ah-console-radius-control);
    background: var(--ah-console-sunken);
    color: var(--ah-console-muted);
    font-size: 0.85rem;
    overflow-wrap: anywhere;
  }

  .error {
    margin-top: 1rem;
    padding: 0.85rem 0.9rem;
    border: 1px solid var(--ah-console-danger-line);
    border-radius: var(--ah-console-radius-control);
    color: var(--ah-console-danger-ink);
    background: var(--ah-console-danger-soft);
    font-size: 0.9rem;
    overflow-wrap: anywhere;
  }

  .error p {
    margin: 0.35rem 0 0;
    font-size: 0.78rem;
  }

  form {
    display: grid;
    gap: 0.9rem;
    margin-top: 1.25rem;
  }

  .field {
    display: grid;
    gap: 0.4rem;
  }

  label {
    color: var(--ah-console-text);
    font-size: 0.85rem;
    font-weight: 600;
  }

  .control {
    position: relative;
    display: flex;
    align-items: center;
  }

  .icon {
    position: absolute;
    left: 0.8rem;
    display: grid;
    place-items: center;
    color: var(--ah-console-muted);
    pointer-events: none;
  }

  .icon svg {
    width: 1.05rem;
    height: 1.05rem;
  }

  input {
    width: 100%;
    min-height: 46px;
    padding: 0.7rem 0.9rem 0.7rem 2.5rem;
    border: 1px solid var(--ah-console-line-strong);
    border-radius: var(--ah-console-radius-control);
    color: var(--ah-console-text);
    background: var(--ah-console-sunken);
    font: inherit;
  }

  input::placeholder {
    color: color-mix(in srgb, var(--ah-console-muted) 75%, transparent);
  }

  input:focus-visible {
    border-color: var(--ah-console-accent);
    outline: 2px solid var(--ah-console-accent);
    outline-offset: 1px;
    background: var(--ah-console-card);
  }

  button {
    min-height: 46px;
    padding: 0.7rem 1rem;
    border: 0;
    border-radius: var(--ah-console-radius-control);
    font: inherit;
    font-weight: 600;
    cursor: pointer;
    transition:
      transform var(--ah-motion-fast) ease,
      background-color var(--ah-motion-fast) ease;
  }

  .submit {
    margin-top: 0.25rem;
    color: var(--ah-console-card);
    background: var(--ah-console-accent);
  }

  .submit:hover,
  .submit:focus-visible {
    background: var(--ah-console-accent-strong);
  }

  .submit:active {
    transform: translateY(1px);
  }

  .secondary {
    width: 100%;
    margin-top: 0.75rem;
    border: 1px solid var(--ah-console-line-strong);
    color: var(--ah-console-accent);
    background: var(--ah-console-card);
  }

  .secondary:hover,
  .secondary:focus-visible {
    background: var(--ah-console-accent-soft);
  }

  .notes {
    display: grid;
    gap: 0.35rem;
    margin-top: 1.25rem;
    padding-top: 1rem;
    border-top: 1px solid var(--ah-console-line-subtle);
  }

  .notes p {
    margin: 0;
    color: var(--ah-console-muted);
    font-size: 0.78rem;
    line-height: 1.5;
  }

  .back {
    margin: 1rem 0 0;
    text-align: center;
  }

  .back a {
    display: inline-flex;
    min-height: 44px;
    align-items: center;
    color: var(--ah-console-muted);
    font-size: 0.85rem;
  }

  .back a:hover,
  .back a:focus-visible {
    color: var(--ah-console-accent);
  }
</style>
