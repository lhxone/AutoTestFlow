package repository

import (
	"auto-test-flow/internal/model"

	"gorm.io/gorm"
)

type ProjectRepo struct {
	db *gorm.DB
}

func NewProjectRepo() *ProjectRepo {
	return &ProjectRepo{db: DB}
}

func (r *ProjectRepo) Create(project *model.Project) error {
	return r.db.Create(project).Error
}

func (r *ProjectRepo) GetByID(id uint64) (*model.Project, error) {
	var p model.Project
	err := r.db.Preload("Owner").First(&p, id).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *ProjectRepo) Update(project *model.Project) error {
	return r.db.Save(project).Error
}

func (r *ProjectRepo) Delete(id uint64) error {
	return r.db.Delete(&model.Project{}, id).Error
}

func (r *ProjectRepo) List(keyword string, status *int8, offset, limit int) ([]model.Project, int64, error) {
	query := r.db.Model(&model.Project{}).Preload("Owner")

	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var projects []model.Project
	if err := query.Offset(offset).Limit(limit).Order("id DESC").Find(&projects).Error; err != nil {
		return nil, 0, err
	}

	return projects, total, nil
}

// GetAllActive 获取所有启用的项目
func (r *ProjectRepo) GetAllActive() ([]model.Project, error) {
	var projects []model.Project
	err := r.db.Where("status = 1").Find(&projects).Error
	return projects, err
}
