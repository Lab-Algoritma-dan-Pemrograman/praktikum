<script lang="ts">
  import type { AppIdentityData, ApplicationLink } from '@algohub/auth-client';

  export let data: AppIdentityData;

  type AppName = ApplicationLink['name'];

  const services: Array<{
    app: AppName;
    index: string;
    title: string;
    description: string;
    features: string[];
    accent: string;
  }> = [
    {
      app: 'siakad',
      index: '01',
      title: 'SIAKAD & layanan lab',
      description:
        'Jadwal, kelas, presensi QR, inventaris, dan penyewaan alat dalam alur yang terukur.',
      features: ['Jadwal & kelas', 'Presensi QR', 'Penyewaan alat'],
      accent: 'blue'
    },
    {
      app: 'elearning',
      index: '02',
      title: 'E-learning interaktif',
      description:
        'Materi berjenjang, quiz, latihan kode di browser, progres belajar, dan gamifikasi.',
      features: ['Kurikulum', 'C & Python', 'XP & progres'],
      accent: 'yellow'
    },
    {
      app: 'praktikum',
      index: '03',
      title: 'Praktikum terarah',
      description:
        'Sesi berbatas waktu, autosave, submission, serta publikasi nilai melalui review manual.',
      features: ['Sesi & timer', 'Autosave', 'Review manual'],
      accent: 'coral'
    },
    {
      app: 'admin',
      index: '04',
      title: 'Operasional terpusat',
      description:
        'Console asisten dan koordinator dengan menu, permission, dan scope sesuai penugasan.',
      features: ['Scoped access', 'Monitoring', 'Audit'],
      accent: 'green'
    }
  ];

  const steps = [
    {
      index: '01',
      title: 'Masuk satu kali',
      description:
        'Gunakan satu identitas Algohub. Setiap aplikasi tetap menjaga sesi host-nya sendiri.'
    },
    {
      index: '02',
      title: 'Buka ruang yang relevan',
      description:
        'Menu dan data mengikuti role, divisi, kelas, serta scope yang diberikan kepada akunmu.'
    },
    {
      index: '03',
      title: 'Kerjakan dengan konteks utuh',
      description:
        'Belajar, mengikuti praktikum, mengelola layanan lab, dan meninjau hasil tanpa akun terpisah.'
    }
  ];

  const publicLinks = [
    {
      app: 'siakad' as AppName,
      label: 'Jadwal dan layanan akademik',
      description: 'Kelas, jadwal, presensi, inventaris, serta penyewaan.'
    },
    {
      app: 'elearning' as AppName,
      label: 'Materi dan modul belajar',
      description: 'Kurikulum, lesson, quiz, playground, dan progres.'
    },
    {
      app: 'praktikum' as AppName,
      label: 'Sesi dan panduan praktikum',
      description: 'Aktivasi, pengerjaan, submission, dan nilai terpublikasi.'
    }
  ];

  function appHref(name: AppName): string {
    const application = data.applications.find((item) => item.name === name);
    return application ? `${application.origin}/login` : '/login';
  }

  function activeAssignments(): string {
    const assignments =
      data.principal?.assignments.filter((item) => item.active) ?? [];
    return (
      assignments
        .map(
          (item) => `${item.role}${item.division ? ` · ${item.division}` : ''}`
        )
        .join(', ') || 'Belum ada penugasan.'
    );
  }
</script>

<svelte:head>
  <title>Algohub — Belajar, Praktikum, dan Layanan Laboratorium</title>
  <meta
    name="description"
    content="Satu pintu untuk layanan akademik, e-learning, praktikum, dan operasional laboratorium Algohub."
  />
  <meta name="theme-color" content="#f5f0e7" />
</svelte:head>

