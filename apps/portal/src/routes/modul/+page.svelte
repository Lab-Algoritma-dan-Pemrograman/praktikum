<script lang="ts">
  import PortalPage from '../../lib/PortalPage.svelte';
  import { appLink } from '../../lib/app-link';
  import { moduleAccess, modulePacks } from '../../lib/portal-content';
</script>

<PortalPage
  eyebrow="Modul"
  docTitle="Modul praktikum — Algohub"
  title="Modul praktikum: apa yang dibagikan dan di mana mengaksesnya."
  lead="Halaman ini menjelaskan jenis dokumen tiap modul dan cara membukanya. Berkas aslinya tetap berada di aplikasi yang memegang sesi, bukan disalin ke portal publik."
>
  <div class="sections">
    <section class="section" aria-labelledby="jenis">
      <h2 id="jenis">Jenis dokumen</h2>
      <div class="section-body">
        <p>
          Setiap modul praktikum tersusun dari dokumen yang sama, sehingga
          peserta tahu apa yang harus dibaca sebelum sesi dimulai.
        </p>
        <ul class="items">
          {#each modulePacks as pack, index (pack.name)}
            <li>
              <span class="index">{String(index + 1).padStart(2, '0')}</span>
              <strong>{pack.name}</strong>
              <p>{pack.body}</p>
            </li>
          {/each}
        </ul>
      </div>
    </section>

    <section class="section" aria-labelledby="akses">
      <h2 id="akses">Cara mengakses</h2>
      <div class="section-body">
        <ul class="items">
          {#each moduleAccess as item, index (item)}
            <li>
              <span class="index">{String(index + 1).padStart(2, '0')}</span>
              <p>{item}</p>
            </li>
          {/each}
        </ul>
        <p>
          Materi belajar berjenjang, quiz, dan latihan kode tersedia di
          E-learning. Modul praktikum yang sedang berjalan mengikuti sesi di
          aplikasi Praktikum.
        </p>
      </div>
    </section>
  </div>

  <div class="cta">
    <p>
      Materi dan progres belajar ada di E-learning; sesi aktif ada di Praktikum.
    </p>
    <a class="cta-link" href={appLink('elearning')}>Buka E-learning</a>
  </div>
</PortalPage>
