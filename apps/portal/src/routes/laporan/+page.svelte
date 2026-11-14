<script lang="ts">
  import PortalPage from '../../lib/PortalPage.svelte';
  import { appLink } from '../../lib/app-link';
  import { reportRules, reportSections } from '../../lib/portal-content';
</script>

<PortalPage
  eyebrow="Laporan"
  docTitle="Panduan laporan — Algohub"
  heroPoints={['Sembilan bagian laporan', 'Empat aturan pengumpulan']}
  title="Laporan yang bisa dinilai, bukan sekadar dikumpulkan."
  lead="Struktur baku, aturan pengumpulan, dan batas antara latihan kode di browser dengan nilai akademik yang sah."
>
  <div class="sections">
    <section class="section" aria-labelledby="struktur">
      <h2 id="struktur">Struktur laporan</h2>
      <div class="section-body">
        <p>
          Urutan ini dipakai agar asisten dapat menilai laporan tanpa menebak
          bagian mana yang berisi hasil dan bagian mana yang berisi pendapat.
        </p>
        <ol class="items">
          {#each reportSections as section, index (section.title)}
            <li>
              <span class="index">{String(index + 1).padStart(2, '0')}</span>
              <strong>{section.title}</strong>
              <p>{section.body}</p>
            </li>
          {/each}
        </ol>
      </div>
    </section>

    <section class="section" aria-labelledby="aturan">
      <h2 id="aturan">Aturan pengumpulan</h2>
      <div class="section-body">
        <ul class="items">
          {#each reportRules as rule, index (rule)}
            <li>
              <span class="index">{String(index + 1).padStart(2, '0')}</span>
              <p>{rule}</p>
            </li>
          {/each}
        </ul>
        <div class="note">
          <p class="note-strong">Latihan kode bukan penilaian</p>
          <p>
            Hasil eksekusi C atau Python di browser hanya umpan balik belajar
            dan progres gamifikasi. Nilai Praktikum tetap berasal dari
            submission yang ditinjau manual.
          </p>
        </div>
      </div>
    </section>
  </div>

  <div class="cta">
    <p>Kumpulkan laporan pada sesi yang sesuai sebelum tenggatnya ditutup.</p>
    <a class="cta-link" href={appLink('praktikum')}>Buka Praktikum</a>
  </div>
</PortalPage>
