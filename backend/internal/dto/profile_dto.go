package dto

// UpdateProfileRequest: update data profil asisten (admin) atau ganti password.
type UpdateProfileRequest struct {
	Nama         *string `json:"nama"`
	NomorHP      *string `json:"nomor_hp"`
	MedsosLink   *string `json:"medsos_link"`
	FotoURL      *string `json:"foto_url"`
	PasswordLama *string `json:"password_lama"`
	PasswordBaru *string `json:"password_baru" binding:"omitempty,min=6"`
}
