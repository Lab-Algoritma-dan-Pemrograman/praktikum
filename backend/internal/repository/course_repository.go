package repository

import (
	"lab-ap/internal/entity"

	"gorm.io/gorm"
)

type CourseRepository interface {
	Create(c *entity.Course) error
	Update(c *entity.Course) error
	Delete(id int) error
	FindByID(id int) (*entity.Course, error)
	ListBySesi(sesiID int) ([]entity.Course, error)
	ListByJenis(jenis entity.JenisCourse) ([]entity.Course, error)
	FindBySesiJenis(sesiID int, jenis entity.JenisCourse) (*entity.Course, error)
}

type courseRepository struct{ db *gorm.DB }

func NewCourseRepository(db *gorm.DB) CourseRepository { return &courseRepository{db: db} }

func (r *courseRepository) Create(c *entity.Course) error { return r.db.Create(c).Error }
func (r *courseRepository) Update(c *entity.Course) error { return r.db.Save(c).Error }
func (r *courseRepository) Delete(id int) error {
	return r.db.Delete(&entity.Course{}, id).Error
}

func (r *courseRepository) FindByID(id int) (*entity.Course, error) {
	var c entity.Course
	if err := r.db.First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *courseRepository) ListBySesi(sesiID int) ([]entity.Course, error) {
	var cs []entity.Course
	return cs, r.db.Where("sesi_praktikum_id = ?", sesiID).Find(&cs).Error
}

func (r *courseRepository) ListByJenis(jenis entity.JenisCourse) ([]entity.Course, error) {
	var cs []entity.Course
	return cs, r.db.Where("jenis = ?", jenis).Order("id asc").Find(&cs).Error
}

func (r *courseRepository) FindBySesiJenis(sesiID int, jenis entity.JenisCourse) (*entity.Course, error) {
	var c entity.Course
	err := r.db.Where("sesi_praktikum_id = ? AND jenis = ?", sesiID, jenis).First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}
