<script lang="ts">
  import {
    canAccessAdminRoute,
    type Principal,
    type UserProfile
  } from '@algohub/auth-client';
  export let data: {
    principal: Principal;
    overview: { user: UserProfile; domains: Record<string, boolean> };
  };
  const domains = [
    { key: 'praktikum', label: 'Praktikum' },
    { key: 'elearning', label: 'E-learning' },
    { key: 'siakad', label: 'SIAKAD' }
  ];
  $: available = domains.filter(
    (domain) =>
      data.overview.domains[domain.key] &&
      canAccessAdminRoute(data.principal, `/${domain.key}`)
  );
</script>

<svelte:head><title>Ringkasan — Algohub Admin</title></svelte:head>
<section class="identity-page">
  <p class="eyebrow">CONSOLE TERPUSAT / RINGKASAN</p>
  <h1>Ruang kendali laboratorium</h1>
  <p>
    Selamat datang, <strong>{data.overview.user.name}</strong>. Pilih ruang
    sesuai permission dan scope yang diberikan kepada Anda.
  </p>
  <div class="identity-panel">
    <h2>Ruang aplikasi</h2>
    {#if available.length}
      <nav aria-label="Ruang aplikasi admin">
        <ul>
          {#each available as domain}<li>
              <a class="button" href={`/${domain.key}`}>{domain.label}</a>
            </li>{/each}
        </ul>
      </nav>
      <p class="muted">
        Buka domain untuk mengelola fitur yang tersedia sesuai permission dan
        scope Anda.
      </p>
    {:else}<p>
        Belum ada akses pengelolaan domain. Profil dan sesi tetap dapat dikelola
        melalui menu Profil.
      </p>{/if}
  </div>
</section>
