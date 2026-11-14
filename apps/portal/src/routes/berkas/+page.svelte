<script lang="ts">
  import PortalPage from '../../lib/PortalPage.svelte';

  type Berkas = {
    nama: string;
    kategori: string;
    ukuran: string;
    versi: string;
    diperbarui: string;
    href?: string;
  };

  const berkasList: Berkas[] = [
    {
      nama: 'Kalender pekan praktikum',
      kategori: 'Jadwal',
      ukuran: '96 KB',
      versi: 'v3',
      diperbarui: '10 Feb 2025',
      href: '/jadwal'
    },
    {
      nama: 'Panduan penggunaan portal',
      kategori: 'Panduan',
      ukuran: '412 KB',
      versi: 'v2',
      diperbarui: '4 Feb 2025',
      href: '/bantuan'
    },
    {
      nama: 'Template laporan praktikum',
      kategori: 'Laporan',
      ukuran: '88 KB',
      versi: 'v4',
      diperbarui: '28 Jan 2025',
      href: '/laporan'
    },
    {
      nama: 'Template modul praktikum',
      kategori: 'Modul',
      ukuran: '64 KB',
      versi: 'v5',
      diperbarui: '20 Jan 2025',
      href: '/modul'
    },
    {
      nama: 'SOP keselamatan laboratorium',
      kategori: 'Lab',
      ukuran: '120 KB',
      versi: 'v2',
      diperbarui: '12 Jan 2025'
    },
    {
      nama: 'Format surat keterangan asisten',
      kategori: 'Administrasi',
      ukuran: '45 KB',
      versi: 'v1',
      diperbarui: '6 Jan 2025'
    }
  ];

  const kategori = [
    'Semua',
    ...Array.from(new Set(berkasList.map((b) => b.kategori)))
  ];

  let dipilih = 'Semua';
  $: tampil =
    dipilih === 'Semua'
      ? berkasList
      : berkasList.filter((b) => b.kategori === dipilih);
</script>

<PortalPage
  eyebrow="Berkas"
  docTitle="Berkas dan unduhan — Algohub"
  lead="Dokumen publik laboratorium: kalender, panduan, template laporan dan modul, serta SOP keselamatan."
  heroPoints={['6 dokumen publik', 'Selalu versi terbaru']}
  title="Dokumen yang dipakai sehari-hari."
>
  <div class="sections">
    <section class="section" aria-labelledby="daftar-berkas">
      <h2 id="daftar-berkas">Daftar berkas</h2>
      <div class="section-body">
        <p>
          Setiap berkas mencantumkan versi dan tanggal pembaruan terakhir, supaya
          mudah memastikan dokumen yang dipakai adalah yang terbaru.
        </p>
      </div>

      <ul class="chips" aria-label="Saring berkas menurut kategori">
        {#each kategori as k (k)}
          <li>
            <button
              type="button"
              class="chip"
              class:is-active={dipilih === k}
              aria-pressed={dipilih === k}
              onclick={() => (dipilih = k)}>{k}</button
            >
          </li>
        {/each}
      </ul>

      <div class="berkas-table">
        <table>
          <caption class="sr-only">Berkas publik beserta versi dan tautannya</caption>
          <thead>
            <tr>
              <th scope="col">Nama berkas</th>
              <th scope="col">Kategori</th>
              <th scope="col">Ukuran</th>
              <th scope="col">Versi</th>
              <th scope="col">Diperbarui</th>
              <th scope="col"><span class="sr-only">Unduh</span></th>
            </tr>
          </thead>
          <tbody>
            {#each tampil as berkas (berkas.nama)}
              <tr>
                <th scope="row">{berkas.nama}</th>
                <td><span class="tag">{berkas.kategori}</span></td>
                <td class="num">{berkas.ukuran}</td>
                <td class="num">{berkas.versi}</td>
                <td class="num">{berkas.diperbarui}</td>
                <td class="act">
                  {#if berkas.href}
                    <a class="unduh" href={berkas.href}>Buka</a>
                  {:else}
                    <span class="unduh is-off">Segera</span>
                  {/if}
                </td>
              </tr>
            {:else}
              <tr><td class="empty" colspan="6">Belum ada berkas di kategori ini.</td></tr>
            {/each}
          </tbody>
        </table>
      </div>

      <div class="note">
        <p>
          Berkas bertanda <strong>Segera</strong> belum diterbitkan pengelola
          laboratorium. Butuh dokumen lain? Ajukan lewat
          <a href="/bantuan">halaman bantuan</a>.
        </p>
      </div>
    </section>
  </div>
</PortalPage>

<style>
  .chips li button {
    min-height: 44px;
  }

  .berkas-table {
    overflow-x: auto;
    margin-top: 1rem;
    border: 1px solid var(--ah-line);
    border-radius: 0.9rem;
    background: #ffffff;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.88rem;
  }

  th,
  td {
    padding: 0.65rem 0.9rem;
    border-bottom: 1px solid color-mix(in srgb, var(--ah-line) 70%, transparent);
    text-align: left;
    vertical-align: middle;
    white-space: nowrap;
  }

  thead th {
    color: var(--ah-muted);
    font-size: 0.75rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  tbody th {
    font-weight: 600;
    white-space: normal;
  }

  tbody tr:last-child th,
  tbody tr:last-child td {
    border-bottom: 0;
  }

  tbody tr:hover {
    background: color-mix(in srgb, var(--ah-coral) 4%, transparent);
  }

  .num {
    color: var(--ah-muted);
    font-variant-numeric: tabular-nums;
  }

  .tag {
    display: inline-flex;
    align-items: center;
    padding: 0.1rem 0.55rem;
    border-radius: 999px;
    color: var(--ah-ink);
    background: color-mix(in srgb, var(--ah-ink) 6%, transparent);
    font-size: 0.75rem;
    font-weight: 600;
  }

  .act {
    text-align: right;
  }

  .unduh {
    display: inline-flex;
    min-height: 44px;
    align-items: center;
    padding-inline: 0.6rem;
    border-radius: 0.6rem;
    color: var(--ah-coral);
    font-size: 0.82rem;
    font-weight: 600;
    text-decoration: none;
  }

  .unduh:hover,
  .unduh:focus-visible {
    background: color-mix(in srgb, var(--ah-coral) 8%, transparent);
  }

  .unduh.is-off {
    color: var(--ah-muted);
    font-weight: 500;
  }

  .empty {
    color: var(--ah-muted);
    font-size: 0.85rem;
    white-space: normal;
  }

  @media (max-width: 44rem) {
    /* Below a phone width the secondary columns go; name, category,
       updated date and the action stay. */
    th:nth-child(3),
    td:nth-child(3),
    th:nth-child(4),
    td:nth-child(4) {
      display: none;
    }

    th,
    td {
      padding: 0.6rem 0.7rem;
      white-space: normal;
    }
  }

  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }
</style>
