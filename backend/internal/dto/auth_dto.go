package dto

// CekNIMRequest: langkah pertama login (cek status NIM).
type CekNIMRequest struct {
	NIM string `json:"nim" binding:"required"`
}

// CekNIMResponse: status NIM untuk menentukan alur (login / register / ditolak).
type CekNIMResponse struct {
	NIM          string `json:"nim"`
	Ditemukan    bool   `json:"ditemukan"`
	IsRegistered bool   `json:"is_registered"`
	Nama         string `json:"nama,omitempty"`
	Pesan        string `json:"pesan"`
}

// LoginRequest: login via NIM atau Email + password.
type LoginRequest struct {
	Identifier string `json:"identifier" binding:"required"`
	Password   string `json:"password" binding:"required"`
}

// RegisterRequest: first-time register (set password) — roster-gated.
type RegisterRequest struct {
	NIM      string `json:"nim" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// ForgotPasswordRequest / ResetPasswordRequest: OTP via Supabase (email only).
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ForgotPasswordResponse: hasil sebenarnya pengiriman OTP.
// Sebelumnya endpoint selalu membalas pesan generik "kode telah dikirim" walau
// tidak ada email yang dikirim sama sekali (akun tanpa kanal email), sehingga
// pengguna menunggu kode yang tak akan pernah datang. Status di sini membuat
// frontend bisa memberi arahan yang benar tanpa membocorkan apakah sebuah
// ALAMAT EMAIL terdaftar (pesan OTP dikirim vs gagal kirim tetap sama).
type ForgotPasswordResponse struct {
	// sent = OTP benar-benar dikirim ke email tersebut.
	Sent bool `json:"sent"`
	// reason = alasan saat sent=false: "no_email_channel" (akun belum punya
	// email/kanal OTP — arahkan ke halaman Profil) atau "send_failed".
	Reason string `json:"reason,omitempty"`
}

type ResetPasswordViaOTPRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
}

// AuthResponse: hasil login/register berhasil.
type AuthResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

// UserResponse: representasi user aman untuk dikirim ke klien.
type UserResponse struct {
	ID         int     `json:"id"`
	Role       string  `json:"role"`
	NIM        string  `json:"nim"`
	Nama       string  `json:"nama"`
	KelasID    *int    `json:"kelas_id"`
	NamaKelas  string  `json:"nama_kelas,omitempty"`
	Shift      *int    `json:"shift"`
	FotoURL    *string `json:"foto_url,omitempty"`
	NomorHP    *string `json:"nomor_hp,omitempty"`
	MedsosLink *string `json:"medsos_link,omitempty"`
}
