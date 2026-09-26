package dto

// UpdateProfileRequest: update data profil asisten (admin) atau ganti password.
// Email boleh diubah pengguna sendiri: email inilah kanal OTP lupa-password.
type UpdateProfileRequest struct {
	Nama         *string `json:"nama"`
	Email        *string `json:"email"`
	NomorHP      *string `json:"nomor_hp"`
	MedsosLink   *string `json:"medsos_link"`
	FotoURL      *string `json:"foto_url"`
	PasswordLama *string `json:"password_lama"`
	PasswordBaru *string `json:"password_baru" binding:"omitempty,min=6"`
}
