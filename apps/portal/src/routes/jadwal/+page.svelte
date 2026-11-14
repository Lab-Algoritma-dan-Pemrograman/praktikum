<script lang="ts">
  import PortalPage from '../../lib/PortalPage.svelte';
  import {
    HARI,
    JAM_MULAI,
    JAM_SELESAI,
    TINGGI_BARIS,
    pekanList,
    sesiPerPekan,
    menit
  } from '../../lib/jadwal-data';

  let pekan = $state(0);
  let hariAktif = $state<number | null>(null);

  const jamLabels = Array.from(
    { length: JAM_SELESAI - JAM_MULAI },
    (_, i) => JAM_MULAI + i
  );
  const tinggiGrid = (JAM_SELESAI - JAM_MULAI) * TINGGI_BARIS;

  const sesi = $derived(sesiPerPekan(pekan));
  const tanggal = $derived(pekanList[pekan].tanggal);
  const perHari = $derived(
    HARI.map((_, i) => sesi.filter((s) => s.hari === i + 1))
  );
  const jumlahHari = $derived(perHari.filter((d) => d.length > 0).length);

  const posisi = (mulai: string) =>
    ((menit(mulai) - JAM_MULAI * 60) / 60) * TINGGI_BARIS;
  const tinggi = (mulai: string, selesai: string) =>
    ((menit(selesai) - menit(mulai)) / 60) * TINGGI_BARIS - 4;

  function pilihHari(hari: number) {
    hariAktif = hariAktif === hari ? null : hari;
  }
</script>

<PortalPage
  eyebrow="Jadwal"
  docTitle="Jadwal praktikum — Algohub"
  title="Kalender pekan praktikum."
  lead="Sesi praktikum per hari dan per jam. Pilih hari untuk menyorot kolomnya, atau ganti pekan dengan tombol panah."
