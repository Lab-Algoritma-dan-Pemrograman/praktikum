# Kajian source Glot — 11 November 2026

> **Historical/superseded for current Next Steps.** Keputusan final ada pada [browser-completion decision](elearning-browser-completion-decision-2026-11-12.md): Glot preview-only, Judge0/RapidAPI abandoned, dan tidak ada gate authoritative provider yang harus dikejar.

## Kesimpulan dan batas

**Belum ditemukan jalur hosted Glot yang dapat diaktifkan sebagai runner authoritative dengan kontrak proyek saat ini.** Penelusuran source resmi memperjelas letak gap, tetapi tidak menutup gate integrasi, successful execution → proof → completion, atau kapasitas runner. Phase 6 tetap ditahan.

Kajian ini hanya membaca GitHub publik. **Nol request execution/metadata Glot, nol operasi staging, nol deployment**, tanpa membaca credential. Budget historis tetap 18/18 terpakai. Source upstream tidak dibangun atau dijalankan. Tidak ada perubahan kode aplikasi atau pengulangan suite yang sudah lulus.

## Source yang diverifikasi

| Komponen                                                                                                       | Commit yang dibaca                         | Peran                                                  |
| -------------------------------------------------------------------------------------------------------------- | ------------------------------------------ | ------------------------------------------------------ |
| [glot-app](https://github.com/glotcode/glot-app/tree/0b8b25571eceb545c98eba10fc7b734c37bc05a9)                 | `0b8b25571eceb545c98eba10fc7b734c37bc05a9` | Route publik, codec, adapter internal, metadata versi  |
| [docker-run](https://github.com/glotcode/docker-run/tree/f984fba5409c0ecbdf6a64492cbfa1d3666ceb1e)             | `f984fba5409c0ecbdf6a64492cbfa1d3666ceb1e` | HTTP internal, container, stream output, cleanup       |
| [code-runner main](https://github.com/glotcode/code-runner/tree/41befd8ae32c0f813f0bd2b7b054c8f405d0814e)      | `41befd8ae32c0f813f0bd2b7b054c8f405d0814e` | Penulisan file, compile/run, serialisasi hasil         |
| [glot-languages](https://github.com/glotcode/glot-languages/tree/31bfa457b521a95e2b31e64bf9dfcf91c286434b)     | `31bfa457b521a95e2b31e64bf9dfcf91c286434b` | Definisi image C/Python dan pin build                  |
| [code-runner pin image](https://github.com/glotcode/code-runner/tree/3b87378c36bd457624cc15c1c2f0dac32f33e69f) | `3b87378c36bd457624cc15c1c2f0dac32f33e69f` | Revision berbeda yang direferensikan build image resmi |

Nama repo yang terverifikasi adalah **`glotcode/docker-run`**, bukan `docker-runner`. README `docker-run`/`glot-languages` masih memuat contoh payload lama; kontrak di bawah mengikuti source codec pada commit yang dicatat, bukan menyalin contoh README sebagai API terkini.

Salinan source terisolasi, URL, ukuran, dan SHA-256 tersedia pada [manifest bukti lokal](../../../tmp/algohub-glot-source-review-71i5cath/manifest.json). Ini snapshot source, **bukan identifikasi binary/image yang berjalan pada hosted Glot**. Fetch `src/request.rs` pada pin image mendapat 404; tipe request revision itu berada di `src/main.rs`, yang berhasil dibaca. Tidak ada klaim file yang gagal diambil telah diperiksa.

## Jalur resmi yang terbaca

1. [Router publik](https://github.com/glotcode/glot-app/blob/0b8b25571eceb545c98eba10fc7b734c37bc05a9/glot_backend/src/glot_backend/system/application/router.gleam#L28-L47) menerima `POST /api/mux`. [Dispatcher](https://github.com/glotcode/glot-app/blob/0b8b25571eceb545c98eba10fc7b734c37bc05a9/glot_backend/src/glot_backend/api/dispatcher/public.gleam#L51-L61) memisahkan action run dan metadata versi.
2. [Request codec](https://github.com/glotcode/glot-app/blob/0b8b25571eceb545c98eba10fc7b734c37bc05a9/glot_core/src/glot_core/run.gleam#L94-L178) menerima `image`, `payload.runInstructions`, `files`, dan `stdin`, kemudian memvalidasi image terhadap daftar bahasa.
3. [Adapter internal](https://github.com/glotcode/glot-app/blob/0b8b25571eceb545c98eba10fc7b734c37bc05a9/glot_backend/src/glot_backend/run_code/adapter/docker_run/client.gleam#L20-L34) meneruskan ke `docker-run` melalui `POST /run` dengan `X-Access-Token` milik konfigurasi layanan. Ini bukan endpoint publik alternatif yang otomatis boleh dipanggil atau memakai token legacy.
4. [docker-run](https://github.com/glotcode/docker-run/blob/f984fba5409c0ecbdf6a64492cbfa1d3666ceb1e/src/docker_run/run.rs#L171-L215) mengirim payload ke stdin container, membaca stream, lalu mendekode stdout sebagai objek JSON. Jalur ini tidak menambahkan status proses mahasiswa dari Docker wait/inspect.
5. [Codec hasil](https://github.com/glotcode/glot-app/blob/0b8b25571eceb545c98eba10fc7b734c37bc05a9/glot_core/src/glot_core/run.gleam#L181-L213) hanya mendekode/mengenkode `duration`, `stdout`, `stderr`, `error`, atau bentuk kegagalan `message`. [Presenter](https://github.com/glotcode/glot-app/blob/0b8b25571eceb545c98eba10fc7b734c37bc05a9/glot_backend/src/glot_backend/api/presenter/response.gleam#L93-L94) memakai codec tersebut.

**Implikasi:** menambahkan field `exit_code`/fase di `code-runner` saja belum membuat field tersebut tersedia melalui `/api/mux`; codec/presenter dan rollout image layanan juga harus mendukungnya. Nama tipe upstream `SuccessfulRun` tidak berarti program mahasiswa exit 0: field `error` masih berada di dalam tipe itu.

## Gap kontrak dan trust boundary

### 1. Status proses dan fase hilang sebelum mencapai aplikasi

Pada [code-runner main](https://github.com/glotcode/code-runner/blob/41befd8ae32c0f813f0bd2b7b054c8f405d0814e/src/main.rs#L78-L118):

- Success menghasilkan `error` kosong tanpa numeric exit status.
- Failure dengan `exit_code: None` juga menghasilkan `error` kosong.
- Compile failure dan runtime failure memakai konversi hasil yang sama; tidak ada discriminator fase.
- `duration` memakai nanodetik command yang dilaporkan, bukan otomatis total compile + run.

[Revision yang dipin image](https://github.com/glotcode/code-runner/blob/3b87378c36bd457624cc15c1c2f0dac32f33e69f/src/main.rs#L103-L143) mempunyai kehilangan informasi status yang sama. Ini konsisten dengan [probe SIGTERM terdahulu](staging-acceptance-2026-11-11.md#glot--hasil-delapan-probe-tambahan), bukan probe baru pada sesi ini.

Regex pesan error, stdout yang sesuai testcase, HTTP 200, atau nama tipe `SuccessfulRun` tidak dapat mengembalikan status proses yang telah hilang. Mapping kasus ambigu ke `provider_error` aman sebagai penolakan, tetapi tidak menyediakan jalur sukses authoritative yang dibutuhkan untuk completion.

### 2. Wrapper di command bukan otomatis authority terisolasi

Custom `buildCommands`/`runCommand` memang didukung. Namun [peluncuran child pada main](https://github.com/glotcode/code-runner/blob/41befd8ae32c0f813f0bd2b7b054c8f405d0814e/src/cmd.rs#L87-L120) dan [pin image](https://github.com/glotcode/code-runner/blob/3b87378c36bd457624cc15c1c2f0dac32f33e69f/src/cmd.rs#L82-L103) hanya menjalankan `sh -c` dengan pipe. Jalur yang dibaca tidak membentuk privilege boundary terpisah antara supervisor tambahan dan program mahasiswa.

Karena itu marker/sentinel/JSON dari wrapper pada lingkungan tersebut belum menjadi bukti terpercaya. Diperlukan boundary yang dibuktikan melindungi pelapor status, saluran hasil, dan metadata runtime dari program mahasiswa. Kajian ini tidak menjalankan exploit maupun menyatakan hosted service berhasil ditembus.

### 3. Pin build bukan pin eksekusi hosted

[Build image](https://github.com/glotcode/glot-languages/blob/31bfa457b521a95e2b31e64bf9dfcf91c286434b/images/common/coderunner.nix#L3-L12) masih mereferensikan `code-runner` **`3b87378`**, bukan head **`41befd8`**. [Nixpkgs](https://github.com/glotcode/glot-languages/blob/31bfa457b521a95e2b31e64bf9dfcf91c286434b/images/common/nixpkgs.nix#L1-L12) juga dipin. Kedua fakta ini tidak membuktikan image hosted sudah dibangun dari revision tertentu.

[Validasi image](https://github.com/glotcode/glot-app/blob/0b8b25571eceb545c98eba10fc7b734c37bc05a9/glot_core/src/glot_core/language.gleam#L235-L237) mencocokkan string secara exact; [daftar image](https://github.com/glotcode/glot-app/blob/0b8b25571eceb545c98eba10fc7b734c37bc05a9/glot_core/src/glot_core/language.gleam#L303-L350) memakai `glot/clang:latest` dan `glot/python:latest`. Mengirim digest/version tag lain tidak lolos jalur validasi itu.

[Metadata versi](https://github.com/glotcode/glot-app/blob/0b8b25571eceb545c98eba10fc7b734c37bc05a9/glot_backend/src/glot_backend/run_code/effect/interpreter.gleam#L37-L54) berasal dari cache atau request terpisah. Maka Clang 17.0.6/Python 3.11.9 yang pernah teramati belum membuktikan runtime untuk setiap eksekusi. Jalur yang diterima harus menyediakan identitas runtime terpercaya yang terikat pada run yang sama; pin immutable merupakan salah satu cara, bukan mengarang versi dari metadata cache.

### 4. Batas resource dan cleanup tersedia sebagai mekanisme, bukan bukti hosted

| Aspek                                      | Observasi source                                                                                                                                                                                                                                                                                                                                                     | Yang belum terbukti                                                                                       |
| ------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| Network, memory, capabilities, proses/file | [Container config](https://github.com/glotcode/docker-run/blob/f984fba5409c0ecbdf6a64492cbfa1d3666ceb1e/src/docker_run/run.rs#L250-L287) meneruskan konfigurasi network/memory, cap add/drop, nofile/nproc, read-only rootfs/tmpfs; `privileged: false`                                                                                                              | Nilai hosted, isolasi pelapor hasil, serta kesesuaian budget proyek                                       |
| Timeout                                    | [Run limits](https://github.com/glotcode/docker-run/blob/f984fba5409c0ecbdf6a64492cbfa1d3666ceb1e/src/docker_run/run.rs#L116-L169) memasang `max_execution_time` sebagai read timeout stream                                                                                                                                                                         | Enforcement deadline total compile + run serta penghentian proses di provider saat request API dibatalkan |
| Output                                     | [Reader](https://github.com/glotcode/docker-run/blob/f984fba5409c0ecbdf6a64492cbfa1d3666ceb1e/src/docker_run/docker.rs#L329-L365) membatasi byte stream container; child output sebelumnya ditampung oleh `wait_with_output`                                                                                                                                         | Cap stdout/stderr pada saat produksi output, nilai hosted, dan indikator truncation terpercaya            |
| Cleanup                                    | [Lifecycle](https://github.com/glotcode/docker-run/blob/f984fba5409c0ecbdf6a64492cbfa1d3666ceb1e/src/docker_run/run.rs#L64-L130) menjadwalkan penghapusan container; lease prewarm single-use. [README](https://github.com/glotcode/docker-run/blob/f984fba5409c0ecbdf6a64492cbfa1d3666ceb1e/README.md#L3-L27) menjelaskan retry/recovery dan `DEBUG_KEEP_CONTAINER` | Konfigurasi deployed dan keberhasilan cleanup hosted di seluruh jalur failure                             |
| Kapasitas                                  | [Default pool aplikasi](https://github.com/glotcode/glot-app/blob/0b8b25571eceb545c98eba10fc7b734c37bc05a9/glot_backend/src/glot_backend/app_config/model/defaults.gleam#L14-L19) bernilai 16 sesi HTTP                                                                                                                                                              | Concurrency/kuota/throughput hosted yang disepakati; angka default bukan hasil load test                  |

Batas response aplikasi 16 KiB per stream/128 KiB JSON tetap penting, tetapi tidak menggantikan enforcement resource di sandbox. Bukti satu marker hilang dan satu koneksi tidak berhasil pada probe lama tetap terbatas pada skenario tersebut.

## Konsekuensi untuk implementasi AlgoHub

Tidak ada alasan untuk melonggarkan kode yang sudah ada:

- [Composition root](../backend/internal/application/app.go#L39-L45) tetap `runner.Unconfigured` di luar fixture lokal.
- [Validasi hasil](../backend/internal/platform/runner/validation.go#L52-L61) mewajibkan numeric exit 0 untuk status sukses.
- [Proof completion](../backend/internal/usecase/learning_admin_runner.go#L73-L82) tetap mengikat owner/lesson/source/definisi dan menolak hasil truncated/simulated yang tidak diizinkan.
- [Eksekusi learning](../backend/internal/usecase/learning_admin_runner.go#L99-L132) tetap menolak runtime version yang hilang/tidak sama dan menggunakan deadline yang sudah ada.

Tidak ditambahkan adapter placeholder baru, regex status, fallback provider, schema/migration, framework preflight, atau perubahan browser preview. Pemeriksaan lokal hanya membaca boundary tersebut dan artefak [verifikasi Go final](../../../tmp/apv2-pg-race-go12611-20261111T070516Z-ae6fe6ba/logs/final-verification.json); angka test terdahulu tidak diberi label sebagai tes sesi ini.

## Keputusan yang diperlukan sebelum melanjutkan

**Rekomendasi: dapatkan konfirmasi/jalur resmi dari pengelola hosted Glot sebelum meminta budget probe baru.** Tambahan request pada kontrak yang sama tidak memulihkan informasi status yang sudah hilang.

Informasi minimum yang diperlukan:

1. Apakah tersedia endpoint/versi layanan yang mengembalikan fase compile/run, numeric exit atau signal/termination reason, serta runtime identity untuk run yang sama melalui saluran hasil yang terlindungi dari program mahasiswa?
2. Jika belum, apakah pengelola bersedia menyediakan perubahan `code-runner` **beserta image pin/build dan codec `glot-app`**? Patch source lokal saja bukan rollout hosted.
3. Apa konfigurasi dan jaminan hosted untuk network, memory/process/file, deadline compile/run/total, output/truncation, serta cleanup? Source/default bukan jawaban konfigurasi deployed.
4. Apakah automated server-to-server use untuk baseline 30 pengguna didukung; berapa concurrency, rate limit, kuota, dan budget yang disetujui untuk acceptance?

Belum ada komunikasi keluar, issue/PR, atau usulan perubahan upstream yang dikirim. Endpoint internal, token, server baru, dan self-hosting tidak diasumsikan tersedia/diizinkan. Jika jalur resmi belum tersedia, runner tetap ditahan; bukan memindahkan aplikasi dari arsitektur serverless atau mengganti Glot sepihak.

Setelah ada pendekatan sah, urutan minimum adalah adapter pada interface `Runner` yang sudah ada → fake-provider tests lokal (success, compile/runtime failure, signal, timeout, malformed/missing status/version, output limit, cancellation, tanpa retry ambigu) → izin probe dengan target/jumlah/payload/deadline konkret → acceptance provider dan proof/completion exactly-once → kapasitas runner yang terpisah. Jangan mengulang provisioning, acceptance non-runner, atau seluruh gate lama hanya karena runner belum tersedia.
