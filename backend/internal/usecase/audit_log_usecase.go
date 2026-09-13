package usecase

import (
	"lab-ap/internal/entity"
	"lab-ap/internal/repository"
)

type AuditLogUsecase struct {
	repo     repository.AuditLogRepository
	userRepo repository.UserRepository
}

func NewAuditLogUsecase(repo repository.AuditLogRepository, userRepo repository.UserRepository) *AuditLogUsecase {
	return &AuditLogUsecase{repo: repo, userRepo: userRepo}
}

func (uc *AuditLogUsecase) LogAction(userID int, customNIM string, action string, description string, ipAddress string, userAgent string) error {
	log := &entity.AuditLog{
		Action:      action,
		Description: description,
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
	}

	if userID > 0 {
		u, err := uc.userRepo.FindByID(userID)
		if err == nil && u != nil {
			log.UserID = &u.ID
			log.NIM = u.NIM
			log.Nama = u.Nama
			log.Role = string(u.Role)
		}
	} else if customNIM != "" {
		log.NIM = customNIM
		log.Nama = "Tamu / Belum Login"
		log.Role = "guest"
		// Coba cari apakah NIM tersebut sudah terdaftar
		u, err := uc.userRepo.FindByNIM(customNIM)
		if err == nil && u != nil {
			log.UserID = &u.ID
			log.Nama = u.Nama
			log.Role = string(u.Role)
		}
	} else {
		log.NIM = "SYSTEM"
		log.Nama = "Sistem / Otomatis"
		log.Role = "system"
	}

	return uc.repo.Create(log)
}

// GetLogs mengambil log. viewerRole menentukan visibilitas: hanya superadmin yang
// boleh melihat jejak akun superadmin (ninja, hidden).
func (uc *AuditLogUsecase) GetLogs(search string, role string, action string, page int, limit int, viewerRole string) ([]entity.AuditLog, int64, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	hideSuperadmin := viewerRole != string(entity.RoleKoordinator)
	// Non-superadmin yang memfilter role=superadmin harus dapat hasil kosong, bukan bocor.
	if hideSuperadmin && role == string(entity.RoleKoordinator) {
		return []entity.AuditLog{}, 0, nil
	}
	return uc.repo.FindAll(search, role, action, page, limit, hideSuperadmin)
}
