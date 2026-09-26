package usecase

import (
	"errors"
	"log"
	"strings"
	"time"

	"lab-ap/config"
	"lab-ap/internal/dto"
	"lab-ap/internal/entity"
	"lab-ap/internal/repository"
	"lab-ap/pkg/hash"
	"lab-ap/pkg/jwt"
	"lab-ap/pkg/supabase"

	"gorm.io/gorm"
)

type AuthUsecase struct {
	users    repository.UserRepository
	kelas    repository.KelasRepository
	jwt      *jwt.Manager
	cfg      *config.Config
	fbScrypt hash.FbScryptConfig
}

func NewAuthUsecase(u repository.UserRepository, k repository.KelasRepository, j *jwt.Manager, cfg *config.Config, fb hash.FbScryptConfig) *AuthUsecase {
	return &AuthUsecase{users: u, kelas: k, jwt: j, cfg: cfg, fbScrypt: fb}
}

// punyaKredensial: baris benar-benar bisa login — ada hash lokal, password Firebase lama,
// atau akun Supabase yang sudah diklaim. Ini sumber kebenaran gate roster, BUKAN flag
// is_registered yang bisa drift: baris dengan is_registered=true tapi password_hash NULL
// bikin cek-nim bilang "belum terdaftar" sementara register ditolak "konflik data" —
// mahasiswa mentok di keduanya (login gagal, daftar ditolak).
func punyaKredensial(u *entity.User) bool {
	return u.PasswordHash != nil || u.FbPasswordHash != nil || u.SupabaseUserID != nil
}

// CekNIM menentukan alur first-time login (login / register / ditolak).
func (uc *AuthUsecase) CekNIM(nim string) (*dto.CekNIMResponse, error) {
	u, err := uc.users.FindByNIM(nim)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &dto.CekNIMResponse{NIM: nim, Ditemukan: false, Pesan: "Akun tidak ada."}, nil
		}
		return nil, err
	}
	punya := punyaKredensial(u)
	resp := &dto.CekNIMResponse{
		NIM:          u.NIM,
		Ditemukan:    true,
		IsRegistered: punya,
		Nama:         u.Nama,
	}
	if punya {
		resp.Pesan = "Silakan masukkan password Anda."
		return resp, nil
	}
	// Belum register: roster-gated — NIM ada di roster = boleh buat password.
	resp.Pesan = "Akun belum terdaftar. Silakan buat password."
	return resp, nil
}

// Login via NIM atau Email (identifier). Mahasiswa belum daftar → "Akun tidak ada" (404).
func (uc *AuthUsecase) Login(req dto.LoginRequest) (*dto.AuthResponse, error) {
	ident := strings.TrimSpace(req.Identifier)
	var u *entity.User
	var err error
	if strings.Contains(ident, "@") {
		norm := strings.ToLower(ident)
		u, err = uc.users.FindByEmail(norm)
	} else {
		u, err = uc.users.FindByNIM(ident)
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	// Mahasiswa roster yang belum punya kredensial dianggap akun tidak ada (jangan bocorkan "belum aktivasi")
	if u.Role == entity.RoleMahasiswa && !punyaKredensial(u) {
		return nil, ErrNotFound
	}
	if u.PasswordHash != nil && hash.Verify(*u.PasswordHash, req.Password) {
		return uc.issue(u)
	}
	if u.PasswordHash == nil && uc.verifyAndMigrateFirebase(u, req.Password) {
		return uc.issue(u)
	}
	return nil, ErrUnauthorized
}

func (uc *AuthUsecase) verifyAndMigrateFirebase(u *entity.User, plain string) bool {
	if !uc.fbScrypt.Enabled() || u.FbPasswordHash == nil || u.FbPasswordSalt == nil {
		return false
	}
	ok, err := hash.VerifyFirebaseScrypt(plain, *u.FbPasswordSalt, *u.FbPasswordHash, uc.fbScrypt)
	if err != nil || !ok {
		return false
	}
	if bh, err := hash.Password(plain); err == nil {
		u.PasswordHash = &bh
		u.FbPasswordHash = nil
		u.FbPasswordSalt = nil
		_ = uc.users.Update(u)
	}
	return true
}

// validateEmailBaru: normalisasi + allowlist domain + anti-alias.
// Dipakai bersama oleh Register dan ganti-email dari halaman profil.
func validateEmailBaru(email string, cfg *config.Config) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(email))
	if err := validateEmailAllowlist(norm, cfg); err != nil {
		return "", err
	}
	// Reject '+' alias dan '.' trick gmail
	local, domain, _ := strings.Cut(norm, "@")
	if strings.Contains(local, "+") {
		return "", errors.Join(ErrBadRequest, errors.New("karakter '+' tidak diizinkan pada email"))
	}
	if domain == "gmail.com" && strings.Contains(local, ".") {
		return "", errors.Join(ErrBadRequest, errors.New("karakter '.' tidak diizinkan pada email Gmail"))
	}
	return norm, nil
}