<div class="landing">
  <a class="skip-link" href="#main-content">Lewati ke konten</a>

  <header class="site-header">
    <div class="nav-shell">
      <a class="brand" href="/" aria-label="Algohub beranda">
        <span class="brand-mark" aria-hidden="true">
          <span></span><span></span><span></span><span></span>
        </span>
        <span>Algohub</span>
      </a>

      <nav class="desktop-nav" aria-label="Navigasi landing page">
        <a href="#services">Layanan</a>
        <a href="#workflow">Cara kerja</a>
        <a href="#information">Informasi</a>
        <a href="#faq">FAQ</a>
      </nav>

      <div class="header-actions">
        {#if data.principal}
          <a class="session-link" href="#workspace">
            <span>Masuk sebagai</span>
            <strong>{data.principal.user.name}</strong>
          </a>
        {:else}
          <a class="text-link" href="/login">Masuk</a>
        {/if}
        <a
          class="button button-small"
          href={data.principal ? '#workspace' : '/login'}
        >
          {data.principal ? 'Buka ruang kerja' : 'Mulai sekarang'}
        </a>
      </div>

      <details class="mobile-menu">
        <summary>Menu</summary>
        <nav aria-label="Navigasi mobile">
          <a href="#services">Layanan</a>
          <a href="#workflow">Cara kerja</a>
          <a href="#information">Informasi</a>
          <a href="#faq">FAQ</a>
          <a href={data.principal ? '#workspace' : '/login'}>
            {data.principal ? 'Buka ruang kerja' : 'Masuk ke Algohub'}
          </a>
        </nav>
      </details>
    </div>
  </header>

  <main id="main-content">
    <section class="hero" aria-labelledby="hero-title">
      <div class="hero-copy">
        <p class="eyebrow"><span></span> Ruang digital laboratorium</p>
        <h1 id="hero-title">
          Belajar, praktik, dan kelola lab dalam satu identitas.
        </h1>
        <p class="hero-lead">
          Algohub menyatukan layanan akademik, pembelajaran interaktif,
          praktikum, dan operasional laboratorium tanpa membuatmu
          berpindah-pindah akun.
        </p>
        <div class="hero-actions">
          <a class="button" href={data.principal ? '#workspace' : '/login'}>
            {data.principal ? 'Lanjutkan aktivitas' : 'Masuk ke Algohub'}
          </a>
          <a class="button button-secondary" href="#services"
            >Jelajahi layanan</a
          >
        </div>
        <ul class="hero-notes" aria-label="Keunggulan Algohub">
          <li><span aria-hidden="true"></span>Satu identitas</li>
          <li><span aria-hidden="true"></span>Akses sesuai peran</li>
          <li><span aria-hidden="true"></span>Responsif di berbagai layar</li>
        </ul>
      </div>

      <div class="hero-visual" aria-hidden="true">
        <div class="visual-orbit orbit-one"></div>
        <div class="visual-orbit orbit-two"></div>
        <div class="lab-window">
          <div class="window-bar">
            <span></span><span></span><span></span>
            <p>algohub / workspace</p>
          </div>
          <div class="window-body">
            <div class="code-panel">
              <p><span>01</span> ruang belajar aktif</p>
              <p><span>02</span> sesi praktikum tersimpan</p>
              <p><span>03</span> layanan lab terhubung</p>
              <div class="code-cursor"></div>
            </div>
            <div class="status-grid">
              <div class="status-card status-card-blue">
                <span>SIAKAD</span>
                <strong>Jadwal & lab</strong>
              </div>
              <div class="status-card status-card-yellow">
                <span>LEARN</span>
                <strong>Materi & XP</strong>
              </div>
              <div class="status-card status-card-coral">
                <span>PRAKTIKUM</span>
                <strong>Sesi & review</strong>
              </div>
            </div>
          </div>
        </div>
        <div class="shape shape-arch"></div>
        <div class="shape shape-circle"></div>
        <div class="shape shape-square"></div>
      </div>
    </section>

    {#if data.principal}
      <section
        id="workspace"
        class="workspace"
        aria-labelledby="workspace-title"
      >
        <div>
          <p class="section-kicker">Sesi aktif</p>
          <h2 id="workspace-title">
            Selamat datang kembali, {data.principal.user.name}.
          </h2>
          <p>{activeAssignments()}</p>
        </div>
        <div class="workspace-links" aria-label="Aplikasi yang tersedia">
          {#each data.applications.filter((app) => !['portal', 'auth'].includes(app.name)) as application}
            <a href={`${application.origin}/login`}>
              <span>{application.label}</span>
              <span aria-hidden="true">↗</span>
            </a>
          {/each}
        </div>
      </section>
    {/if}

    <section
      id="services"
      class="section services"
      aria-labelledby="services-title"
    >
      <div class="section-heading">
        <p class="section-kicker">Satu ekosistem, empat ruang</p>
        <h2 id="services-title">Berpindah layanan tanpa kehilangan konteks.</h2>
        <p>
          Setiap ruang memiliki fungsi yang jelas, tetapi tetap memakai
          identitas, permission, dan data yang konsisten.
        </p>
      </div>

      <div class="service-grid">
        {#each services as service}
          <article
            class:blue={service.accent === 'blue'}
            class:yellow={service.accent === 'yellow'}
            class:coral={service.accent === 'coral'}
            class:green={service.accent === 'green'}
          >
            <div class="service-topline">
              <span>{service.index}</span>
              <span class="service-shape" aria-hidden="true"></span>
            </div>
            <h3>{service.title}</h3>
            <p>{service.description}</p>
            <ul>
              {#each service.features as feature}<li>{feature}</li>{/each}
            </ul>
            <a href={appHref(service.app)}>
              Buka {service.title}<span aria-hidden="true">↗</span>
            </a>
          </article>
        {/each}
      </div>
    </section>

    <section id="workflow" class="workflow" aria-labelledby="workflow-title">
      <div class="workflow-heading">
        <p class="section-kicker">Cara kerja</p>
        <h2 id="workflow-title">Sederhana untuk pengguna, ketat pada akses.</h2>
        <p>
          Satu alur masuk mengantar setiap pengguna ke ruang dan data yang
          sesuai dengan tanggung jawabnya.
        </p>
      </div>
      <ol>
        {#each steps as step}
          <li>
            <span>{step.index}</span>
            <div>
              <h3>{step.title}</h3>
              <p>{step.description}</p>
            </div>
          </li>
        {/each}
      </ol>
    </section>

    <section
      id="information"
      class="section information"
      aria-labelledby="information-title"
    >
      <div class="section-heading split-heading">
        <div>
          <p class="section-kicker">Akses cepat</p>
          <h2 id="information-title">Temukan ruang yang kamu butuhkan.</h2>
        </div>
        <p>
          Tautan berikut mengarah ke aplikasi resmi. Setelah masuk, konten yang
          tampil tetap mengikuti permission dan scope akunmu.
        </p>
      </div>

      <div class="information-list">
        {#each publicLinks as link, index}
          <a href={appHref(link.app)}>
            <span class="information-index">0{index + 1}</span>
            <span>
              <strong>{link.label}</strong>
              <small>{link.description}</small>
            </span>
            <span class="information-arrow" aria-hidden="true">↗</span>
          </a>
        {/each}
      </div>

      <aside class="trust-note" aria-labelledby="trust-title">
        <div class="trust-mark" aria-hidden="true"><span></span></div>
        <div>
          <p class="section-kicker">Batas yang jelas</p>
          <h3 id="trust-title">
            Latihan kode mendukung belajar, bukan menggantikan penilaian.
          </h3>
          <p>
            Hasil run kode di browser digunakan sebagai umpan balik dan progres
            gamifikasi. Nilai Praktikum tetap melalui submission dan review
            manual.
          </p>
        </div>
      </aside>
    </section>

    <section id="faq" class="section faq" aria-labelledby="faq-title">
      <div class="faq-heading">
        <p class="section-kicker">Pertanyaan umum</p>
        <h2 id="faq-title">Sebelum mulai.</h2>
      </div>
      <div class="faq-list">
        <details>
          <summary>Apakah semua aplikasi memakai akun yang sama?</summary>
          <p>
            Ya. Auth Center menghubungkan identitas yang sama ke Portal, SIAKAD,
            E-learning, Praktikum, dan Admin sesuai hak akses akun.
          </p>
        </details>
        <details>
          <summary>Mengapa menu setiap pengguna bisa berbeda?</summary>
          <p>
            Menu mengikuti role, divisi, permission, dan scope. Perbedaan menu
            adalah bagian dari pembatasan akses, bukan error tampilan.
          </p>
        </details>
        <details>
          <summary>Apakah hasil run kode otomatis menjadi nilai?</summary>
          <p>
            Tidak. Run browser pada E-learning mendukung feedback belajar dan
            XP. Penilaian Praktikum tetap dipublikasikan setelah review manual.
          </p>
        </details>
        <details>
          <summary>Di mana layanan peminjaman alat tersedia?</summary>
          <p>
            Inventaris dan penyewaan berada di SIAKAD. Status dan tindakan yang
            tersedia mengikuti permission akun peminjam atau pengelola.
          </p>
        </details>
      </div>
    </section>

    <section class="closing" aria-labelledby="closing-title">
      <div class="closing-shape" aria-hidden="true"></div>
      <p class="section-kicker">Mulai dari sini</p>
      <h2 id="closing-title">Satu pintu untuk seluruh aktivitas Algohub.</h2>
      <p>
        Masuk sekali, lalu lanjutkan ke ruang belajar, praktikum, atau layanan
        laboratorium yang sesuai dengan akunmu.
      </p>
      <a
        class="button button-dark"
        href={data.principal ? '#workspace' : '/login'}
      >
        {data.principal ? 'Buka ruang kerja' : 'Masuk ke Algohub'}
      </a>
    </section>
  </main>

  <footer>
    <div class="footer-brand">
      <a class="brand" href="/" aria-label="Algohub beranda">
        <span class="brand-mark" aria-hidden="true">
          <span></span><span></span><span></span><span></span>
        </span>
        <span>Algohub</span>
      </a>
      <p>Ruang digital untuk pembelajaran dan operasional laboratorium.</p>
    </div>
    <nav aria-label="Navigasi footer">
      <div>
        <strong>Jelajahi</strong>
        <a href="#services">Layanan</a>
        <a href="#workflow">Cara kerja</a>
        <a href="#information">Informasi</a>
      </div>
      <div>
        <strong>Akun</strong>
        <a href="/login">Masuk</a>
        <a href={data.authOrigin}>Auth Center</a>
        <a href="#faq">Bantuan awal</a>
      </div>
    </nav>
    <p class="copyright">
      <span>© 2026 Algohub. Sistem akademik dan laboratorium.</span>
      <a
        href="https://8b.com/templates/burrow/"
        target="_blank"
        rel="noopener noreferrer">Design direction: 8B Burrow</a
      >
    </p>
  </footer>
</div>

<style>
  :global(html) {
    scroll-behavior: smooth;
    scroll-padding-top: 6.5rem;
  }

  :global(body) {
    color: #142724;
    background: #f5f0e7;
  }

  :global(a) {
    color: inherit;
  }

  .landing {
    --ink: var(--ah-ink);
    --muted: var(--ah-muted);
    --cream: var(--ah-cream);
    --surface: var(--ah-surface);
    --paper: var(--ah-paper);
    --coral: var(--ah-coral);
    --blue: var(--ah-blue);
    --yellow: var(--ah-yellow);
    --green: var(--ah-green);
    --line: var(--ah-line);
    min-height: 100vh;
    overflow: clip;
  }

  .skip-link {
    position: fixed;
    z-index: 100;
    top: 0.75rem;
    left: 0.75rem;
    padding: 0.75rem 1rem;
    border-radius: 0.75rem;
    color: var(--cream);
    background: var(--ink);
    transform: translateY(-150%);
  }

  .skip-link:focus {
    transform: translateY(0);
  }

  .site-header {
    position: sticky;
    z-index: 50;
    top: 0;
    padding: 1rem 0;
    border-bottom: 1px solid var(--line);
    background: var(--cream);
  }

  .nav-shell {
    display: flex;
    width: min(74rem, calc(100% - 2rem));
    min-height: 3.5rem;
    align-items: center;
    gap: 1rem;
    margin: 0 auto;
  }

  .brand {
    display: inline-flex;
    align-items: center;
    gap: 0.7rem;
    font-size: 1.05rem;
    font-weight: 800;
    letter-spacing: -0.025em;
    text-decoration: none;
  }

  .brand-mark {
    display: grid;
    width: 2.4rem;
    height: 2.4rem;
    grid-template-columns: repeat(2, 1fr);
    gap: 0.18rem;
    padding: 0.35rem;
    border-radius: 1rem 1rem 0.45rem 0.45rem;
    background: var(--ink);
  }

  .brand-mark span {
    border-radius: 50%;
    background: var(--cream);
  }

  .brand-mark span:nth-child(2) {
    border-radius: 50% 50% 0 50%;
    background: var(--coral);
  }

  .brand-mark span:nth-child(3) {
    border-radius: 50% 0 50% 50%;
    background: var(--blue);
  }

  .desktop-nav {
    display: flex;
    align-items: center;
    gap: 0.2rem;
    margin-left: auto;
    padding: 0.3rem;
    border-radius: 999px;
    background: var(--surface);
  }

  .desktop-nav a,
  .text-link {
    display: inline-flex;
    min-height: 2.7rem;
    align-items: center;
    padding: 0 1rem;
    border-radius: 999px;
    color: var(--muted);
    font-size: 0.9rem;
    font-weight: 700;
    text-decoration: none;
  }

  .desktop-nav a:hover,
  .text-link:hover {
    color: var(--ink);
    background: var(--paper);
  }

  .header-actions {
    display: flex;
    align-items: center;
    gap: 0.35rem;
  }

  .session-link {
    display: grid;
    gap: 0.05rem;
    padding: 0.35rem 0.7rem;
    text-decoration: none;
  }

  .session-link span {
    color: var(--muted);
    font-size: 0.68rem;
  }

  .session-link strong {
    max-width: 10rem;
    overflow: hidden;
    font-size: 0.82rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .button {
    display: inline-flex;
    min-height: 3.25rem;
    align-items: center;
    justify-content: center;
    padding: 0 1.45rem;
    border: 1.5px solid var(--ink);
    border-radius: 1.6rem 1.6rem 0.65rem 0.65rem;
    color: var(--ink);
    background: var(--coral);
    font-weight: 800;
    text-decoration: none;
    transition:
      border-radius 160ms ease,
      transform 160ms ease;
  }

  .button:hover {
    border-radius: 0.65rem 0.65rem 1.6rem 1.6rem;
    transform: translateY(-2px);
  }

  .button-small {
    min-height: 2.8rem;
    padding: 0 1.1rem;
    font-size: 0.86rem;
  }

  .button-secondary {
    background: transparent;
  }

  .button-dark {
    color: var(--cream);
    background: var(--ink);
  }

  .mobile-menu {
    display: none;
    margin-left: auto;
  }

  .mobile-menu summary {
    min-height: 2.75rem;
    padding: 0.8rem 1rem;
    border-radius: 999px;
    background: var(--surface);
    cursor: pointer;
    font-weight: 800;
    list-style: none;
  }

  .mobile-menu summary::-webkit-details-marker {
    display: none;
  }

  .mobile-menu nav {
    position: absolute;
    top: 4.7rem;
    right: 1rem;
    left: 1rem;
    display: grid;
    padding: 0.75rem;
    border: 1.5px solid var(--ink);
    border-radius: 1rem;
    background: var(--paper);
  }

  .mobile-menu nav a {
    min-height: 3rem;
    padding: 0.85rem;
    border-bottom: 1px solid var(--line);
    font-weight: 750;
    text-decoration: none;
  }

  .mobile-menu nav a:last-child {
    border: 0;
  }

  .hero {
    display: grid;
    width: min(74rem, calc(100% - 2rem));
    min-height: calc(100svh - 5.5rem);
    grid-template-columns: minmax(0, 1.08fr) minmax(24rem, 0.92fr);
    align-items: center;
    gap: clamp(2.5rem, 6vw, 6rem);
    margin: 0 auto;
    padding: clamp(4rem, 8vw, 7rem) 0;
  }

  .eyebrow,
  .section-kicker {
    margin: 0 0 1rem;
    color: #365e56;
    font-size: 0.78rem;
    font-weight: 850;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .eyebrow {
    display: flex;
    align-items: center;
    gap: 0.55rem;
  }

  .eyebrow span {
    width: 0.75rem;
    height: 0.75rem;
    border-radius: 50% 50% 0 50%;
    background: var(--green);
    transform: rotate(45deg);
  }

  h1,
  h2,
  h3,
  p {
    overflow-wrap: anywhere;
  }

  h1,
  h2,
  h3 {
    text-wrap: balance;
  }

  h1 {
    max-width: 12ch;
    margin: 0;
    font-size: clamp(3.25rem, 7.6vw, 6.8rem);
    font-weight: 760;
    line-height: 0.94;
    letter-spacing: -0.065em;
  }

  .hero-lead {
    max-width: 38rem;
    margin: 1.75rem 0 0;
    color: var(--muted);
    font-size: clamp(1.05rem, 1.5vw, 1.25rem);
    line-height: 1.65;
  }

  .hero-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.75rem;
    margin-top: 2rem;
  }

  .hero-notes {
    display: flex;
    flex-wrap: wrap;
    gap: 0.75rem 1.25rem;
    margin: 2rem 0 0;
    padding: 0;
    color: var(--muted);
    font-size: 0.86rem;
    list-style: none;
  }

  .hero-notes li {
    display: flex;
    align-items: center;
    gap: 0.45rem;
  }

  .hero-notes li span {
    width: 0.48rem;
    height: 0.48rem;
    border-radius: 50%;
    background: var(--green);
  }

  .hero-visual {
    position: relative;
    min-height: 35rem;
  }

  .lab-window {
    position: absolute;
    z-index: 2;
    top: 7%;
    right: 2%;
    left: 5%;
    overflow: hidden;
    border: 1.5px solid var(--ink);
    border-radius: 7rem 7rem 1.5rem 1.5rem;
    background: var(--ink);
    transform: rotate(2.5deg);
  }

  .window-bar {
    display: flex;
    min-height: 3.4rem;
    align-items: center;
    gap: 0.4rem;
    padding: 0 1.2rem;
    border-bottom: 1px solid #ffffff2c;
    color: #b7c8c3;
  }

  .window-bar > span {
    width: 0.65rem;
    height: 0.65rem;
    border-radius: 50%;
    background: var(--coral);
  }

  .window-bar > span:nth-child(2) {
    background: var(--yellow);
  }

  .window-bar > span:nth-child(3) {
    background: var(--green);
  }

  .window-bar p {
    margin: 0 0 0 auto;
    font-size: 0.72rem;
    font-weight: 700;
  }

  .window-body {
    padding: clamp(1.1rem, 3vw, 2rem);
  }

  .code-panel {
    display: grid;
    gap: 1rem;
    padding: 1.5rem;
    border: 1px solid #ffffff26;
    border-radius: 1rem;
    color: #dce7e3;
    background: #1e3531;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.78rem;
  }

  .code-panel p {
    margin: 0;
  }

  .code-panel span {
    margin-right: 0.8rem;
    color: var(--yellow);
  }

  .code-cursor {
    width: 1.8rem;
    height: 0.45rem;
    background: var(--coral);
  }

  .status-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 0.65rem;
    margin-top: 0.65rem;
  }

  .status-card {
    display: grid;
    min-height: 7.5rem;
    align-content: space-between;
    padding: 1rem;
    border-radius: 1rem;
    color: var(--ink);
  }

  .status-card span {
    font-size: 0.62rem;
    font-weight: 900;
    letter-spacing: 0.09em;
  }

  .status-card strong {
    font-size: 0.85rem;
    line-height: 1.25;
  }

  .status-card-blue {
    background: var(--blue);
  }

  .status-card-yellow {
    background: var(--yellow);
  }

  .status-card-coral {
    background: var(--coral);
  }

  .visual-orbit,
  .shape {
    position: absolute;
    border: 1.5px solid var(--ink);
  }

  .visual-orbit {
    border-radius: 50%;
  }

  .orbit-one {
    top: 1%;
    right: -10%;
    width: 14rem;
    height: 14rem;
  }

  .orbit-two {
    bottom: 4%;
    left: -7%;
    width: 9rem;
    height: 9rem;
  }

  .shape-arch {
    z-index: 3;
    right: -2%;
    bottom: 3%;
    width: 6rem;
    height: 8rem;
    border-radius: 5rem 5rem 0.8rem 0.8rem;
    background: var(--coral);
    transform: rotate(7deg);
  }

  .shape-circle {
    z-index: 1;
    top: 0;
    left: 0;
    width: 5.5rem;
    height: 5.5rem;
    border-radius: 50%;
    background: var(--yellow);
  }

  .shape-square {
    z-index: 3;
    bottom: 9%;
    left: 0;
    width: 4rem;
    height: 4rem;
    border-radius: 1rem;
    background: var(--blue);
    transform: rotate(-11deg);
  }

  .workspace {
    display: grid;
    width: min(74rem, calc(100% - 2rem));
    grid-template-columns: minmax(0, 1fr) minmax(18rem, 0.85fr);
    align-items: center;
    gap: 2rem;
    margin: 0 auto 2rem;
    padding: clamp(1.5rem, 4vw, 2.5rem);
    border: 1.5px solid var(--ink);
    border-radius: 2rem;
    background: var(--paper);
  }

  .workspace h2 {
    margin: 0;
    font-size: clamp(1.6rem, 3vw, 2.4rem);
    letter-spacing: -0.04em;
  }

  .workspace p:last-child {
    margin-bottom: 0;
    color: var(--muted);
  }

  .workspace-links {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 0.5rem;
  }

  .workspace-links a {
    display: flex;
    min-height: 3.5rem;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.85rem 1rem;
    border-radius: 1rem;
    background: var(--surface);
    font-weight: 800;
    text-decoration: none;
  }

  .section {
    width: min(74rem, calc(100% - 2rem));
    margin: 0 auto;
    padding: clamp(5rem, 10vw, 9rem) 0;
  }

  .section-heading {
    max-width: 48rem;
    margin: 0 auto clamp(2.5rem, 5vw, 4.5rem);
    text-align: center;
  }

  .section-heading h2,
  .workflow h2,
  .faq h2,
  .closing h2 {
    margin: 0;
    font-size: clamp(2.5rem, 6vw, 5rem);
    line-height: 1;
    letter-spacing: -0.055em;
  }

  .section-heading > p:last-child,
  .workflow-heading > p:last-child,
  .closing > p {
    color: var(--muted);
    font-size: 1.05rem;
    line-height: 1.65;
  }

  .service-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 1rem;
  }

  .service-grid article {
    display: flex;
    min-height: 25rem;
    flex-direction: column;
    padding: clamp(1.5rem, 4vw, 2.5rem);
    border: 1.5px solid var(--ink);
    border-radius: 2rem;
  }

  .service-grid article.blue {
    background: var(--blue);
  }

  .service-grid article.yellow {
    background: var(--yellow);
  }

  .service-grid article.coral {
    background: var(--coral);
  }

  .service-grid article.green {
    background: var(--green);
  }

  .service-topline {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.8rem;
    font-weight: 800;
  }

  .service-shape {
    width: 2.7rem;
    height: 2.7rem;
    border: 1.5px solid var(--ink);
    border-radius: 50% 50% 0.4rem 50%;
    background: var(--cream);
  }

  .service-grid article:nth-child(2) .service-shape {
    border-radius: 50%;
  }

  .service-grid article:nth-child(3) .service-shape {
    border-radius: 1.5rem 1.5rem 0.3rem 0.3rem;
  }

  .service-grid article:nth-child(4) .service-shape {
    border-radius: 0.5rem;
    transform: rotate(12deg);
  }

  .service-grid h3 {
    max-width: 12ch;
    margin: auto 0 0;
    font-size: clamp(2rem, 4vw, 3.2rem);
    line-height: 1;
    letter-spacing: -0.05em;
  }

  .service-grid p {
    max-width: 34rem;
    margin: 1rem 0;
    line-height: 1.6;
  }

  .service-grid ul {
    display: flex;
    flex-wrap: wrap;
    gap: 0.45rem;
    margin: 0 0 1.5rem;
    padding: 0;
    list-style: none;
  }

  .service-grid li {
    padding: 0.45rem 0.75rem;
    border: 1px solid #14272470;
    border-radius: 999px;
    font-size: 0.76rem;
    font-weight: 750;
  }

  .service-grid article > a {
    display: flex;
    min-height: 3rem;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding-top: 1rem;
    border-top: 1.5px solid #14272470;
    font-weight: 850;
    text-decoration: none;
  }

  .workflow {
    display: grid;
    width: min(82rem, calc(100% - 2rem));
    grid-template-columns: minmax(0, 0.85fr) minmax(28rem, 1.15fr);
    gap: clamp(3rem, 7vw, 7rem);
    margin: clamp(3rem, 7vw, 6rem) auto;
    padding: clamp(3rem, 7vw, 6rem);
    border-radius: 3rem;
    color: var(--cream);
    background: var(--ink);
  }

  .workflow .section-kicker {
    color: #9ac8ba;
  }

  .workflow-heading > p:last-child {
    color: #b9c7c3;
  }

  .workflow ol {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .workflow li {
    display: grid;
    grid-template-columns: 3.5rem 1fr;
    gap: 1.2rem;
    padding: 1.5rem 0;
    border-top: 1px solid #ffffff2d;
  }

  .workflow li:last-child {
    border-bottom: 1px solid #ffffff2d;
  }

  .workflow li > span {
    display: grid;
    width: 2.5rem;
    height: 2.5rem;
    place-items: center;
    border-radius: 50%;
    color: var(--ink);
    background: var(--yellow);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.75rem;
    font-weight: 900;
  }

  .workflow h3 {
    margin: 0;
    font-size: 1.35rem;
  }

  .workflow li p {
    margin: 0.6rem 0 0;
    color: #b9c7c3;
    line-height: 1.6;
  }

  .split-heading {
    display: grid;
    max-width: none;
    grid-template-columns: minmax(0, 1fr) minmax(18rem, 0.6fr);
    align-items: end;
    gap: 3rem;
    text-align: left;
  }

  .split-heading > p {
    margin: 0;
  }

  .information-list {
    border-top: 1.5px solid var(--ink);
  }

  .information-list > a {
    display: grid;
    min-height: 7rem;
    grid-template-columns: 4rem 1fr auto;
    align-items: center;
    gap: 1.25rem;
    border-bottom: 1.5px solid var(--ink);
    text-decoration: none;
  }

  .information-list > a:hover .information-arrow {
    color: var(--cream);
    background: var(--ink);
    transform: rotate(8deg);
  }

  .information-index {
    color: var(--muted);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.8rem;
  }

  .information-list strong,
  .information-list small {
    display: block;
  }

  .information-list strong {
    font-size: clamp(1.25rem, 3vw, 2rem);
    letter-spacing: -0.035em;
  }

  .information-list small {
    margin-top: 0.35rem;
    color: var(--muted);
    font-size: 0.9rem;
  }

  .information-arrow {
    display: grid;
    width: 3.2rem;
    height: 3.2rem;
    place-items: center;
    border: 1.5px solid var(--ink);
    border-radius: 50%;
    font-size: 1.3rem;
    transition:
      color 160ms ease,
      background 160ms ease,
      transform 160ms ease;
  }

  .trust-note {
    display: grid;
    grid-template-columns: 8rem minmax(0, 1fr);
    gap: clamp(1.5rem, 5vw, 4rem);
    margin-top: clamp(3rem, 7vw, 6rem);
    padding: clamp(1.5rem, 5vw, 3.5rem);
    border-radius: 2rem;
    background: var(--surface);
  }

  .trust-mark {
    display: grid;
    width: 7rem;
    height: 7rem;
    place-items: center;
    border-radius: 50% 50% 0.75rem 50%;
    background: var(--coral);
  }

  .trust-mark span {
    width: 2.25rem;
    height: 3.25rem;
    border: 0.45rem solid var(--ink);
    border-top: 0;
    border-radius: 0 0 2rem 2rem;
    transform: rotate(-45deg);
  }

  .trust-note h3 {
    max-width: 24ch;
    margin: 0;
    font-size: clamp(1.7rem, 4vw, 3rem);
    line-height: 1.05;
    letter-spacing: -0.045em;
  }

  .trust-note p:last-child {
    max-width: 48rem;
    color: var(--muted);
    line-height: 1.65;
  }

  .faq {
    display: grid;
    grid-template-columns: minmax(15rem, 0.65fr) minmax(0, 1.35fr);
    gap: clamp(3rem, 8vw, 8rem);
  }

  .faq-heading {
    align-self: start;
  }

  .faq-list {
    border-top: 1.5px solid var(--ink);
  }

  .faq details {
    border-bottom: 1.5px solid var(--ink);
  }

  .faq summary {
    position: relative;
    padding: 1.5rem 3rem 1.5rem 0;
    cursor: pointer;
    font-size: clamp(1.05rem, 2vw, 1.3rem);
    font-weight: 800;
    list-style: none;
  }

  .faq summary::-webkit-details-marker {
    display: none;
  }

  .faq summary::after {
    position: absolute;
    top: 50%;
    right: 0.4rem;
    content: '+';
    color: var(--coral);
    font-size: 2rem;
    font-weight: 500;
    transform: translateY(-50%);
  }

  .faq details[open] summary::after {
    content: '−';
  }

  .faq details p {
    max-width: 44rem;
    margin: 0;
    padding: 0 3rem 1.5rem 0;
    color: var(--muted);
    line-height: 1.65;
  }

  .closing {
    position: relative;
    width: min(74rem, calc(100% - 2rem));
    overflow: hidden;
    margin: clamp(4rem, 8vw, 8rem) auto 1rem;
    padding: clamp(3rem, 9vw, 7rem);
    border: 1.5px solid var(--ink);
    border-radius: 8rem 8rem 2rem 2rem;
    background: var(--coral);
    text-align: center;
  }

  .closing > *:not(.closing-shape) {
    position: relative;
    z-index: 1;
  }

  .closing h2 {
    max-width: 15ch;
    margin-right: auto;
    margin-left: auto;
  }

  .closing > p {
    max-width: 42rem;
    margin: 1.4rem auto 2rem;
    color: #3f453f;
  }

  .closing-shape {
    position: absolute;
    top: -5rem;
    left: -4rem;
    width: 14rem;
    height: 14rem;
    border: 1.5px solid var(--ink);
    border-radius: 50%;
    background: var(--yellow);
  }

  footer {
    display: grid;
    width: min(74rem, calc(100% - 2rem));
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 3rem;
    margin: 0 auto;
    padding: clamp(2rem, 5vw, 4rem);
    border-radius: 2rem 2rem 0 0;
    color: var(--cream);
    background: var(--ink);
  }

  .footer-brand p {
    max-width: 24rem;
    color: #b9c7c3;
    line-height: 1.6;
  }

  footer .brand-mark {
    background: var(--cream);
  }

  footer .brand-mark span {
    background: var(--ink);
  }

  footer .brand-mark span:nth-child(2) {
    background: var(--coral);
  }

  footer .brand-mark span:nth-child(3) {
    background: var(--blue);
  }

  footer nav {
    display: grid;
    grid-template-columns: repeat(2, minmax(8rem, 1fr));
    gap: 2.5rem;
  }

  footer nav div {
    display: grid;
    align-content: start;
    gap: 0.75rem;
  }

  footer nav strong {
    color: #9ac8ba;
    font-size: 0.75rem;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  footer nav a {
    color: var(--cream);
    font-size: 0.9rem;
    text-decoration: none;
  }

  footer nav a:hover {
    text-decoration: underline;
    text-underline-offset: 0.3rem;
  }

  .copyright {
    display: flex;
    grid-column: 1 / -1;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 1rem;
    margin: 1rem 0 0;
    padding-top: 1.25rem;
    border-top: 1px solid #ffffff2d;
    color: #91a39d;
    font-size: 0.75rem;
  }

  .copyright a {
    color: inherit;
  }

  @media (max-width: 960px) {
    .desktop-nav,
    .header-actions {
      display: none;
    }

    .mobile-menu {
      display: block;
    }

    .hero {
      min-height: auto;
      grid-template-columns: 1fr;
      padding-top: 4rem;
    }

    h1 {
      max-width: 11ch;
    }

    .hero-visual {
      width: min(38rem, 100%);
      min-height: 32rem;
      margin: 0 auto;
    }

    .workflow {
      grid-template-columns: 1fr;
    }
  }

  @media (max-width: 720px) {
    :global(html) {
      scroll-padding-top: 5rem;
    }

    .site-header {
      padding: 0.7rem 0;
    }

    .nav-shell,
    .hero,
    .workspace,
    .section,
    .workflow,
    .closing,
    footer {
      width: min(100% - 1.25rem, 74rem);
    }

    .hero {
      gap: 1.5rem;
      padding: 3.25rem 0 4rem;
    }

    h1 {
      font-size: clamp(3rem, 15vw, 5rem);
    }

    .hero-actions,
    .hero-actions .button {
      width: 100%;
    }

    .hero-notes {
      display: grid;
    }

    .hero-visual {
      min-height: 25rem;
    }

    .lab-window {
      right: 1rem;
      left: 1rem;
      border-radius: 4rem 4rem 1.2rem 1.2rem;
      transform: none;
    }

    .window-body {
      padding: 0.85rem;
    }

    .code-panel {
      gap: 0.75rem;
      padding: 1rem;
      font-size: 0.68rem;
    }

    .status-grid {
      grid-template-columns: 1fr;
    }

    .status-card {
      min-height: 4.5rem;
    }

    .shape-arch {
      right: 0;
      bottom: 0;
      width: 4.5rem;
      height: 6rem;
    }

    .shape-circle {
      width: 4rem;
      height: 4rem;
    }

    .shape-square {
      display: none;
    }

    .workspace,
    .split-heading,
    .faq,
    footer {
      grid-template-columns: 1fr;
    }

    .workspace-links,
    .service-grid {
      grid-template-columns: 1fr;
    }

    .section {
      padding: 5rem 0;
    }

    .section-heading {
      text-align: left;
    }

    .service-grid article {
      min-height: 23rem;
    }

    .workflow {
      gap: 2.5rem;
      padding: 2rem 1.25rem;
      border-radius: 2rem;
    }

    .workflow li {
      grid-template-columns: 2.8rem 1fr;
      gap: 0.8rem;
    }

    .split-heading {
      gap: 1rem;
    }

    .information-list > a {
      min-height: 8rem;
      grid-template-columns: 2rem 1fr auto;
      gap: 0.75rem;
    }

    .information-list small {
      max-width: 16rem;
    }

    .information-arrow {
      width: 2.6rem;
      height: 2.6rem;
    }

    .trust-note {
      grid-template-columns: 1fr;
    }

    .trust-mark {
      width: 5rem;
      height: 5rem;
    }

    .closing {
      padding: 5rem 1.25rem 3rem;
      border-radius: 4rem 4rem 1.5rem 1.5rem;
    }

    .closing-shape {
      top: -6rem;
      left: -6rem;
    }

    footer {
      gap: 2rem;
      padding: 2rem 1.25rem;
    }

    footer nav {
      gap: 1rem;
    }
  }

  @media (max-width: 390px) {
    .information-list > a {
      grid-template-columns: 1fr auto;
    }

    .information-index {
      display: none;
    }

    footer nav {
      grid-template-columns: 1fr;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    :global(html) {
      scroll-behavior: auto;
    }

    .button,
    .information-arrow {
      transition: none;
    }
  }
</style>
