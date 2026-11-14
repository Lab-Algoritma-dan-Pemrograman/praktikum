<script lang="ts">
  import PortalPage from '../../lib/PortalPage.svelte';
  import { divisions } from '../../lib/portal-content';
</script>

<PortalPage
  eyebrow="Asisten"
  docTitle="Divisi asisten — Algohub"
  heroPoints={['Enam divisi operasional', 'Kebijakan kontak dan privasi']}
  title="Divisi yang menjalankan laboratorium."
  lead="Struktur asisten dipublikasikan sebagai divisi dan tanggung jawab. Nama, kontak pribadi, dan penugasan internal tidak ditampilkan di halaman publik."
>
  <div class="sections">
    <section class="section" aria-labelledby="divisi">
      <h2 id="divisi">Enam divisi operasional</h2>
      <div class="section-body">
        <p>
          Divisi menjelaskan bidang kerja, bukan tingkat akses. Permission tetap
          berasal dari role dan scope yang diberikan pada akun, sehingga nama
          divisi tidak pernah menjadi dasar otorisasi.
        </p>
        <ol class="items">
          {#each divisions as division, index (division.code)}
            <li>
              <span class="index">{String(index + 1).padStart(2, '0')}</span>
              <strong>{division.code}</strong>
              <p>{division.focus}</p>
            </li>
          {/each}
        </ol>
      </div>
    </section>

    <section class="section" aria-labelledby="privasi">
      <h2 id="privasi">Kenapa tanpa direktori nama</h2>
      <div class="section-body">
        <p>
          Publikasi profil asisten membutuhkan persetujuan masing-masing orang
          dan aturan kontak yang jelas. Selama keputusan itu belum ada, halaman
          ini hanya memuat informasi yang aman dibagikan.
        </p>
        <div class="note">
          <p class="note-strong">Butuh menghubungi asisten?</p>
          <p>
            Gunakan kanal resmi laboratorium atau ajukan pertanyaan melalui
            halaman bantuan. Pesan pribadi di luar kanal resmi tidak menjadi
            bukti pengumpulan tugas.
          </p>
        </div>
      </div>
    </section>
  </div>

  <div class="cta">
    <p>Pertanyaan tentang jadwal jaga, presensi, atau pengumpulan tugas?</p>
    <a class="cta-link" href="/bantuan">Buka bantuan</a>
  </div>
</PortalPage>
