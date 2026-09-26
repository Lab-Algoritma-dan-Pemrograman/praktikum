package usecase

import (
	"crypto/rand"
	"encoding/base64"
	"log"

	"lab-ap/config"
	"lab-ap/internal/dto"
	"lab-ap/internal/entity"
	"lab-ap/internal/repository"
	"lab-ap/pkg/hash"
	"lab-ap/pkg/supabase"
)

type ProfileUsecase struct {
	users repository.UserRepository
	cfg   *config.Config
}

func NewProfileUsecase(u repository.UserRepository, cfg *config.Config) *ProfileUsecase {
	return &ProfileUsecase{users: u, cfg: cfg}
}

func (uc *ProfileUsecase) Get(userID int) (*entity.User, error) {
	u, err := uc.users.FindByID(userID)
	if err != nil {
		return nil, ErrNotFound
	}
	return u, nil
}

// Update memperbarui profil (nama, kontak, foto, medsos), email, dan opsional ganti password.
func (uc *ProfileUsecase) Update(userID int, req dto.UpdateProfileRequest) (*entity.User, error) {
	u, err := uc.users.FindByID(userID)
	if err != nil {
		return nil, ErrNotFound
	}
	if req.Nama != nil {
		u.Nama = *req.Nama
	}
	if req.NomorHP != nil {
		u.NomorHP = req.NomorHP
	}
	if req.MedsosLink != nil {
		u.MedsosLink = req.MedsosLink
	}
	if req.FotoURL != nil {
		u.FotoURL = req.FotoURL
	}
	if req.PasswordBaru != nil && *req.PasswordBaru != "" {
		// Verifikasi password lama jika user sudah punya hash.
		if u.PasswordHash != nil {
			if req.PasswordLama == nil || !hash.Verify(*u.PasswordHash, *req.PasswordLama) {
				return nil, ErrUnauthorized
			}
		}
		hashed, err := hash.Password(*req.PasswordBaru)
		if err != nil {
			return nil, err
		}
		u.PasswordHash = &hashed
	}
	emailBaru, err := uc.siapkanEmail(u, req.Email)
	if err != nil {
		return nil, err
	}
	if err := uc.users.Update(u); err != nil {
		return nil, err
	}
	// Sinkron ke Supabase setelah baris DB tersimpan: kalau langkah ini gagal,
	// email lokal sudah benar (kode recovery tetap cocok) — cukup dicatat.
	if emailBaru != "" && u.SupabaseUserID != nil {
		if admin := uc.admin(); admin != nil {
			if err := admin.UpdateAuthUserEmail(*u.SupabaseUserID, emailBaru); err != nil {
				log.Printf("WARN sync email Supabase user %d gagal: %v", u.ID, err)
			}
		}
	}
	// Akun lama (register sebelum Supabase aktif, ~302 mahasiswa) tidak punya akun
	// auth — tanpa itu ForgotPassword diam-diam tidak kirim OTP. Buatkan sekarang
	// supaya email yang baru diisi benar-benar berfungsi sebagai kanal pemulihan.
	// Password acak: login tetap lewat hash bcrypt lokal, Supabase cuma kanal OTP.
	if emailBaru != "" && u.SupabaseUserID == nil {
		if admin := uc.admin(); admin != nil {
			if uid, err := admin.CreateAuthUser(emailBaru, randToken(), true); err != nil {
				log.Printf("WARN buat akun auth Supabase user %d gagal: %v", u.ID, err)
			} else {
				u.SupabaseUserID = &uid
				if err := uc.users.Update(u); err != nil {
					log.Printf("WARN simpan supabase_user_id user %d gagal: %v", u.ID, err)
				}
			}
		}
	}
	return u, nil
}

// siapkanEmail memvalidasi & menetapkan email baru pada struct user; kembalikan
// email yang perlu disinkronkan ke Supabase ("" kalau tidak berubah).
// Email adalah kanal OTP lupa-password, jadi wajib unik antar pengguna.
func (uc *ProfileUsecase) siapkanEmail(u *entity.User, req *string) (string, error) {
	if req == nil {
		return "", nil
	}
	norm, err := validateEmailBaru(*req, uc.cfg)
	if err != nil {
		return "", err
	}
	if u.Email != nil && *u.Email == norm {
		return "", nil
	}
	if other, err := uc.users.FindByEmail(norm); err == nil && other.ID != u.ID {
		return "", ErrConflict
	}
	u.Email = &norm
	return norm, nil
}

func (uc *ProfileUsecase) admin() *supabase.Admin {
	if uc.cfg == nil || uc.cfg.SupabaseURL == "" || uc.cfg.SupabaseServiceKey == "" {
		return nil
	}
	return supabase.NewAdmin(uc.cfg.SupabaseURL, uc.cfg.SupabaseServiceKey)
}

// randToken: password acak untuk akun Supabase baru. Login mahasiswa tetap lewat
// hash bcrypt lokal; password ini tidak pernah dipakai siapa pun.
func randToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "sinkron-email-lokal"
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
