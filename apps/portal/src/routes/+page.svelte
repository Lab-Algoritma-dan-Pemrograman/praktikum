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
    accent: 'blue' | 'yellow' | 'coral' | 'green';
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
  <meta name="theme-color" content="#fdfbf7" />
</svelte:head>

<div class="landing">
  <a class="skip-link" href="#main-content">Lewati ke konten</a>

  <header class="site-header">
    <div class="nav-shell">
      <a class="brand" href="/" aria-label="Algohub beranda">
        <img
          class="brand-mark"
          src="/logo-mark.png"
          alt=""
          width="55"
          height="96"
        />
        <span class="brand-word">Algohub</span>
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
            <span>{data.principal.user.name}</span>
            <small>Lihat sesi</small>
          </a>
        {:else}
          <a class="text-link" href="/login">Masuk</a>
        {/if}
        <a
          class="button button-small"
          href={data.principal ? '#workspace' : '/login'}
        >
          <span class="button-label"
            >{data.principal ? 'Buka ruang kerja' : 'Mulai sekarang'}</span
          >
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
        <p class="eyebrow">
          <span class="eyebrow-mark" aria-hidden="true"></span>Ruang digital
          laboratorium
        </p>
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
            <span class="button-label"
              >{data.principal
                ? 'Lanjutkan aktivitas'
                : 'Masuk ke Algohub'}</span
            >
          </a>
          <a class="button button-secondary" href="#services">
            <span class="button-label">Jelajahi layanan</span>
          </a>
        </div>
        <ul class="hero-notes" aria-label="Keunggulan Algohub">
          <li>
            <span class="note-dot note-dot-green" aria-hidden="true"></span>Satu
            identitas
          </li>
          <li>
            <span class="note-dot note-dot-blue" aria-hidden="true"></span>Akses
            sesuai peran
          </li>
          <li>
            <span class="note-dot note-dot-yellow" aria-hidden="true"
            ></span>Responsif di berbagai layar
          </li>
        </ul>
      </div>

      <div class="hero-visual" aria-hidden="true">
        <div class="visual-frame">
          <div class="visual-bar">
            <span></span><span></span><span></span>
            <p>algohub / workspace</p>
          </div>
          <div class="visual-body">
            <ul class="visual-list">
              <li><span>01</span>ruang belajar aktif</li>
              <li><span>02</span>sesi praktikum tersimpan</li>
              <li><span>03</span>layanan lab terhubung</li>
            </ul>
            <div class="visual-cards">
              <div class="visual-card card-blue">
                <span>SIAKAD</span>
                <strong>Jadwal &amp; lab</strong>
              </div>
              <div class="visual-card card-yellow">
                <span>E-learning</span>
                <strong>Materi &amp; XP</strong>
              </div>
              <div class="visual-card card-coral">
                <span>Praktikum</span>
                <strong>Sesi &amp; review</strong>
              </div>
            </div>
          </div>
        </div>
        <span class="float-shape float-arch"></span>
        <span class="float-shape float-circle"></span>
        <span class="float-shape float-plus"></span>
        <span class="float-shape float-diamond"></span>
      </div>
    </section>

    {#if data.principal}
      <section
        id="workspace"
        class="workspace"
        aria-labelledby="workspace-title"
      >
        <div>
          <p class="kicker">Sesi aktif</p>
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
      <div class="section-heading" data-reveal>
        <p class="kicker">Satu ekosistem, empat ruang</p>
        <h2 id="services-title">Berpindah layanan tanpa kehilangan konteks.</h2>
        <p>
          Setiap ruang memiliki fungsi yang jelas, tetapi tetap memakai
          identitas, permission, dan data yang konsisten.
        </p>
      </div>

      <div class="service-grid">
        {#each services as service}
          <article
            class="service-panel"
            class:blue={service.accent === 'blue'}
            class:yellow={service.accent === 'yellow'}
            class:coral={service.accent === 'coral'}
            class:green={service.accent === 'green'}
            data-reveal
          >
            <div class="service-top">
              <span class="badge">{service.index}</span>
              <span class="service-shape" aria-hidden="true"></span>
            </div>
            <h3>{service.title}</h3>
            <p>{service.description}</p>
            <ul>
              {#each service.features as feature}<li>{feature}</li>{/each}
            </ul>
            <a class="service-link" href={appHref(service.app)}>
              <span>Buka {service.title}</span>
              <span aria-hidden="true">↗</span>
            </a>
          </article>
        {/each}
      </div>
    </section>

    <section
      id="workflow"
      class="workflow-band"
      aria-labelledby="workflow-title"
      data-reveal
    >
      <div class="workflow-copy">
        <p class="kicker">Cara kerja</p>
        <h2 id="workflow-title">Sederhana untuk pengguna, ketat pada akses.</h2>
        <p>
          Satu alur masuk mengantar setiap pengguna ke ruang dan data yang
          sesuai dengan tanggung jawabnya.
        </p>
      </div>
      <ol class="workflow-steps">
        {#each steps as step}
          <li>
            <span class="step-index">{step.index}</span>
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
      <div class="section-heading split-heading" data-reveal>
        <div>
          <p class="kicker">Akses cepat</p>
          <h2 id="information-title">Temukan ruang yang kamu butuhkan.</h2>
        </div>
        <p>
          Tautan berikut mengarah ke aplikasi resmi. Setelah masuk, konten yang
          tampil tetap mengikuti permission dan scope akunmu.
        </p>
      </div>

      <div class="information-list" data-reveal>
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

      <aside class="trust-note" aria-labelledby="trust-title" data-reveal>
        <div class="trust-mark" aria-hidden="true"><span></span></div>
        <div>
          <p class="kicker">Batas yang jelas</p>
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
      <div class="faq-heading" data-reveal>
        <p class="kicker">Pertanyaan umum</p>
        <h2 id="faq-title">Sebelum mulai.</h2>
      </div>
      <div class="faq-list" data-reveal>
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

    <section class="closing" aria-labelledby="closing-title" data-reveal>
      <div class="closing-shape" aria-hidden="true"></div>
      <p class="kicker">Mulai dari sini</p>
      <h2 id="closing-title">Satu pintu untuk seluruh aktivitas Algohub.</h2>
      <p>
        Masuk sekali, lalu lanjutkan ke ruang belajar, praktikum, atau layanan
        laboratorium yang sesuai dengan akunmu.
      </p>
      <a
        class="button button-dark"
        href={data.principal ? '#workspace' : '/login'}
      >
        <span class="button-label"
          >{data.principal ? 'Buka ruang kerja' : 'Masuk ke Algohub'}</span
        >
      </a>
    </section>
  </main>

  <footer data-reveal>
    <div class="footer-brand">
      <a class="brand" href="/" aria-label="Algohub beranda">
        <img
          class="brand-mark"
          src="/logo-mark.png"
          alt=""
          width="55"
          height="96"
        />
        <span class="brand-word">Algohub</span>
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
    scroll-padding-top: 7rem;
  }

  :global(a) {
    color: inherit;
  }

  .landing {
    --wrap: min(var(--ah-content-max), calc(100% - 2.75rem));
    min-height: 100vh;
    background: var(--ah-cream);
    color: var(--ah-ink);
    font-size: 1.0625rem;
    line-height: 1.55;
    letter-spacing: 0;
  }

  .skip-link {
    position: fixed;
    z-index: 100;
    top: 0.75rem;
    left: 0.75rem;
    padding: 0.75rem 1rem;
    border-radius: 0.875rem;
    color: var(--ah-cream);
    background: var(--ah-ink);
    transform: translateY(-160%);
  }

  .skip-link:focus-visible {
    transform: none;
  }

  .site-header {
    position: fixed;
    inset: 0 0 auto;
    z-index: 40;
    padding: 0.85rem 0;
  }

  .nav-shell {
    position: relative;
    display: flex;
    width: var(--wrap);
    min-height: 3.5rem;
    align-items: center;
    gap: 0.5rem;
    margin: 0 auto;
    padding: 0.4rem;
    border-radius: 999px;
    background: var(--ah-surface);
  }

  .brand {
    display: inline-flex;
    align-items: center;
    gap: 0.65rem;
    padding: 0.15rem 0.9rem 0.15rem 0.15rem;
    border-radius: 999px;
    font-size: 1.4rem;
    font-weight: 500;
    letter-spacing: -0.04em;
    text-decoration: none;
    white-space: nowrap;
  }

  .brand-word {
    padding-top: 0.15rem;
  }

  .brand-mark {
    width: auto;
    height: 2.6rem;
    object-fit: contain;
  }

  .desktop-nav {
    display: flex;
    align-items: center;
    gap: 0.1rem;
    margin-left: auto;
  }

  .desktop-nav a,
  .text-link {
    display: inline-flex;
    min-height: 2.75rem;
    align-items: center;
    padding: 0 0.95rem;
    border-radius: 999px;
    color: var(--ah-muted);
    font-size: 0.95rem;
    font-weight: 500;
    text-decoration: none;
    white-space: nowrap;
    transition:
      color 0.3s ease,
      background-color 0.3s ease;
  }

  .desktop-nav a:hover,
  .desktop-nav a:focus-visible,
  .text-link:hover,
  .text-link:focus-visible {
    color: var(--ah-ink);
    background: var(--ah-cream);
  }

  .header-actions {
    display: flex;
    align-items: center;
    gap: 0.15rem;
  }

  .session-link {
    display: grid;
    gap: 0.05rem;
    padding: 0.3rem 0.75rem;
    border-radius: 999px;
    text-decoration: none;
  }

  .session-link span {
    max-width: 10rem;
    overflow: hidden;
    font-size: 0.85rem;
    font-weight: 500;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .session-link small {
    color: var(--ah-muted);
    font-size: 0.75rem;
  }

  .button {
    position: relative;
    isolation: isolate;
    overflow: hidden;
    display: inline-flex;
    min-height: 3.25rem;
    align-items: center;
    justify-content: center;
    padding: 0 1.625rem;
    border: 0;
    border-radius: 1.625rem 1.625rem 0.625rem 0.625rem;
    color: var(--ah-cream);
    background: var(--ah-coral);
    font-size: 1rem;
    font-weight: 500;
    line-height: 1;
    letter-spacing: -0.005em;
    text-decoration: none;
    cursor: pointer;
    transition: color 0.3s ease;
  }

  .button::before {
    content: '';
    position: absolute;
    inset: -2px;
    z-index: -1;
    border-radius: 50% 50% 0 0 / 70% 70% 0 0;
    background: var(--ah-ink);
    transform: translateY(102%);
    transition:
      transform 0.5s cubic-bezier(0.3, 0.7, 0.2, 1),
      border-radius 0.5s cubic-bezier(0.3, 0.7, 0.2, 1);
  }

  .button-label {
    position: relative;
  }

  .button-small {
    min-height: 2.75rem;
    padding: 0 1.15rem;
    font-size: 0.9rem;
  }

  .button-secondary {
    border-radius: 0.625rem 0.625rem 1.625rem 1.625rem;
    color: var(--ah-ink);
    background: var(--ah-cream);
  }

  .button-secondary:focus-visible {
    color: var(--ah-ink);
  }

  .button-dark {
    color: var(--ah-cream);
    background: var(--ah-ink);
  }

  .button-dark::before {
    background: var(--ah-cream);
  }

  @media (hover: hover) {
    .button:hover::before,
    .button-secondary:hover::before,
    .button-dark:hover::before {
      transform: translateY(0);
      border-radius: 0;
    }

    .button:hover,
    .button-secondary:hover {
      color: var(--ah-cream);
    }

    .button-dark:hover {
      color: var(--ah-ink);
    }
  }

  .button:focus-visible::before {
    transform: translateY(0);
    border-radius: 0;
  }

  .button:focus-visible {
    color: var(--ah-cream);
  }

  .button-dark:focus-visible {
    color: var(--ah-ink);
  }

  .mobile-menu {
    display: none;
    margin-left: auto;
  }

  .mobile-menu summary {
    display: inline-flex;
    min-height: 2.75rem;
    align-items: center;
    padding: 0 1.15rem;
    border-radius: 999px;
    background: var(--ah-cream);
    cursor: pointer;
    font-weight: 500;
    list-style: none;
  }

  .mobile-menu summary::-webkit-details-marker {
    display: none;
  }

  .mobile-menu[open] summary {
    background: var(--ah-ink);
    color: var(--ah-cream);
  }

  .mobile-menu nav {
    position: absolute;
    top: calc(100% + 0.5rem);
    right: 0;
    left: 0;
    display: grid;
    gap: 0.15rem;
    padding: 0.75rem;
    border-radius: 2rem;
    background: var(--ah-coral);
    color: var(--ah-cream);
  }

  .mobile-menu nav a {
    display: flex;
    min-height: 3.25rem;
    align-items: center;
    padding: 0.5rem 0.75rem;
    border-bottom: 1.5px solid
      color-mix(in srgb, var(--ah-cream) 24%, transparent);
    font-size: clamp(1.35rem, 6vw, 1.9rem);
    font-weight: 500;
    letter-spacing: -0.03em;
    text-decoration: none;
  }

  .mobile-menu nav a:last-child {
    border-bottom: 0;
  }

  .hero {
    display: grid;
    width: var(--wrap);
    grid-template-columns: minmax(0, 1.04fr) minmax(0, 0.96fr);
    align-items: center;
    gap: clamp(2.5rem, 5vw, 5rem);
    margin: 0 auto;
    padding: clamp(6.5rem, 9vw, 8.5rem) 0 clamp(3rem, 5vw, 5rem);
  }

  .eyebrow,
  .kicker {
    margin: 0 0 1.1rem;
    color: var(--ah-muted);
    font-size: 0.95rem;
    font-weight: 500;
    line-height: 1.3;
  }

  .eyebrow {
    display: flex;
    align-items: center;
    gap: 0.65rem;
  }

  .eyebrow-mark {
    width: 0.85rem;
    height: 0.85rem;
    border-radius: 50%;
    background: var(--ah-khaki);
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
    font-weight: 500;
  }

  h1 {
    max-width: 13ch;
    margin: 0;
    font-size: var(--ah-type-display);
    line-height: var(--ah-leading-display);
    letter-spacing: var(--ah-tracking-display);
  }

  .hero-lead {
    max-width: 34rem;
    margin: 1.6rem 0 0;
    color: var(--ah-muted);
    font-size: var(--ah-type-lead);
    line-height: var(--ah-leading-body);
  }

  .hero-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.65rem;
    margin-top: 2rem;
  }

  .hero-notes {
    display: flex;
    flex-wrap: wrap;
    gap: 0.7rem 1.4rem;
    margin: 2rem 0 0;
    padding: 0;
    color: var(--ah-muted);
    font-size: 0.9rem;
    list-style: none;
  }

  .hero-notes li {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .note-dot {
    width: 0.6rem;
    height: 0.6rem;
    border-radius: 50%;
    background: var(--ah-green);
  }

  .note-dot-blue {
    background: var(--ah-blue);
  }

  .note-dot-yellow {
    clip-path: polygon(50% 0, 100% 50%, 50% 100%, 0 50%);
    border-radius: 0;
    background: var(--ah-yellow);
  }

  .hero-visual {
    position: relative;
    display: grid;
    min-height: 32rem;
    align-items: center;
    justify-items: center;
    padding: 1.5rem 0;
  }

  .visual-frame {
    width: 100%;
    max-width: 30rem;
    overflow: hidden;
    padding: 0 0.9rem 1.1rem;
    border-radius: 13rem 13rem 2rem 2rem;
    background: var(--ah-ink);
    color: var(--ah-cream);
  }

  .visual-bar {
    display: flex;
    min-height: 3.25rem;
    align-items: center;
    gap: 0.35rem;
    padding: 0 0.4rem;
    border-bottom: 1px solid
      color-mix(in srgb, var(--ah-cream) 14%, transparent);
    color: var(--ah-muted-on-dark);
  }

  .visual-bar > span {
    width: 0.6rem;
    height: 0.6rem;
    border-radius: 50%;
    background: var(--ah-coral);
  }

  .visual-bar > span:nth-child(2) {
    background: var(--ah-yellow);
  }

  .visual-bar > span:nth-child(3) {
    background: var(--ah-green);
  }

  .visual-bar p {
    margin: 0 0 0 auto;
    font-size: 0.75rem;
  }

  .visual-body {
    padding: 1.4rem 0.4rem 0.2rem;
  }

  .visual-list {
    display: grid;
    gap: 0.95rem;
    margin: 0;
    padding: 0 0.3rem;
    list-style: none;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.78rem;
    color: color-mix(in srgb, var(--ah-cream) 80%, transparent);
  }

  .visual-list span {
    margin-right: 0.7rem;
    color: var(--ah-yellow);
  }

  .visual-cards {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 0.5rem;
    margin-top: 1.4rem;
  }

  .visual-card {
    display: grid;
    min-height: 6rem;
    align-content: space-between;
    gap: 0.75rem;
    padding: 0.8rem;
    border-radius: 0.9rem;
    color: var(--ah-ink);
  }

  .visual-card span {
    font-size: 0.75rem;
    font-weight: 500;
  }

  .visual-card strong {
    font-size: 0.82rem;
    font-weight: 500;
    line-height: 1.2;
  }

  .card-blue {
    background: var(--ah-blue);
  }

  .card-yellow {
    background: var(--ah-yellow);
  }

  .card-coral {
    background: var(--ah-coral);
  }

  .float-shape {
    position: absolute;
    z-index: 2;
    display: block;
    border-radius: 0.4rem;
    background: var(--ah-coral);
  }

  .float-arch {
    top: 4%;
    right: -1.5%;
    width: 4.5rem;
    height: 6rem;
    border-radius: 50% 50% 0.6rem 0.6rem / 55% 55% 0 0;
    transform: rotate(7deg);
  }

  .float-circle {
    bottom: 9%;
    left: -2%;
    width: 4rem;
    height: 4rem;
    border-radius: 50%;
    background: var(--ah-yellow);
  }

  .float-plus {
    top: 20%;
    left: -3%;
    width: 2.6rem;
    height: 2.6rem;
    background: var(--ah-pink);
    clip-path: polygon(
      34% 0,
      66% 0,
      66% 34%,
      100% 34%,
      100% 66%,
      66% 66%,
      66% 100%,
      34% 100%,
      34% 66%,
      0 66%,
      0 34%,
      34% 34%
    );
  }

  .float-diamond {
    right: 7%;
    bottom: 1%;
    width: 2rem;
    height: 2rem;
    border-radius: 0;
    background: var(--ah-blue);
    clip-path: polygon(50% 0, 100% 50%, 50% 100%, 0 50%);
  }

  .workspace {
    display: grid;
    width: var(--wrap);
    grid-template-columns: minmax(0, 1fr) minmax(17rem, 0.85fr);
    align-items: center;
    gap: 2rem;
    margin: 0 auto clamp(2rem, 5vw, 4rem);
    padding: clamp(1.5rem, 4vw, 2.75rem);
    border-radius: 2rem;
    background: var(--ah-surface);
  }

  .workspace h2 {
    margin: 0;
    font-size: clamp(1.6rem, 3vw, 2.4rem);
    line-height: 1.06;
    letter-spacing: -0.03em;
  }

  .workspace p:last-child {
    margin-bottom: 0;
    color: var(--ah-muted);
  }

  .workspace-links {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.5rem;
  }

  .workspace-links a {
    display: flex;
    min-height: 3.25rem;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.8rem 1rem;
    border-radius: 1rem;
    background: var(--ah-cream);
    font-weight: 500;
    text-decoration: none;
  }

  .section {
    width: var(--wrap);
    margin: 0 auto;
    padding: var(--ah-space-section) 0;
  }

  .section-heading {
    max-width: 46rem;
    margin: 0 auto clamp(2.5rem, 5vw, 4.5rem);
    text-align: center;
  }

  .section-heading h2,
  .workflow-copy h2,
  .faq-heading h2,
  .closing h2 {
    margin: 0;
    font-size: var(--ah-type-h2);
    line-height: var(--ah-leading-h2);
    letter-spacing: var(--ah-tracking-h2);
  }

  .section-heading > p:last-child,
  .workflow-copy > p:last-child,
  .closing > p {
    color: var(--ah-muted);
    font-size: 1.05rem;
    line-height: 1.6;
  }

  .service-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: clamp(0.9rem, 1.6vw, 1.25rem);
  }

  .service-panel {
    display: flex;
    min-height: 24rem;
    flex-direction: column;
    padding: clamp(1.5rem, 3vw, 2.5rem);
    border-radius: 2rem;
    color: var(--ah-ink);
    transition:
      transform var(--ah-motion-standard) cubic-bezier(0.25, 0.8, 0.25, 1),
      box-shadow var(--ah-motion-standard) ease;
  }

  .service-panel:hover {
    transform: translateY(-4px);
    box-shadow: 0 8px 0 color-mix(in srgb, var(--ah-ink) 16%, transparent);
  }

  .service-panel.blue {
    background: var(--ah-blue);
  }

  .service-panel.yellow {
    background: var(--ah-yellow);
  }

  .service-panel.coral {
    color: var(--ah-cream);
    background: var(--ah-rust);
  }

  .service-panel.coral .service-link {
    border-top-color: color-mix(in srgb, var(--ah-cream) 35%, transparent);
  }

  .service-panel.green {
    background: var(--ah-green);
  }

  .service-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .badge {
    display: inline-flex;
    min-height: 1.875rem;
    align-items: center;
    padding: 0 0.875rem;
    border-radius: 999px;
    color: var(--ah-cream);
    background: var(--ah-ink);
    font-size: 0.8rem;
    font-weight: 500;
    line-height: 1;
  }

  .service-shape {
    width: 2.6rem;
    height: 2.6rem;
    border-radius: 50% 50% 0 50%;
    background: var(--ah-cream);
  }

  .service-panel:nth-child(2) .service-shape {
    border-radius: 50%;
  }

  .service-panel:nth-child(3) .service-shape {
    border-radius: 50% 50% 0.4rem 0.4rem / 55% 55% 0 0;
  }

  .service-panel:nth-child(4) .service-shape {
    border-radius: 0;
    clip-path: polygon(50% 0, 100% 50%, 50% 100%, 0 50%);
  }

  .service-panel h3 {
    max-width: 13ch;
    margin: auto 0 0.9rem;
    font-size: clamp(1.7rem, 2.6vw, 2.25rem);
    line-height: 1.02;
    letter-spacing: -0.03em;
  }

  .service-panel > p {
    margin: 0;
    font-size: 1rem;
    line-height: 1.55;
  }

  .service-panel ul {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
    margin: 1.1rem 0 0;
    padding: 0;
    list-style: none;
  }

  .service-panel li {
    padding: 0.4rem 0.75rem;
    border-radius: 999px;
    background: var(--ah-cream);
    font-size: 0.8rem;
    font-weight: 500;
    line-height: 1.2;
  }

  .service-link {
    display: flex;
    min-height: 3rem;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    margin-top: 1.4rem;
    padding-top: 0.9rem;
    border-top: 1.5px solid var(--ah-line-strong);
    font-size: 0.95rem;
    font-weight: 500;
    text-decoration: none;
  }

  .service-link:hover span:first-child,
  .service-link:focus-visible span:first-child {
    text-decoration: underline;
    text-decoration-thickness: 1.5px;
    text-underline-offset: 0.25rem;
  }

  .workflow-band {
    display: grid;
    width: var(--wrap);
    grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr);
    gap: clamp(2.5rem, 6vw, 6rem);
    margin: clamp(2rem, 5vw, 4rem) auto;
    padding: clamp(2.25rem, 5vw, 4.5rem);
    border-radius: 2rem;
    color: var(--ah-cream);
    background: var(--ah-ink);
  }

  .workflow-band .kicker {
    color: var(--ah-muted-on-dark);
  }

  .workflow-copy h2 {
    max-width: 13ch;
    font-size: clamp(2rem, 3.6vw, 3.5rem);
  }

  .workflow-copy > p:last-child {
    max-width: 30rem;
    margin: 1.4rem 0 0;
    color: var(--ah-muted-on-dark);
    font-size: 1rem;
  }

  .workflow-steps {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .workflow-steps li {
    display: grid;
    grid-template-columns: 2.5rem minmax(0, 1fr);
    gap: 1.1rem;
    padding: 1.4rem 0;
    border-top: 1.5px solid color-mix(in srgb, var(--ah-cream) 24%, transparent);
  }

  .workflow-steps li:last-child {
    border-bottom: 1.5px solid
      color-mix(in srgb, var(--ah-cream) 24%, transparent);
  }

  .step-index {
    display: grid;
    width: 2.4rem;
    height: 2.4rem;
    place-items: center;
    border-radius: 50%;
    color: var(--ah-ink);
    background: var(--ah-yellow);
    font-size: 0.78rem;
    font-weight: 500;
  }

  .workflow-steps h3 {
    margin: 0;
    font-size: 1.3rem;
    letter-spacing: -0.02em;
  }

  .workflow-steps p {
    margin: 0.5rem 0 0;
    color: var(--ah-muted-on-dark);
    font-size: 0.95rem;
    line-height: 1.55;
  }

  .split-heading {
    display: grid;
    max-width: none;
    grid-template-columns: minmax(0, 1fr) minmax(17rem, 0.6fr);
    align-items: end;
    gap: 2.5rem;
    text-align: left;
  }

  .split-heading > p {
    margin: 0;
    color: var(--ah-muted);
    font-size: 1rem;
    line-height: 1.6;
  }

  .information-list {
    border-top: 1.5px solid var(--ah-ink);
  }

  .information-list > a {
    display: grid;
    min-height: 6.75rem;
    grid-template-columns: 4rem minmax(0, 1fr) auto;
    align-items: center;
    gap: 1.25rem;
    padding: 1rem 0;
    border-bottom: 1.5px solid var(--ah-ink);
    text-decoration: none;
  }

  .information-index {
    color: var(--ah-muted);
    font-size: 0.9rem;
  }

  .information-list strong {
    display: block;
    font-size: clamp(1.2rem, 2.5vw, 1.75rem);
    font-weight: 500;
    letter-spacing: -0.02em;
  }

  .information-list small {
    display: block;
    margin-top: 0.4rem;
    color: var(--ah-muted);
    font-size: 0.95rem;
  }

  .information-arrow {
    display: grid;
    width: 3rem;
    height: 3rem;
    place-items: center;
    border-radius: 50%;
    background: var(--ah-surface);
    font-size: 1.1rem;
    transition:
      background-color 0.3s ease,
      transform 0.3s ease;
  }

  .information-list > a:hover .information-arrow,
  .information-list > a:focus-visible .information-arrow {
    background: var(--ah-coral);
    transform: rotate(8deg);
  }

  .trust-note {
    display: grid;
    grid-template-columns: 7rem minmax(0, 1fr);
    gap: clamp(1.5rem, 5vw, 4rem);
    margin-top: clamp(2.5rem, 6vw, 5rem);
    padding: clamp(1.5rem, 4vw, 3rem);
    border-radius: 2rem;
    background: var(--ah-surface);
  }

  .trust-mark {
    display: grid;
    width: 6rem;
    height: 7.5rem;
    place-items: center;
    border-radius: 50% 50% 0.9rem 0.9rem / 55% 55% 0 0;
    background: var(--ah-coral);
  }

  .trust-mark span {
    width: 2rem;
    height: 3rem;
    border: 0.4rem solid var(--ah-cream);
    border-top: 0;
    border-radius: 0 0 2rem 2rem;
    transform: rotate(-45deg);
  }

  .trust-note h3 {
    max-width: 24ch;
    margin: 0;
    font-size: clamp(1.6rem, 3.4vw, 2.75rem);
    line-height: 1.05;
    letter-spacing: -0.035em;
  }

  .trust-note p:last-child {
    max-width: 48rem;
    margin-bottom: 0;
    color: var(--ah-muted);
    font-size: 1rem;
    line-height: 1.6;
  }

  .faq {
    display: grid;
    grid-template-columns: minmax(14rem, 0.6fr) minmax(0, 1.4fr);
    gap: clamp(2rem, 6vw, 7rem);
  }

  .faq-heading {
    align-self: start;
  }

  .faq-list {
    border-top: 1.5px solid var(--ah-ink);
  }

  .faq details {
    border-bottom: 1.5px solid var(--ah-ink);
  }

  .faq summary {
    position: relative;
    padding: 1.5rem 3rem 1.5rem 0;
    cursor: pointer;
    font-size: clamp(1.05rem, 1.8vw, 1.375rem);
    font-weight: 500;
    letter-spacing: -0.02em;
    line-height: 1.3;
    list-style: none;
  }

  .faq summary::-webkit-details-marker {
    display: none;
  }

  .faq summary::after {
    position: absolute;
    top: 50%;
    right: 0.25rem;
    content: '+';
    color: var(--ah-coral);
    font-size: 1.9rem;
    font-weight: 400;
    line-height: 1;
    transform: translateY(-50%);
  }

  .faq details[open] summary::after {
    content: '−';
  }

  .faq details p {
    max-width: 44rem;
    margin: 0;
    padding: 0 3rem 1.5rem 0;
    color: var(--ah-muted);
    font-size: 1rem;
    line-height: 1.6;
  }

  .closing {
    position: relative;
    width: var(--wrap);
    overflow: hidden;
    margin: clamp(3rem, 7vw, 6rem) auto 1.25rem;
    padding: clamp(4.5rem, 9vw, 9rem) clamp(1.25rem, 6vw, 6rem)
      clamp(3rem, 6vw, 5rem);
    border-radius: clamp(3.5rem, 12vw, 13rem) clamp(3.5rem, 12vw, 13rem) 2rem
      2rem;
    background: var(--ah-coral);
    color: var(--ah-cream);
    text-align: center;
  }

  .closing > *:not(.closing-shape) {
    position: relative;
    z-index: 1;
  }

  .closing .kicker {
    color: var(--ah-cream);
  }

  .closing h2 {
    max-width: 15ch;
    margin-right: auto;
    margin-left: auto;
  }

  .closing > p {
    max-width: 40rem;
    margin: 1.4rem auto 2rem;
    color: var(--ah-cream);
  }

  .closing-shape {
    position: absolute;
    top: -4rem;
    left: -3rem;
    width: 12rem;
    height: 12rem;
    border-radius: 50%;
    background: var(--ah-yellow);
  }

  footer {
    display: grid;
    width: var(--wrap);
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 2.5rem;
    margin: 0 auto;
    padding: clamp(2rem, 4vw, 3.5rem);
    border-radius: 2rem 2rem 0 0;
    color: var(--ah-cream);
    background: var(--ah-ink);
  }

  footer > * {
    min-width: 0;
  }

  .footer-brand p {
    max-width: 24rem;
    color: var(--ah-muted-on-dark);
    line-height: 1.6;
  }

  footer .brand-mark {
    box-sizing: content-box;
    height: 2rem;
    padding: 0.35rem;
    border-radius: 0.45rem;
    background: var(--ah-cream);
  }

  footer nav {
    display: grid;
    min-width: 0;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: clamp(1rem, 4vw, 2.5rem);
  }

  footer nav div {
    display: grid;
    align-content: start;
    gap: 0.75rem;
  }

  footer nav strong {
    color: var(--ah-muted-on-dark);
    font-size: 0.9rem;
    font-weight: 500;
  }

  footer nav a {
    display: inline-flex;
    min-height: 44px;
    align-items: center;
    color: var(--ah-cream);
    font-size: 0.9rem;
    text-decoration: none;
  }

  footer nav a:hover,
  footer nav a:focus-visible {
    text-decoration: underline;
    text-underline-offset: 0.3rem;
  }

  .copyright {
    display: flex;
    grid-column: 1 / -1;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    margin: 0.75rem 0 0;
    padding-top: 1.25rem;
    border-top: 1px solid color-mix(in srgb, var(--ah-cream) 24%, transparent);
    color: var(--ah-muted-on-dark);
    font-size: 0.8rem;
  }

  .copyright a {
    display: inline-flex;
    min-height: 24px;
    align-items: center;
    padding: 0 0.15rem;
  }

  @media (prefers-reduced-motion: no-preference) {
    .hero-copy {
      animation: hero-rise var(--ah-duration-hero) var(--ah-ease-out) backwards;
    }

    .hero-visual {
      animation: hero-rise var(--ah-duration-hero-slow) var(--ah-delay-hero)
        var(--ah-ease-out) backwards;
    }

    @supports (animation-timeline: view()) {
      [data-reveal] {
        animation: reveal-rise linear both;
        animation-timeline: view();
        animation-range: entry 6% cover 24%;
      }
    }
  }

  @keyframes hero-rise {
    from {
      opacity: 0;
      transform: translateY(var(--ah-rise));
    }
  }

  @keyframes reveal-rise {
    from {
      opacity: 0;
      transform: translateY(var(--ah-rise));
    }
    to {
      opacity: 1;
      transform: none;
    }
  }

  @media (min-width: 721px) and (max-width: 960px) {
    .brand {
      gap: 0.45rem;
      font-size: 1.15rem;
    }

    .brand-mark {
      height: 2.2rem;
    }

    .desktop-nav a {
      padding: 0 0.55rem;
      font-size: 0.88rem;
    }

    .text-link {
      display: none;
    }

    .button-small {
      padding: 0 0.9rem;
      font-size: 0.85rem;
    }
  }

  @media (max-width: 900px) {
    .hero {
      grid-template-columns: 1fr;
      padding-top: 7rem;
    }

    .hero-visual {
      width: min(30rem, 100%);
      min-height: 0;
      margin: 0 auto;
      padding: 2rem 0 1rem;
    }

    .workflow-band,
    .faq {
      grid-template-columns: 1fr;
    }

    .split-heading {
      grid-template-columns: 1fr;
      align-items: start;
      gap: 1rem;
    }
  }

  @media (max-width: 720px) {
    :global(html) {
      scroll-padding-top: 5.5rem;
    }

    .desktop-nav,
    .header-actions {
      display: none;
    }

    .mobile-menu {
      display: block;
    }

    .landing {
      --wrap: min(var(--ah-content-max), calc(100% - 1.25rem));
    }

    .site-header {
      padding: 0.6rem 0;
    }

    .hero {
      gap: 1rem;
      padding: 6.5rem 0 3rem;
    }

    h1 {
      font-size: clamp(2.5rem, 11vw, 3.5rem);
    }

    .hero-actions,
    .hero-actions .button {
      width: 100%;
    }

    .hero-notes {
      display: grid;
      gap: 0.6rem;
    }

    .visual-frame {
      border-radius: 8rem 8rem 1.5rem 1.5rem;
    }

    .visual-card {
      min-height: 5rem;
      padding: 0.65rem;
    }

    .float-arch {
      width: 3.5rem;
      height: 4.75rem;
    }

    .float-circle {
      width: 3rem;
      height: 3rem;
    }

    .float-plus {
      width: 2.1rem;
      height: 2.1rem;
    }

    .workspace,
    .service-grid,
    .workspace-links,
    footer {
      grid-template-columns: 1fr;
    }

    .section {
      padding: 4.5rem 0;
    }

    .section-heading {
      text-align: left;
    }

    .service-panel {
      min-height: 21rem;
    }

    .workflow-band {
      gap: 2.25rem;
      padding: 2rem 1.25rem;
    }

    .workflow-copy h2 {
      max-width: none;
    }

    .workflow-steps li {
      grid-template-columns: 2.2rem minmax(0, 1fr);
      gap: 0.8rem;
    }

    .information-list > a {
      min-height: 7.5rem;
      grid-template-columns: 2rem minmax(0, 1fr) auto;
      gap: 0.85rem;
    }

    .information-list small {
      max-width: 17rem;
    }

    .information-arrow {
      width: 2.75rem;
      height: 2.75rem;
    }

    .trust-note {
      grid-template-columns: 1fr;
    }

    .trust-mark {
      width: 5rem;
      height: 6rem;
    }

    .closing {
      padding: 4rem 1.25rem 3rem;
      border-radius: 4rem 4rem 1.5rem 1.5rem;
    }

    .closing-shape {
      top: -5rem;
      left: -5rem;
      width: 10rem;
      height: 10rem;
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
      grid-template-columns: minmax(0, 1fr) auto;
    }

    .information-index {
      display: none;
    }

    footer nav {
      grid-template-columns: 1fr;
    }
  }
</style>
