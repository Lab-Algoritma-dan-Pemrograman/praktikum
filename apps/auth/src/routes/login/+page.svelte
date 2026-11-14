<script lang="ts">
  export let data: {
    loginToken: string;
    returnTo: string | null;
    fixtures: boolean;
    oauthAvailable: boolean;
  };
  export let form: { message?: string; requestId?: string } | null = null;

  const year = new Date().getFullYear();
</script>

<svelte:head
  ><title>Masuk — Algohub</title><meta
    name="referrer"
    content="same-origin"
  /><meta name="theme-color" content="#1b1f2a" /></svelte:head
>

<div class="login-page">
  <main class="split">
    <aside class="identity" aria-label="Tentang Algohub">
      <div class="identity-top">
        <a class="brand brand-on-dark" href="/" aria-label="Algohub beranda">
          <span class="brand-chip"
            ><img
              src="/logo-mark.png"
              alt=""
              width="55"
              height="96"
              decoding="async"
            /></span
          >
          <span class="wordmark"
            ><span class="wordmark-algo">Algo</span
            ><span class="wordmark-hub">Hub</span></span
          >
        </a>

        <div class="identity-copy">
          <h1>Satu ruang untuk praktikum, akademik, dan operasional lab.</h1>
          <p class="identity-lead">
            Masuk sekali, akses SIAKAD, E-Learning, Praktikum, dan Admin Lab
            sesuai peran Anda.
          </p>
        </div>

        <ul class="bullets">
          <li>Satu akun kampus untuk semua aplikasi</li>
          <li>Presensi dan nilai sinkron otomatis</li>
          <li>Sesi berakhir otomatis saat browser ditutup</li>
        </ul>
      </div>

      <p class="identity-foot">Lab Algoritma &amp; Pemrograman</p>
    </aside>

    <section class="form-side" aria-labelledby="login-title">
      <div class="card">
        <p class="eyebrow">Masuk</p>
        <h2 id="login-title">Selamat datang kembali</h2>
        <p class="lead">Gunakan akun kampus Anda untuk melanjutkan.</p>

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
      </div>
    </section>
  </main>

  <footer class="site-footer">
    <div class="footer-inner">
      <a class="brand footer-brand" href="/" aria-label="Algohub beranda">
        <img
          src="/logo-mark.png"
          alt=""
          width="55"
          height="96"
          decoding="async"
        />
        <span class="wordmark"
          ><span class="wordmark-algo">Algo</span
          ><span class="wordmark-hub">Hub</span></span
        >
      </a>
      <p class="footer-copy">
        © {year} Lab Algoritma &amp; Pemrograman
      </p>
      <nav class="footer-links" aria-label="Navigasi footer">
        <a href="/">Portal</a>
        <a href="/">Panduan</a>
        <a href="/">Bantuan</a>
      </nav>
    </div>
  </footer>
</div>

