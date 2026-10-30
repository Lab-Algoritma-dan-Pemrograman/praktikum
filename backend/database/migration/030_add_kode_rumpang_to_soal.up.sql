-- 030: Kolom kode_rumpang pada soal coding keterampilan (kelas karyawan).
--
-- Kode rumpang = kerangka kode dengan blank __ / ____ yang tampil di editor
-- mahasiswa sebagai jawaban awal (prefill), tanpa komentar — mahasiswa
-- mengandalkan instruksi di teks_soal. NULL = soal tanpa rumpang (editor kosongan).
-- Kolom ini tidak dikirim ke AI grading sebagai kunci, kunci tetap di kunci_jawaban.
ALTER TABLE soal ADD COLUMN IF NOT EXISTS kode_rumpang TEXT NULL;