>
  <div class="kalender">
    <div class="toolbar">
      <div class="pekan-nav">
        <button
          class="nav-btn"
          type="button"
          aria-label="Pekan sebelumnya"
          disabled={pekan === 0}
          onclick={() => (pekan = Math.max(0, pekan - 1))}>‹</button
        >
        <p class="pekan-label">
          <strong>{pekanList[pekan].label}</strong>
          <span>Pekan {pekan + 1} dari {pekanList.length}</span>
        </p>
        <button
          class="nav-btn"
          type="button"
          aria-label="Pekan berikutnya"
          disabled={pekan === pekanList.length - 1}
          onclick={() => (pekan = Math.min(pekanList.length - 1, pekan + 1))}
          >›</button
        >
      </div>
      <p class="ringkas">
        {sesi.length} sesi · {jumlahHari} hari aktif · {JAM_MULAI}.00–{JAM_SELESAI}.00
      </p>
    </div>

    <!-- Desktop: week grid -->
    <div class="grid-wrap" style="--row:{TINGGI_BARIS}px">
      <div class="grid-head">
        <div class="gutter-head" aria-hidden="true"></div>
        {#each HARI as nama, i (nama)}
          <button
            class="day-head"
            class:picked={hariAktif === i + 1}
            type="button"
            aria-pressed={hariAktif === i + 1}
            onclick={() => pilihHari(i + 1)}
          >
            <span class="day-name">{nama}</span>
            <span class="day-date">{tanggal[i]}</span>
          </button>
        {/each}
      </div>

      <div class="grid-body" style="height:{tinggiGrid}px">
        <div class="gutter">
          {#each jamLabels as jam (jam)}
            <span class="jam">{String(jam).padStart(2, '0')}:00</span>
          {/each}
        </div>

        {#each HARI as nama, i (nama)}
          <div
            class="day-col"
            class:picked={hariAktif === i + 1}
            class:dimmed={hariAktif !== null && hariAktif !== i + 1}
          >
            {#each perHari[i] as item (item.id)}
              <article
                class="event"
                style="top:{posisi(item.mulai)}px;height:{tinggi(
                  item.mulai,
                  item.selesai
                )}px"
                title={`${item.kelas} (${item.kode}) · ${item.ruang} · ${item.asisten}`}
              >
                <span class="event-bar" aria-hidden="true"></span>
                <h3 class="event-title">{item.kelas}</h3>
                <p class="event-meta">{item.kode} · {item.ruang}</p>
              </article>
            {/each}
          </div>
        {/each}
      </div>
    </div>

    <!-- Mobile: day chips + list, same box style -->
    <div class="list-wrap">
      <div class="hari-chips" role="group" aria-label="Pilih hari">
        {#each HARI as nama, i (nama)}
          <button
            class="chip"
            class:picked={hariAktif === i + 1}
            type="button"
            aria-pressed={hariAktif === i + 1}
            onclick={() => pilihHari(i + 1)}
          >
            {nama.slice(0, 3)}
            <span>{tanggal[i]}</span>
          </button>
        {/each}
      </div>

      {#each HARI as nama, i (nama)}
        {#if perHari[i].length > 0 && (hariAktif === null || hariAktif === i + 1)}
          <section class="hari-blok" aria-label={nama}>
            <h3>{nama}, {tanggal[i]}</h3>
            {#each perHari[i] as item (item.id)}
              <article class="event event-list">
                <span class="event-bar" aria-hidden="true"></span>
                <div>
                  <h4 class="event-title">{item.kelas}</h4>
                  <p class="event-meta">
                    {item.mulai}–{item.selesai} · {item.ruang}
                  </p>
                  <p class="event-meta">{item.kode} · {item.modul}</p>
                </div>
              </article>
            {/each}
          </section>
        {/if}
      {/each}
    </div>

    <ul class="legend">
      <li><span class="dot" aria-hidden="true"></span> Sesi praktikum</li>
      <li>Klik nama hari untuk menyorot kolom</li>
      <li>Jadwal personal ada di aplikasi Praktikum setelah masuk</li>
    </ul>
  </div>
</PortalPage>

<style>
  .kalender {
    display: grid;
    gap: 1.25rem;
  }

  .toolbar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem 1.5rem;
  }

  .pekan-nav {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .nav-btn {
    display: grid;
    width: 44px;
    height: 44px;
    place-items: center;
    border: 1px solid var(--ah-line-strong);
    border-radius: 0.75rem;
    color: var(--ah-ink);
    background: var(--ah-cream);
    font-size: 1.1rem;
    line-height: 1;
    cursor: pointer;
  }

  .nav-btn:hover:not(:disabled),
  .nav-btn:focus-visible {
    background: var(--ah-surface);
  }

  .nav-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .pekan-label {
    display: grid;
    gap: 0.1rem;
    margin: 0;
    min-width: 11.5rem;
  }

  .pekan-label strong {
    color: var(--ah-ink);
    font-size: 0.95rem;
    font-weight: 600;
    letter-spacing: -0.01em;
  }

  .pekan-label span,
  .ringkas {
    color: var(--ah-muted);
    font-size: 0.78rem;
  }

  .ringkas {
    margin: 0;
  }

  /* ---------- week grid (box style kept as reference) ---------- */
  .grid-wrap {
    border-top: 1px solid var(--ah-line);
    border-bottom: 1px solid var(--ah-line);
  }

  .grid-head,
  .grid-body {
    display: grid;
    grid-template-columns: 4.75rem repeat(7, minmax(0, 1fr));
  }

  .grid-head {
    border-bottom: 1px solid var(--ah-line);
  }

  .gutter-head {
    height: 3.4rem;
  }

  .day-head {
    display: grid;
    gap: 0.1rem;
    height: 3.4rem;
    padding: 0.4rem 0;
    border: 0;
    border-left: 1px solid var(--ah-line);
    color: var(--ah-ink);
    background: transparent;
    font-family: inherit;
    cursor: pointer;
  }

  .day-head:hover {
    background: color-mix(in srgb, var(--ah-coral) 5%, transparent);
  }

  .day-head.picked {
    background: var(--ah-surface);
  }

  .day-name {
    align-self: end;
    font-size: 0.875rem;
    font-weight: 600;
    text-align: center;
  }

  .day-date {
    align-self: start;
    color: var(--ah-muted);
    font-size: 0.72rem;
    text-align: center;
  }

  .grid-body {
    position: relative;
  }

  .gutter {
    display: grid;
    grid-auto-rows: var(--row);
    padding-top: 0;
  }

  .jam {
    position: relative;
    top: -0.45rem;
    padding-right: 0.6rem;
    color: var(--ah-muted);
    font-size: 0.72rem;
    text-align: right;
  }

  .day-col {
    position: relative;
    border-left: 1px solid var(--ah-line);
    background-image: repeating-linear-gradient(
      to bottom,
      var(--ah-line) 0 1px,
      transparent 1px var(--row)
    );
    transition: background-color var(--ah-motion-fast) ease;
  }

  .day-col.picked {
    background-color: color-mix(in srgb, var(--ah-coral) 6%, transparent);
  }

  .day-col.dimmed .event {
    opacity: 0.45;
  }

  /* the box: rounded, no border, no shadow, thick left bar */
  .event {
    position: absolute;
    left: 0.3rem;
    right: 0.3rem;
    display: grid;
    grid-template-columns: 4px minmax(0, 1fr);
    gap: 0.5rem;
    align-content: center;
    padding: 0.35rem 0.5rem 0.35rem 0;
    border-radius: 0.7rem;
    background: var(--ah-surface);
    overflow: hidden;
  }

  .event-bar {
    width: 4px;
    height: 100%;
    border-radius: 999px;
    background: var(--ah-coral);
  }

  .event-title {
    margin: 0;
    color: var(--ah-ink);
    font-size: 0.78rem;
    font-weight: 600;
    line-height: 1.25;
    letter-spacing: -0.01em;
  }

  .event-meta {
    margin: 0.1rem 0 0;
    color: var(--ah-muted);
    font-size: 0.7rem;
    line-height: 1.3;
  }

  /* ---------- mobile list ---------- */
  .list-wrap {
    display: none;
    gap: 1rem;
  }

  .hari-chips {
    display: flex;
    gap: 0.4rem;
    overflow-x: auto;
    padding-bottom: 0.25rem;
    scrollbar-width: thin;
  }

  .chip {
    display: grid;
    min-width: 3.4rem;
    padding: 0.45rem 0.6rem;
    border: 1px solid var(--ah-line-strong);
    border-radius: 0.7rem;
    color: var(--ah-ink);
    background: var(--ah-cream);
    font-family: inherit;
    font-size: 0.8rem;
    font-weight: 600;
    cursor: pointer;
  }

  .chip span {
    color: var(--ah-muted);
    font-size: 0.7rem;
    font-weight: 500;
  }

  .chip.picked {
    border-color: var(--ah-coral);
    background: var(--ah-surface);
  }

  .hari-blok {
    display: grid;
    gap: 0.45rem;
  }

  .hari-blok h3 {
    margin: 0;
    color: var(--ah-muted);
    font-size: 0.75rem;
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }

  .event-list {
    position: static;
    grid-template-columns: 4px minmax(0, 1fr);
    gap: 0.6rem;
    padding: 0.65rem 0.75rem 0.65rem 0;
  }

  .event-list .event-bar {
    height: auto;
  }

  .event-list .event-title {
    font-size: 0.9rem;
  }

  .legend {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem 1.5rem;
    margin: 0;
    padding: 0;
    color: var(--ah-muted);
    font-size: 0.78rem;
    list-style: none;
  }

  .legend li {
    display: flex;
    align-items: center;
    gap: 0.45rem;
  }

  .dot {
    width: 4px;
    height: 1rem;
    border-radius: 999px;
    background: var(--ah-coral);
  }

  .nav-btn:focus-visible,
  .day-head:focus-visible,
  .chip:focus-visible {
    outline: var(--ah-focus-brand);
    outline-offset: -2px;
  }

  @media (max-width: 60rem) {
    .grid-wrap {
      display: none;
    }

    .list-wrap {
      display: grid;
    }
  }
</style>
