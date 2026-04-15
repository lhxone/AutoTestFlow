package repository

import (
	"auto-test-flow/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SettingRepo struct {
	db *gorm.DB
}

func NewSettingRepo() *SettingRepo {
	return &SettingRepo{db: DB}
}

// GetByCategory 获取某分类下所有配置
func (r *SettingRepo) GetByCategory(category string) ([]model.SystemSetting, error) {
	if r == nil || r.db == nil {
		return nil, gorm.ErrInvalidDB
	}
	var settings []model.SystemSetting
	err := r.db.Where("category = ?", category).Order("`key` ASC").Find(&settings).Error
	return settings, err
}

// Get 获取单个配置
func (r *SettingRepo) Get(category, key string) (*model.SystemSetting, error) {
	if r == nil || r.db == nil {
		return nil, gorm.ErrInvalidDB
	}
	var s model.SystemSetting
	err := r.db.Where("category = ? AND `key` = ?", category, key).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetValue 获取配置值(快捷方法)
func (r *SettingRepo) GetValue(category, key string) string {
	s, err := r.Get(category, key)
	if err != nil {
		return ""
	}
	return s.Value
}

// Upsert 插入或更新配置
func (r *SettingRepo) Upsert(setting *model.SystemSetting) error {
	if r == nil || r.db == nil {
		return gorm.ErrInvalidDB
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "category"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "encrypted", "description", "updated_by"}),
	}).Create(setting).Error
}

// BatchUpsert 批量更新某分类的配置
func (r *SettingRepo) BatchUpsert(settings []model.SystemSetting) error {
	if r == nil || r.db == nil {
		return gorm.ErrInvalidDB
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i := range settings {
			err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "category"}, {Name: "key"}},
				DoUpdates: clause.AssignmentColumns([]string{"value", "encrypted", "description", "updated_by"}),
			}).Create(&settings[i]).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}
