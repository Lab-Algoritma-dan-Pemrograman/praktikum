package repository

import (
	"errors"
	"lab-ap/internal/entity"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrRosterClaimed: baris sudah diklaim (supabase_user_id non-null)
var ErrRosterClaimed = errors.New("roster sudah diklaim")

type UserRepository interface {
	Create(u *entity.User) error
	Update(u *entity.User) error
	Delete(id int) error
	FindByID(id int) (*entity.User, error)
	FindByNIM(nim string) (*entity.User, error)
	FindByEmail(email string) (*entity.User, error)
	FindBySupabaseUserID(uid string) (*entity.User, error)
	ClaimRoster(userID int, supabaseUID string, email *string, passwordHash string) error
	UpdatePasswordByEmail(email, hash string) (int64, error)
	List(role string, kelasID *int, shift *int) ([]entity.User, error)
	BulkUpsert(users []entity.User) error
	ListAsisten() ([]entity.User, error)
	CountByRole(role entity.RoleType) (int64, error)
	CountRegistered(registered bool) (int64, error)
	CountPerKelasShift() ([]KelasShiftCount, error)
}

// KelasShiftCount: hasil agregasi jumlah mahasiswa per kelas + shift.
type KelasShiftCount struct {
	KelasID   int    `json:"kelas_id"`
	NamaKelas string `json:"nama_kelas"`
	Shift     int    `json:"shift"`
	Jumlah    int64  `json:"jumlah"`
}

type userRepository struct{ db *gorm.DB }

func NewUserRepository(db *gorm.DB) UserRepository { return &userRepository{db: db} }

func (r *userRepository) Create(u *entity.User) error { return r.db.Create(u).Error }
func (r *userRepository) Update(u *entity.User) error { return r.db.Save(u).Error }
func (r *userRepository) Delete(id int) error {
	return r.db.Delete(&entity.User{}, id).Error
}

func (r *userRepository) BulkUpsert(users []entity.User) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "nim"}},
		DoUpdates: clause.AssignmentColumns([]string{"nama", "kelas_id", "shift", "gelombang", "kelompok"}),
	}).CreateInBatches(users, 100).Error
}

func (r *userRepository) FindByID(id int) (*entity.User, error) {
	var u entity.User
	if err := r.db.Preload("Kelas").First(&u, id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) FindByNIM(nim string) (*entity.User, error) {
	var u entity.User
	if err := r.db.Where("nim = ?", nim).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) FindByEmail(email string) (*entity.User, error) {
	var u entity.User
	if err := r.db.Where("LOWER(email) = LOWER(?)", email).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) FindBySupabaseUserID(uid string) (*entity.User, error) {
	var u entity.User
	if err := r.db.Where("supabase_user_id = ?", uid).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) ClaimRoster(userID int, supabaseUID string, email *string, passwordHash string) error {
	res := r.db.Model(&entity.User{}).Where("id = ? AND supabase_user_id IS NULL", userID).
		Updates(map[string]interface{}{"supabase_user_id": supabaseUID, "email": email, "is_registered": true, "password_hash": passwordHash})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrRosterClaimed
	}
	return nil
}

func (r *userRepository) UpdatePasswordByEmail(email, hash string) (int64, error) {
	res := r.db.Model(&entity.User{}).Where("LOWER(email) = LOWER(?)", email).Update("password_hash", hash)
	return res.RowsAffected, res.Error
}

func (r *userRepository) List(role string, kelasID *int, shift *int) ([]entity.User, error) {
	var users []entity.User
	q := r.db.Preload("Kelas").Order("nim asc")
	if role != "" {
		q = q.Where("role = ?", role)
	}
	if kelasID != nil {
		q = q.Where("kelas_id = ?", *kelasID)
	}
	if shift != nil {
		q = q.Where("shift = ?", *shift)
	}
	return users, q.Find(&users).Error
}

func (r *userRepository) ListAsisten() ([]entity.User, error) {
	var users []entity.User
	err := r.db.Where("role = ?", entity.RoleAsisten).Order("nama asc").Find(&users).Error
	return users, err
}

func (r *userRepository) CountByRole(role entity.RoleType) (int64, error) {
	var n int64
	err := r.db.Model(&entity.User{}).Where("role = ?", role).Count(&n).Error
	return n, err
}

func (r *userRepository) CountRegistered(registered bool) (int64, error) {
	var n int64
	err := r.db.Model(&entity.User{}).
		Where("role = ? AND is_registered = ?", entity.RoleMahasiswa, registered).
		Count(&n).Error
	return n, err
}

func (r *userRepository) CountPerKelasShift() ([]KelasShiftCount, error) {
	var out []KelasShiftCount
	err := r.db.Model(&entity.User{}).
		Select("users.kelas_id, kelas.nama_kelas, users.shift, COUNT(*) as jumlah").
		Joins("LEFT JOIN kelas ON kelas.id = users.kelas_id").
		Where("users.role = ?", entity.RoleMahasiswa).
		Group("users.kelas_id, kelas.nama_kelas, users.shift").
		Order("users.kelas_id, users.shift").
		Scan(&out).Error
	return out, err
}
