package entity

import "time"

// User: role admin = asisten lab, role superadmin = koordinator (ninja, hidden), role user = mahasiswa.
type User struct {
	ID             int      `gorm:"primaryKey;autoIncrement" json:"id"`
	Role           RoleType `gorm:"type:varchar(10);not null;index" json:"role"`
	NIM            string   `gorm:"type:varchar(32);uniqueIndex;not null" json:"nim"`
	Nama           string   `gorm:"type:varchar(150);not null" json:"nama"`
	Email          *string  `gorm:"type:varchar(150);uniqueIndex" json:"email,omitempty"`
	SupabaseUserID *string  `gorm:"type:uuid;uniqueIndex" json:"-"`
	PasswordHash   *string  `gorm:"type:varchar(255)" json:"-"`
	// Hash+salt password lama dari Firebase Auth (scrypt). Diisi saat migrasi,
	// dikosongkan setelah login pertama (di-rehash ke bcrypt di PasswordHash).
	FbPasswordHash *string    `gorm:"type:varchar(255)" json:"-"`
	FbPasswordSalt *string    `gorm:"type:varchar(255)" json:"-"`
	IsRegistered   bool       `gorm:"default:false" json:"is_registered"`
	KelasID        *int       `gorm:"index" json:"kelas_id"`
	Shift          *int       `json:"shift"`
	Gelombang      *int       `json:"gelombang"` // hanya dipakai saat ujian praktik
	Kelompok       *string    `gorm:"type:varchar(50)" json:"kelompok"`
	FotoURL        *string    `gorm:"type:varchar(500)" json:"foto_url"`
	NomorHP        *string    `gorm:"type:varchar(30)" json:"nomor_hp"`
	MedsosLink     *string    `gorm:"type:varchar(500)" json:"medsos_link"`
	LastLoginAt    *time.Time `json:"last_login_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	Kelas *Kelas `gorm:"foreignKey:KelasID" json:"kelas,omitempty"`
}

func (User) TableName() string { return "users" }
