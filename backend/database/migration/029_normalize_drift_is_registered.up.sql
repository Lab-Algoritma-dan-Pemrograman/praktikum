-- 029: Normalisasi baris drift antara flag is_registered dan kredensial nyata.
--
-- Definisi kredensial: password_hash ATAU fb_password_hash ATAU supabase_user_id terisi.
-- Kedua arah drift selalu bug, bukan state yang sah:
--   (a) flag=true tanpa kredensial -> cek-nim bilang "belum terdaftar", tapi register
--       ditolak 409 dan login 401 — mahasiswa mentok di ketiganya.
--       Pemicu: admin Reset Password (flag->false) lalu simpan-edit data mahasiswa,
--       struct hasil FindByID sebelum reset menulis flag lama (true) kembali.
--   (b) flag=false padahal kredensial ada -> cek-nim menyuruh "buat password" padahal
--       akun sudah bisa login; register ikut menimpa hash yang sedang dipakai.
--
-- Setelah 029, gate auth memakai punyaKredensial() (lihat auth_usecase.go) sehingga
-- flag tidak lagi menentukan alur; normalisasi ini menyelaraskan data & tampilan admin.

-- (a) flag true tanpa kredensial -> belum daftar
UPDATE users
SET is_registered = false
WHERE is_registered = true
  AND password_hash IS NULL
  AND fb_password_hash IS NULL
  AND supabase_user_id IS NULL;

-- (b) flag false padahal kredensial ada -> sudah terdaftar
UPDATE users
SET is_registered = true
WHERE is_registered = false
  AND (
    password_hash IS NOT NULL
    OR fb_password_hash IS NOT NULL
    OR supabase_user_id IS NOT NULL
  );
