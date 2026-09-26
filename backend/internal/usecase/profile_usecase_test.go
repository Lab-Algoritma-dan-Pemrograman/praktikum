package usecase_test

import (
	"testing"

	"lab-ap/config"
	"lab-ap/internal/dto"
	"lab-ap/internal/entity"
	"lab-ap/internal/repository/mocks"
	"lab-ap/internal/usecase"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

func ptr(s string) *string { return &s }

// mahasiswaTanpaEmail: baris hasil register jalur lokal lama — hash ada, email belum.
func mahasiswaTanpaEmail() *entity.User {
	kelasID := 1
	return &entity.User{
		ID:           10,
		NIM:          "202615056",
		Nama:         "Efraim",
		Role:         entity.RoleMahasiswa,
		KelasID:      &kelasID,
		IsRegistered: true,
	}
}

// Email diisi sendiri mahasiswa: tanpa ini 302 akun tidak punya kanal OTP lupa-password.
func TestProfile_UpdateEmail_Sukses(t *testing.T) {
	repo := mocks.NewUserRepository(t)
	uc := usecase.NewProfileUsecase(repo, &config.Config{})

	repo.On("FindByID", 10).Return(mahasiswaTanpaEmail(), nil)
	repo.On("FindByEmail", "efraim@gmail.com").Return(nil, gorm.ErrRecordNotFound)
	repo.On("Update", mock.AnythingOfType("*entity.User")).Return(nil)

	res, err := uc.Update(10, dto.UpdateProfileRequest{Email: ptr("Efraim@Gmail.com")})

	assert.NoError(t, err)
	assert.NotNil(t, res.Email)
	assert.Equal(t, "efraim@gmail.com", *res.Email, "email harus dinormalisasi lowercase")
	repo.AssertExpectations(t)
}

func TestProfile_UpdateEmail_DitolakDomainLuar(t *testing.T) {
	repo := mocks.NewUserRepository(t)
	uc := usecase.NewProfileUsecase(repo, &config.Config{})
	repo.On("FindByID", 10).Return(mahasiswaTanpaEmail(), nil)

	res, err := uc.Update(10, dto.UpdateProfileRequest{Email: ptr("efraim@bukanmail.xyz")})

	assert.ErrorIs(t, err, usecase.ErrBadRequest)
	assert.Nil(t, res)
	repo.AssertExpectations(t)
}

func TestProfile_UpdateEmail_DitolakKalauDipakaiOrangLain(t *testing.T) {
	repo := mocks.NewUserRepository(t)
	uc := usecase.NewProfileUsecase(repo, &config.Config{})
	lain := mahasiswaTanpaEmail()
	lain.ID = 99

	repo.On("FindByID", 10).Return(mahasiswaTanpaEmail(), nil)
	repo.On("FindByEmail", "efraim@gmail.com").Return(lain, nil)

	res, err := uc.Update(10, dto.UpdateProfileRequest{Email: ptr("efraim@gmail.com")})

	assert.ErrorIs(t, err, usecase.ErrConflict)
	assert.Nil(t, res)
	repo.AssertExpectations(t)
}

// Email sendiri diulang = no-op: tidak perlu lookup unik maupun tulis apa pun
// (kalau dipaksa lookup, orang bisa "menemukan dirinya sendiri" lalu dianggap konflik).
func TestProfile_UpdateEmail_SamaSendiri_NoOp(t *testing.T) {
	repo := mocks.NewUserRepository(t)
	uc := usecase.NewProfileUsecase(repo, &config.Config{})
	u := mahasiswaTanpaEmail()
	u.Email = ptr("efraim@gmail.com")

	repo.On("FindByID", 10).Return(u, nil)
	repo.On("Update", mock.AnythingOfType("*entity.User")).Return(nil)

	res, err := uc.Update(10, dto.UpdateProfileRequest{Email: ptr("efraim@gmail.com")})

	assert.NoError(t, err)
	assert.Equal(t, "efraim@gmail.com", *res.Email)
	repo.AssertExpectations(t)
	repo.AssertNotCalled(t, "FindByEmail", mock.Anything)
}

// Tanpa Supabase di config, update email tetap jalan lokal (tidak panic / tidak hang).
func TestProfile_UpdateEmail_TanpaSupabase_TetapSimpanLokal(t *testing.T) {
	repo := mocks.NewUserRepository(t)
	uc := usecase.NewProfileUsecase(repo, &config.Config{})

	repo.On("FindByID", 10).Return(mahasiswaTanpaEmail(), nil)
	repo.On("FindByEmail", "efraim@gmail.com").Return(nil, gorm.ErrRecordNotFound)
	repo.On("Update", mock.AnythingOfType("*entity.User")).Return(nil)

	res, err := uc.Update(10, dto.UpdateProfileRequest{Email: ptr("efraim@gmail.com")})

	assert.NoError(t, err)
	assert.Equal(t, "efraim@gmail.com", *res.Email)
	repo.AssertExpectations(t)
}