<style>
  @font-face {
    font-family: 'Fredoka';
    font-style: normal;
    font-weight: 500;
    font-display: swap;
    src: url('/fonts/fredoka-500.woff2') format('woff2');
  }

  .login-page {
    display: flex;
    flex-direction: column;
    min-height: 100vh;
    min-height: 100dvh;
    background: var(--ah-cream);
  }

  .split {
    display: flex;
    flex: 1;
    min-height: 0;
  }

  /* ---------- identity panel ---------- */
  .identity {
    display: flex;
    flex: 0 0 clamp(20rem, 40%, 30rem);
    flex-direction: column;
    justify-content: space-between;
    gap: 2.5rem;
    padding: clamp(2rem, 4vw, 3.5rem) clamp(1.75rem, 3vw, 2.75rem);
    color: var(--ah-cream);
    background: var(--ah-ink-strong);
  }

  .identity-top {
    display: grid;
    gap: 2.25rem;
  }

  .brand {
    display: inline-flex;
    width: fit-content;
    min-height: 44px;
    align-items: center;
    gap: 0.6rem;
    text-decoration: none;
  }

  .brand-chip {
    display: grid;
    width: 2.25rem;
    height: 2.5rem;
    place-items: center;
    border-radius: 0.6rem;
    background: var(--ah-cream);
  }

  .brand-chip img {
    height: 1.6rem;
    width: auto;
  }

  .wordmark {
    display: inline-flex;
    align-items: baseline;
    font-family: 'Fredoka', ui-rounded, 'Segoe UI', system-ui, sans-serif;
    font-size: 1.375rem;
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

  .brand-on-dark .wordmark-algo {
    color: var(--ah-cream);
  }

  .brand-on-dark .wordmark-hub {
    color: #e8778f;
  }

  .identity-copy h1 {
    margin: 0;
    max-width: 18ch;
    color: var(--ah-cream);
    font-size: clamp(1.65rem, 2.6vw, 2.15rem);
    font-weight: 500;
    line-height: 1.14;
    letter-spacing: -0.03em;
    text-wrap: balance;
  }

  .identity-lead {
    margin: 1rem 0 0;
    max-width: 34ch;
    color: #b9b4b1;
    font-size: 0.95rem;
    line-height: 1.6;
  }

  .bullets {
    display: grid;
    gap: 0.7rem;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .bullets li {
    position: relative;
    padding-left: 1.15rem;
    color: #ddd8d6;
    font-size: 0.875rem;
    line-height: 1.5;
  }

  .bullets li::before {
    position: absolute;
    top: 0.5em;
    left: 0;
    width: 6px;
    height: 6px;
    border-radius: 999px;
    background: var(--ah-coral);
    content: '';
  }

  .identity-foot {
    margin: 0;
    color: #8f8a88;
    font-size: 0.78rem;
    letter-spacing: 0.02em;
  }

  /* ---------- form side ---------- */
  .form-side {
    display: grid;
    flex: 1;
    min-width: 0;
    place-items: center;
    padding: clamp(1.75rem, 4vw, 3rem) clamp(1.25rem, 4vw, 3.5rem);
  }

  .card {
    width: min(25rem, 100%);
    padding: clamp(1.5rem, 3vw, 2.25rem);
    border: 1px solid var(--ah-line);
    border-radius: var(--ah-radius-md);
    background: #ffffff;
    box-shadow: var(--ah-shadow-float);
  }

  .eyebrow {
    margin: 0 0 0.6rem;
    color: var(--ah-coral);
    font-size: 0.75rem;
    font-weight: 700;
    letter-spacing: 0.14em;
    text-transform: uppercase;
  }

  h2 {
    margin: 0;
    color: var(--ah-ink);
    font-size: clamp(1.5rem, 2.6vw, 1.85rem);
    font-weight: 500;
    line-height: 1.15;
    letter-spacing: -0.025em;
  }

  .lead {
    margin: 0.55rem 0 0;
    color: var(--ah-muted);
    font-size: 0.925rem;
    line-height: 1.55;
  }

  .destination {
    margin: 1rem 0 0;
    padding: 0.55rem 0.75rem;
    border-radius: var(--ah-radius-sm);
    background: var(--ah-surface);
    color: var(--ah-muted);
    font-size: 0.825rem;
    overflow-wrap: anywhere;
  }

  .error {
    margin-top: 1rem;
    padding: 0.85rem 0.9rem;
    border: 1px solid var(--ah-console-danger-line);
    border-radius: var(--ah-radius-sm);
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
    margin-top: 1.35rem;
  }

  .field {
    display: grid;
    gap: 0.4rem;
  }

  label {
    color: var(--ah-ink);
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
    left: 0.85rem;
    display: grid;
    place-items: center;
    color: var(--ah-muted);
    pointer-events: none;
  }

  .icon svg {
    width: 1.05rem;
    height: 1.05rem;
  }

  input {
    width: 100%;
    min-height: 46px;
    padding: 0.7rem 0.9rem 0.7rem 2.55rem;
    border: 1px solid var(--ah-line-strong);
    border-radius: var(--ah-radius-sm);
    color: var(--ah-ink);
    background: #faf6f3;
    font: inherit;
  }

  input::placeholder {
    color: color-mix(in srgb, var(--ah-muted) 78%, transparent);
  }

  input:focus-visible {
    border-color: var(--ah-coral);
    outline: 2px solid var(--ah-coral);
    outline-offset: 1px;
    background: #ffffff;
  }

  button {
    min-height: 46px;
    padding: 0.7rem 1rem;
    border: 0;
    border-radius: 999px;
    font: inherit;
    font-weight: 600;
    cursor: pointer;
    transition:
      transform var(--ah-motion-fast) ease,
      background-color var(--ah-motion-fast) ease;
  }

  .submit {
    margin-top: 0.25rem;
    color: var(--ah-cream);
    background: var(--ah-ink);
  }

  .submit:hover,
  .submit:focus-visible {
    background: var(--ah-ink-strong);
  }

  .submit:active {
    transform: translateY(1px);
  }

  .secondary {
    width: 100%;
    margin-top: 0.75rem;
    border: 1px solid var(--ah-line-strong);
    color: var(--ah-ink);
    background: #ffffff;
  }

  .secondary:hover,
  .secondary:focus-visible {
    background: var(--ah-surface);
  }

  .notes {
    display: grid;
    gap: 0.35rem;
    margin-top: 1.35rem;
    padding-top: 1rem;
    border-top: 1px solid var(--ah-line);
  }

  .notes p {
    margin: 0;
    color: var(--ah-muted);
    font-size: 0.775rem;
    line-height: 1.5;
  }

  /* ---------- footer ---------- */
  .site-footer {
    border-top: 1px solid var(--ah-line);
    background: var(--ah-cream);
  }

  .footer-inner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1.25rem 2rem;
    width: min(75rem, 100%);
    margin: 0 auto;
    padding: 1.25rem clamp(1.25rem, 3vw, 2.75rem);
  }

  .footer-brand {
    gap: 0.5rem;
  }

  .footer-brand img {
    height: 1.55rem;
    width: auto;
  }

  .footer-brand .wordmark {
    font-size: 1.05rem;
  }

  .footer-copy {
    margin: 0;
    color: var(--ah-muted);
    font-size: 0.8rem;
  }

  .footer-links {
    display: flex;
    flex-wrap: wrap;
    gap: 0.25rem 1.5rem;
  }

  .footer-links a {
    display: inline-flex;
    min-height: 44px;
    align-items: center;
    color: var(--ah-ink);
    font-size: 0.85rem;
    font-weight: 500;
    text-decoration: none;
  }

  .footer-links a:hover,
  .footer-links a:focus-visible {
    color: var(--ah-coral);
  }

  a:focus-visible {
    outline: var(--ah-focus-brand);
    outline-offset: 2px;
  }

  /* ---------- responsive ---------- */
  @media (max-width: 60rem) {
    .split {
      flex-direction: column;
    }

    .identity {
      flex: 0 0 auto;
      gap: 1.25rem;
      padding: 1.5rem clamp(1.25rem, 4vw, 2rem);
    }

    .identity-top {
      gap: 1.15rem;
    }

    .identity-copy h1 {
      max-width: 32ch;
      font-size: 1.35rem;
    }

    .identity-lead,
    .bullets,
    .identity-foot {
      display: none;
    }

    .form-side {
      padding: 1.5rem clamp(1.25rem, 4vw, 2rem) 2rem;
      place-items: start center;
    }

    .card {
      width: min(28rem, 100%);
    }

    .footer-inner {
      flex-direction: column;
      align-items: flex-start;
      gap: 0.5rem;
      padding-block: 1rem;
    }

    .footer-links {
      gap: 0 1.25rem;
    }
  }
</style>
