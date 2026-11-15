<script lang="ts">
  import { SiteFooter } from '@algohub/ui';

  export let data: {
    loginToken: string;
    returnTo: string | null;
    fixtures: boolean;
    oauthAvailable: boolean;
  };
  export let form: { message?: string; requestId?: string } | null = null;

  const footerLinks = [
    { label: 'Portal', href: '/' },
    { label: 'Panduan', href: '/' },
    { label: 'Bantuan', href: '/' }
  ];
</script>

<svelte:head
  ><title>Masuk — Algohub</title><meta
    name="referrer"
    content="same-origin"
  /></svelte:head
>

<div class="login-page">
  <main class="login-main">
    <div class="column">
      <a class="brand" href="/" aria-label="Algohub beranda">
        <img
          src="/logo-mark.png"
          alt=""
          width="55"
          height="96"
          decoding="async"
        />
        <span class="wordmark"
          ><span class="wordmark-algo">Algo</span><span class="wordmark-hub"
            >Hub</span
          ></span
        >
      </a>

      <h1 id="login-title">Masuk ke Algohub</h1>

      {#if data.oauthAvailable}
        <form method="POST" action="?/oauth" class="sso-form">
          <input type="hidden" name="login_token" value={data.loginToken} />
          <button class="sso" type="submit">
            <span class="sso-icon" aria-hidden="true">
              <svg
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="1.6"
                ><path
                  d="M12 3.5 5 6.5v5c0 4.2 2.9 7.4 7 9 4.1-1.6 7-4.8 7-9v-5z"
                /><path d="m9 12 2.2 2.2L15.5 10" /></svg
              >
            </span>
            Masuk dengan SSO kampus
          </button>
        </form>

        <div class="divider" role="separator" aria-label="atau">
          <span>atau</span>
        </div>
      {/if}

      {#if form?.message}
        <div class="error" role="alert">
          <strong>{form.message}</strong>
          {#if form.requestId}<p>Request ID: {form.requestId}</p>{/if}
        </div>
      {/if}

      <form method="POST" action="?/password" class="password-form">
        <input type="hidden" name="login_token" value={data.loginToken} />

        <div class="field">
          <label for="email">Email atau NIM</label>
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

        <div class="field">
          <label for="password">Password</label>
          <input
            id="password"
            name="password"
            type="password"
            autocomplete="current-password"
            required
            maxlength="1024"
          />
        </div>

        <button class="submit" type="submit">Masuk</button>
      </form>

      <p class="prompt">Belum punya akun? Hubungi admin laboratorium.</p>

      {#if data.fixtures}
        <p class="fixtures">
          Environment fixture lokal. Gunakan akun pengujian yang disediakan.
        </p>
      {/if}
    </div>
  </main>

  <SiteFooter links={footerLinks} />
</div>

<style>
  .login-page {
    display: flex;
    flex-direction: column;
    min-height: 100vh;
    min-height: 100dvh;
    background: var(--ah-cream);
  }

  .login-main {
    display: flex;
    flex: 1;
    align-items: center;
    justify-content: center;
    padding: clamp(2rem, 6vw, 3.5rem) 1.25rem;
  }

  .column {
    display: grid;
    width: min(21.5rem, 100%);
    gap: 0.9rem;
    justify-items: stretch;
  }

  .brand {
    display: inline-flex;
    width: fit-content;
    min-height: 44px;
    align-items: center;
    gap: 0.5rem;
    margin-bottom: 0.35rem;
    text-decoration: none;
  }

  .brand img {
    height: 1.9rem;
    width: auto;
  }

  .wordmark {
    display: inline-flex;
    align-items: baseline;
    font-family: 'Fredoka', ui-rounded, 'Segoe UI', system-ui, sans-serif;
    font-size: 1.3rem;
    font-weight: 500;
    letter-spacing: -0.02em;
    line-height: 1;
  }

  .wordmark-algo {
    color: var(--ah-ink);
  }

  .wordmark-hub {
    color: var(--ah-coral);
  }

  h1 {
    margin: 0 0 0.35rem;
    color: var(--ah-ink);
    font-size: clamp(1.4rem, 4vw, 1.6rem);
    font-weight: 500;
    line-height: 1.2;
    letter-spacing: -0.025em;
  }

  .sso-form {
    display: grid;
  }

  .sso {
    display: inline-flex;
    min-height: 44px;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    padding: 0.6rem 1rem;
    border: 1px solid var(--ah-line-strong);
    border-radius: 0.625rem;
    color: var(--ah-ink);
    background: #f4f1ee;
    font: inherit;
    font-size: 0.9rem;
    font-weight: 600;
    cursor: pointer;
  }

  .sso:hover,
  .sso:focus-visible {
    background: #ffffff;
  }

  .sso-icon {
    display: grid;
    place-items: center;
    color: var(--ah-ink);
  }

  .sso-icon svg {
    width: 1.05rem;
    height: 1.05rem;
  }

  .divider {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    margin: 0.35rem 0;
    color: var(--ah-muted);
    font-size: 0.78rem;
  }

  .divider::before,
  .divider::after {
    flex: 1;
    height: 1px;
    background: var(--ah-line);
    content: '';
  }

  .error {
    padding: 0.8rem 0.85rem;
    border: 1px solid var(--ah-console-danger-line);
    border-radius: 0.625rem;
    color: var(--ah-console-danger-ink);
    background: var(--ah-console-danger-soft);
    font-size: 0.875rem;
    overflow-wrap: anywhere;
  }

  .error p {
    margin: 0.3rem 0 0;
    font-size: 0.75rem;
  }

  .password-form {
    display: grid;
    gap: 0.9rem;
    margin-top: 0.35rem;
  }

  .field {
    display: grid;
    gap: 0.35rem;
  }

  label {
    color: var(--ah-muted);
    font-size: 0.8rem;
    font-weight: 600;
  }

  input {
    width: 100%;
    min-height: 44px;
    padding: 0.6rem 0.85rem;
    border: 1px solid transparent;
    border-radius: 0.625rem;
    color: var(--ah-ink);
    background: #f4f1ee;
    font: inherit;
    font-size: 0.95rem;
  }

  input::placeholder {
    color: color-mix(in srgb, var(--ah-muted) 80%, transparent);
  }

  input:hover {
    background: #f0ece9;
  }

  input:focus-visible {
    border-color: var(--ah-coral);
    outline: 2px solid var(--ah-coral);
    outline-offset: 1px;
    background: #ffffff;
  }

  .submit {
    min-height: 44px;
    margin-top: 0.35rem;
    padding: 0.6rem 1rem;
    border: 0;
    border-radius: 0.625rem;
    color: #ffffff;
    background: var(--ah-coral);
    font: inherit;
    font-size: 0.9rem;
    font-weight: 600;
    cursor: pointer;
    transition: background-color var(--ah-motion-fast) ease;
  }

  .submit:hover,
  .submit:focus-visible {
    background: var(--ah-rust);
  }

  .prompt {
    margin: 0.35rem 0 0;
    color: var(--ah-muted);
    font-size: 0.8rem;
    text-align: center;
  }

  .fixtures {
    margin: 0;
    color: var(--ah-muted);
    font-size: 0.75rem;
    line-height: 1.5;
    text-align: center;
  }

  a:focus-visible,
  button:focus-visible {
    outline: var(--ah-focus-brand);
    outline-offset: 2px;
  }
</style>
