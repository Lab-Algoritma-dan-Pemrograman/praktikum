package usecase

import (
	"errors"
	"testing"

	"lab-ap/internal/entity"
	"lab-ap/internal/repository/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Guard hapus user: hanya superadmin boleh hapus asisten, superadmin kebal,
// dan tak seorang pun bisa hapus akunnya sendiri.
func TestUserUsecase_Delete_Guards(t *testing.T) {
	const (
		superID = 1
		adminID = 10
		mhsID   = 99
	)

	newUC := func(t *testing.T, target *entity.User, expectDelete bool) *UserUsecase {
		users := mocks.NewUserRepository(t)
		users.On("FindByID", target.ID).Return(target, nil).Maybe()
		if expectDelete {
			users.On("Delete", target.ID).Return(nil).Once()
		}
		return NewUserUsecase(users, mocks.NewKelasRepository(t))
	}

	admin := &entity.User{ID: adminID, Role: entity.RoleAsisten, NIM: "202314020"}
	super := &entity.User{ID: superID, Role: entity.RoleKoordinator, NIM: "superadmin_ap"}
	mhs := &entity.User{ID: mhsID, Role: entity.RoleMahasiswa, NIM: "202411001"}

	t.Run("admin tidak boleh hapus asisten lain", func(t *testing.T) {
		uc := newUC(t, admin, false)
		err := uc.Delete(adminID, 11, string(entity.RoleAsisten))
		assert.True(t, errors.Is(err, ErrForbidden))
	})

	t.Run("superadmin boleh hapus asisten", func(t *testing.T) {
		uc := newUC(t, admin, true)
		assert.NoError(t, uc.Delete(adminID, superID, string(entity.RoleKoordinator)))
	})

	t.Run("superadmin tidak bisa dihapus superadmin lain", func(t *testing.T) {
		uc := newUC(t, super, false)
		err := uc.Delete(superID, 2, string(entity.RoleKoordinator))
		assert.True(t, errors.Is(err, ErrForbidden))
	})

	t.Run("tidak bisa hapus akun sendiri", func(t *testing.T) {
		uc := newUC(t, admin, false)
		err := uc.Delete(adminID, adminID, string(entity.RoleKoordinator))
		assert.True(t, errors.Is(err, ErrForbidden))
	})

	t.Run("admin tetap boleh hapus mahasiswa", func(t *testing.T) {
		uc := newUC(t, mhs, true)
		assert.NoError(t, uc.Delete(mhsID, adminID, string(entity.RoleAsisten)))
	})
}

// Log superadmin hanya terlihat oleh superadmin; role lain difilter di query.
func TestAuditLogUsecase_GetLogs_HidesSuperadmin(t *testing.T) {
	t.Run("viewer admin memicu hideSuperadmin=true", func(t *testing.T) {
		repo := mocks.NewAuditLogRepository(t)
		repo.On("FindAll", "", "", "", 1, 20, true).Return([]entity.AuditLog{}, int64(0), nil).Once()
		uc := NewAuditLogUsecase(repo, mocks.NewUserRepository(t))
		_, _, err := uc.GetLogs("", "", "", 1, 20, string(entity.RoleAsisten))
		assert.NoError(t, err)
	})

	t.Run("viewer superadmin melihat semua", func(t *testing.T) {
		repo := mocks.NewAuditLogRepository(t)
		repo.On("FindAll", "", "", "", 1, 20, false).Return([]entity.AuditLog{}, int64(0), nil).Once()
		uc := NewAuditLogUsecase(repo, mocks.NewUserRepository(t))
		_, _, err := uc.GetLogs("", "", "", 1, 20, string(entity.RoleKoordinator))
		assert.NoError(t, err)
	})

	t.Run("admin filter role=superadmin dapat kosong tanpa query", func(t *testing.T) {
		repo := mocks.NewAuditLogRepository(t)
		repo.AssertNotCalled(t, "FindAll", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		uc := NewAuditLogUsecase(repo, mocks.NewUserRepository(t))
		logs, total, err := uc.GetLogs("", string(entity.RoleKoordinator), "", 1, 20, string(entity.RoleAsisten))
		assert.NoError(t, err)
		assert.Empty(t, logs)
		assert.Zero(t, total)
	})
}