// Register roster-gated: NIM harus ada di roster & belum diklaim, email allowlist, kelas harus dibuka.
func (uc *AuthUsecase) Register(req dto.RegisterRequest) (*dto.AuthResponse, error) {
	emailNorm, err := validateEmailBaru(req.Email, uc.cfg)
	if err != nil {
		return nil, err
	}

	u, err := uc.users.FindByNIM(req.NIM)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	// Klaim roster ditolak hanya kalau kredensial sudah ada — flag is_registered saja
	// tidak cukup (baris drift: flag true tapi password kosong = mahasiswa mentok).
	if punyaKredensial(u) {
		return nil, ErrConflict
	}
	if u.KelasID == nil {
		return nil, errors.Join(ErrBadRequest, errors.New("data kelas belum lengkap, hubungi asisten"))
	}
	// Cek email sudah dipakai (case-insensitive)
	if _, err := uc.users.FindByEmail(emailNorm); err == nil {
		return nil, ErrConflict
	}

	hashed, err := hash.Password(req.Password)
	if err != nil {
		return nil, err
	}

	// Supabase: buat akun auth (email_confirm=true, kita skip verifikasi email; email hanya untuk forgot)
	var supaUID string
	if uc.cfg.SupabaseURL != "" && uc.cfg.SupabaseServiceKey != "" {
		admin := supabase.NewAdmin(uc.cfg.SupabaseURL, uc.cfg.SupabaseServiceKey)
		uid, err := admin.CreateAuthUser(emailNorm, req.Password, true)
		if err != nil {
			if strings.Contains(err.Error(), "sudah terdaftar") {
				return nil, ErrConflict
			}
			return nil, err
		}
		supaUID = uid
	}

	// Klaim roster
	if supaUID != "" {
		if err := uc.users.ClaimRoster(u.ID, supaUID, &emailNorm, hashed); err != nil {
			if errors.Is(err, repository.ErrRosterClaimed) {
				if supaUID != "" {
					_ = supabase.NewAdmin(uc.cfg.SupabaseURL, uc.cfg.SupabaseServiceKey).DeleteAuthUser(supaUID)
				}
				return nil, ErrConflict
			}
			if supaUID != "" {
				if derr := supabase.NewAdmin(uc.cfg.SupabaseURL, uc.cfg.SupabaseServiceKey).DeleteAuthUser(supaUID); derr != nil {
					log.Printf("WARN Register rollback DeleteAuthUser(%s) gagal: %v", supaUID, derr)
				}
			}
			return nil, err
		}
		// Reload user setelah claim
		if fresh, err := uc.users.FindByID(u.ID); err == nil {
			u = fresh
		}
	} else {
		// Fallback lokal tanpa Supabase: simpan hash langsung
		u.PasswordHash = &hashed
		u.Email = &emailNorm
		u.IsRegistered = true
		if err := uc.users.Update(u); err != nil {
			return nil, err
		}
	}
	// Pastikan token issue pakai data terbaru
	return uc.issue(u)
}

