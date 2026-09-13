package repository

import (
	"lab-ap/internal/entity"

	"gorm.io/gorm"
)

type AuditLogRepository interface {
	Create(log *entity.AuditLog) error
	// hideSuperadmin menyembunyikan baris milik role superadmin (akun ninja) dari
	// penonton non-superadmin. Baris tetap tersimpan, hanya tidak ditampilkan.
	FindAll(search string, role string, action string, page int, limit int, hideSuperadmin bool) ([]entity.AuditLog, int64, error)
}

type auditLogRepository struct {
	db *gorm.DB
}

func NewAuditLogRepository(db *gorm.DB) AuditLogRepository {
	return &auditLogRepository{db: db}
}

func (r *auditLogRepository) Create(log *entity.AuditLog) error {
	return r.db.Create(log).Error
}

func (r *auditLogRepository) FindAll(search string, role string, action string, page int, limit int, hideSuperadmin bool) ([]entity.AuditLog, int64, error) {
	var logs []entity.AuditLog
	var total int64

	query := r.db.Model(&entity.AuditLog{})

	if hideSuperadmin {
		query = query.Where("role <> ?", string(entity.RoleKoordinator))
	}

	if search != "" {
		s := "%" + search + "%"
		query = query.Where("nim ILIKE ? OR nama ILIKE ? OR description ILIKE ? OR action ILIKE ?", s, s, s, s)
	}

	if role != "" {
		query = query.Where("role = ?", role)
	}

	if action != "" {
		query = query.Where("action = ?", action)
	}

	// Count total rows matching criteria
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and sort by newest
	offset := (page - 1) * limit
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
