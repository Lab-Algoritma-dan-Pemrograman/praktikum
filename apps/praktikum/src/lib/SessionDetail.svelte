<script lang="ts">
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import {
    PracticumClient,
    reusableCommand,
    errorMessage,
    courseLabels,
    jakarta,
    type Attempt,
    type Command,
    type SessionDetail
  } from '@algohub/practicum-client';
  export let id: string;
  let detail: SessionDetail | null = null;
  let loading = true;
  let issue = '';
  let starting = '';
  let tokens: Record<string, string> = {};
  let pending: Command | null = null;
  const client = new PracticumClient();
  async function load() {
    if (starting) return;
    loading = true;
    issue = '';
    try {
      detail = await client.detail(id);
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      loading = false;
    }
  }
  async function start(activationId: string) {
    if (starting) return;
    starting = activationId;
    issue = '';
    pending = reusableCommand(pending, '/practicum/attempts', 'POST', {
      activation_id: activationId,
      ...(tokens[activationId] ? { access_token: tokens[activationId] } : {})
    });
    try {
      const attempt = await client.execute<Attempt>(pending);
      tokens = {};
      pending = null;
      await goto(`/attempts/${encodeURIComponent(attempt.attempt_id)}`);
    } catch (error) {
      issue = errorMessage(error);
    } finally {
      starting = '';
    }
  }
  onMount(() => {
    void load();
  });
</script>

<svelte:head
  ><title
    >{detail
      ? `${detail.session.title} — Praktikum`
      : 'Detail sesi — Praktikum'}</title
  ></svelte:head
>
<a class="back-link" href="/">← Kembali ke sesi praktikum</a>
<header class="page-heading">
  <p class="eyebrow">DETAIL SESI</p>
  <h1>{detail?.session.title ?? 'Detail sesi praktikum'}</h1>
  <p class="muted">
    Pilih kegiatan yang dibuka pengajar. Melanjutkan pengerjaan tidak mengulang
    waktu; satu pengerjaan berlaku untuk setiap aktivasi.
  </p>
</header>
{#if loading}<p role="status" class="muted">Memuat detail sesi…</p>{/if}
{#if issue}<div class="notice error" role="alert">
    {issue}
    <div class="actions">
      <button class="secondary" onclick={load} disabled={loading || !!starting}
        >Muat ulang detail</button
      ><a href={`/login?${new URLSearchParams({ return_to: `/sesi/${id}` })}`}
        >Masuk kembali</a
      >
    </div>
  </div>{/if}
{#if detail}
  {#if !detail.session.active}<p class="notice">
      Sesi belum aktif. Anda dapat melihat informasi kegiatan, tetapi belum
      dapat memulai pengerjaan baru.
    </p>{/if}
  <div class="stack" aria-busy={loading || !!starting}>
    {#each detail.courses as course (course.id)}
      <section class="course-block" aria-labelledby={`course-${course.id}`}>
        <div class="section-heading">
          <div>
            <p class="eyebrow">
              {courseLabels[course.kind] ?? 'Kegiatan praktikum'}
            </p>
            <h2 id={`course-${course.id}`}>{course.title}</h2>
          </div>
          <span class="badge">{course.duration_minutes} menit</span>
        </div>
        <div class="activation-grid">
          {#each detail.activations.filter((a) => a.course_id === course.id) as activation (activation.id)}
            <form
              class="panel stack activation-card"
              data-activation-id={activation.id}
              onsubmit={(event) => {
                event.preventDefault();
                void start(activation.id);
              }}
            >
              <div class="card-meta">
                <strong
                  >{activation.scope.class_id
                    ? `Kelas ${activation.scope.class_id}`
                    : 'Kelas praktikum'}</strong
                ><span class="badge"
                  >{activation.is_open ? 'Terbuka' : 'Ditutup'}</span
                >
              </div>
              <p class="muted">
                {activation.scope.shift
                  ? `Shift ${activation.scope.shift} · `
                  : ''}{activation.question_count} soal
              </p>
              <dl class="session-facts">
                <div>
                  <dt>Waktu pengerjaan</dt>
                  <dd>
                    {activation.timer_mode === 'shared'
                      ? 'Bersama · dimulai peserta pertama'
                      : 'Individual · dimulai saat Anda masuk'}
                  </dd>
                </div>
                <div>
                  <dt>Batas penutupan (WIB)</dt>
                  <dd>
                    {activation.closes_at
                      ? jakarta(activation.closes_at)
                      : 'Ditentukan pengajar'}
                  </dd>
                </div>
                {#if activation.deadline_at}<div>
                    <dt>Batas pengerjaan (WIB)</dt>
                    <dd>{jakarta(activation.deadline_at)}</dd>
                  </div>{/if}
              </dl>
              {#if course.kind === 'ujian_praktik'}<p class="notice">
                  Ujian praktik memerlukan persetujuan masuk dari pengajar.
                </p>{/if}
              {#if course.token_required && activation.is_open}<label
                  >Token akses<input
                    type="password"
                    autocomplete="off"
                    maxlength="128"
                    bind:value={tokens[activation.id]}
                    disabled={!!starting}
                    aria-describedby={`token-help-${activation.id}`}
                  /></label
                >
                <p class="muted field-help" id={`token-help-${activation.id}`}>
                  Masukkan token yang dibagikan pengajar. Jangan bagikan token
                  ke luar kelas.
                </p>{/if}
              <button
                disabled={!activation.is_open ||
                  !detail.session.active ||
                  !!starting ||
                  loading}
                >{starting === activation.id
                  ? 'Membuka…'
                  : 'Mulai / lanjutkan pengerjaan'}</button
              >
              {#if !activation.is_open}<p class="muted">
                  Aktivasi ditutup. Jawaban yang sudah dikumpulkan dapat dilihat
                  di <a href="/nilai">Nilai & riwayat</a>.
                </p>{/if}
            </form>
          {:else}<div class="panel empty-state">
              <h3>Belum ada kegiatan dibuka untuk Anda</h3>
              <p>
                Pengajar perlu membuka aktivasi untuk kelas dan shift yang
                terdaftar.
              </p>
            </div>{/each}
        </div>
      </section>
    {:else}<section class="panel empty-state">
        <h2>Kegiatan belum tersedia</h2>
        <p>Pengajar belum menambahkan kegiatan untuk sesi ini.</p>
      </section>{/each}
  </div>
{/if}