// ForgotPassword tetap SELALU sukses dari sisi HTTP (anti-enumeration), tetapi
// melaporkan apakah OTP benar-benar terkirim. Balasan untuk email yang tidak
// terdaftar dan email yang gagal kirim sengaja dibedakan: yang pertama tetap
// terlihat "terkirim" (tidak membocorkan data akun), sedangkan pengguna yang
// memang login — tapi akunnya belum punya email/kanal OTP — diberi tahu apa
// yang harus dilakukan, bukan dibiarkan menunggu kode yang tak akan datang.
func (uc *AuthUsecase) ForgotPassword(email string) dto.ForgotPasswordResponse {
	norm := strings.ToLower(strings.TrimSpace(email))
	u, err := uc.users.FindByEmail(norm)
	if err != nil {
		// Alamat tidak terdaftar: tetap tampilkan seolah terkirim.
		return dto.ForgotPasswordResponse{Sent: true}
	}
	// Akun ada tapi belum punya kanal OTP (email belum diisi / belum terhubung
	// ke Supabase Auth). Ini yang sebelumnya diam-diam tidak mengirim apa pun.
	if u.SupabaseUserID == nil {
		return dto.ForgotPasswordResponse{Sent: false, Reason: "no_email_channel"}
	}
	if uc.cfg.SupabaseURL == "" || uc.cfg.SupabaseServiceKey == "" {
		return dto.ForgotPasswordResponse{Sent: false, Reason: "send_failed"}
	}
	if err := supabase.NewAdmin(uc.cfg.SupabaseURL, uc.cfg.SupabaseServiceKey).SendRecoveryOTP(norm); err != nil {
		log.Printf("WARN ForgotPassword kirim OTP ke %s gagal: %v", norm, err)
		return dto.ForgotPasswordResponse{Sent: false, Reason: "send_failed"}
	}
	return dto.ForgotPasswordResponse{Sent: true}
}

// ResetPassword verifikasi OTP + set password baru (Supabase & lokal).
func (uc *AuthUsecase) ResetPassword(req dto.ResetPasswordViaOTPRequest) error {
	norm := strings.ToLower(strings.TrimSpace(req.Email))
	admin := supabase.NewAdmin(uc.cfg.SupabaseURL, uc.cfg.SupabaseServiceKey)
	if err := admin.VerifyRecoveryOTP(norm, req.Token, req.Password); err != nil {
		return errors.Join(ErrBadRequest, err)
	}
	hashed, err := hash.Password(req.Password)
	if err != nil {
		return err
	}
	if _, err := uc.users.UpdatePasswordByEmail(norm, hashed); err != nil {
		return err
	}
	return nil
}

func validateEmailAllowlist(email string, cfg *config.Config) error {
	_, domain, ok := strings.Cut(email, "@")
	if !ok || domain == "" {
		return errors.Join(ErrBadRequest, errors.New("format email tidak valid"))
	}
	if cfg != nil && cfg.EmailAllowItpln && domain == "itpln.ac.id" {
		return nil
	}
	if entity.AllowedEmailDomains[domain] {
		return nil
	}
	return errors.Join(ErrBadRequest, errors.New("domain email tidak diizinkan (hanya gmail.com, yahoo.com, outlook.com, hotmail.com)"))
}

func (uc *AuthUsecase) Logout(userID int) { _ = userID }

func (uc *AuthUsecase) issue(u *entity.User) (*dto.AuthResponse, error) {
	token, err := uc.jwt.Generate(u.ID, u.NIM, string(u.Role))
	if err != nil {
		return nil, err
	}
	now := time.Now()
	u.LastLoginAt = &now
	_ = uc.users.Update(u)
	if u.KelasID != nil && u.Kelas == nil {
		if full, err := uc.users.FindByID(u.ID); err == nil {
			u = full
		}
	}
	return &dto.AuthResponse{Token: token, User: toUserResponse(u)}, nil
}
