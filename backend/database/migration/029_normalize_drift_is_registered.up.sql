-- 029: Normalisasi baris drift — is_registered=true tapi TIDAK punya kredensial apa pun
-- (password_hash, fb_password_hash, supabase_user_id semuanya NULL).
--
-- Baris begini bikin mahasiswa mentok: cek-nim menjawab "akun belum terdaftar",
-- tapi register ditolak "konflik data" karena flag-nya true, dan login ditolak 401.
-- Setelah ini gate auth memakai punyaKredensial() (lihat auth_usecase.go), bukan flag;
-- normalisasi ini cuma membersihkan data lama supaya tampilan admin konsisten.
UPDATE users
SET is_registered = false
WHERE is_registered = true
  AND password_hash IS NULL
  AND fb_password_hash IS NULL
  AND supabase_user_id IS NULL;
