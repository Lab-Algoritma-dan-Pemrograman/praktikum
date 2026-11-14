<script lang="ts">
  import PortalPage from '../../lib/PortalPage.svelte';
  import { appLink } from '../../lib/app-link';
  import { scheduleNotes, weeklyPhases } from '../../lib/portal-content';
</script>

<PortalPage
  eyebrow="Jadwal"
  docTitle="Jadwal praktikum — Algohub"
  heroPoints={[
    'Empat tahap di setiap sesi',
    'Data yang tidak dibuka ke publik'
  ]}
  title="Ritme pekan praktikum, dari aktivasi sampai publikasi nilai."
  lead="Jadwal personal dan token sesi hanya terbuka setelah masuk. Halaman ini menjelaskan urutan yang berlaku di setiap sesi, tanpa membuka data kelas."
>
  <div class="sections">
    <section class="section" aria-labelledby="alur">
      <h2 id="alur">Alur satu pekan</h2>
      <div class="section-body">
        <p>
          Setiap sesi melewati empat tahap yang sama. Asisten membuka sesi,
          peserta mengerjakan di aplikasi Praktikum, lalu nilai diputuskan lewat
          review manual sebelum dipublikasikan.
        </p>
        <ol class="items">
          {#each weeklyPhases as phase (phase.step)}
            <li>
              <span class="index">{phase.step}</span>
              <strong>{phase.title}</strong>
              <p>{phase.body}</p>
            </li>
          {/each}
        </ol>
      </div>
    </section>

    <section class="section" aria-labelledby="privat">
      <h2 id="privat">Yang tidak dipublikasikan</h2>
      <div class="section-body">
        <ul class="items">
          {#each scheduleNotes as note, index (note)}
            <li>
              <span class="index">{String(index + 1).padStart(2, '0')}</span>
              <p>{note}</p>
            </li>
          {/each}
        </ul>
        <p>
          Jadwal lengkap per kelas tersedia di dalam aplikasi Praktikum setelah
          masuk, karena setiap peserta hanya melihat sesi yang menjadi
          scope-nya.
        </p>
      </div>
    </section>
  </div>

  <div class="cta">
    <p>
      Lihat sesi, tenggat, dan token presensi milikmu di aplikasi Praktikum.
    </p>
    <a class="cta-link" href={appLink('praktikum')}>Buka Praktikum</a>
  </div>
</PortalPage>
