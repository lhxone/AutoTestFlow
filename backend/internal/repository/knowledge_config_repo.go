package repository

import (
	"auto-test-flow/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const KnowledgeBaseSettingCategory = "knowledge_base"

type KnowledgeConfigRepo struct {
	db *gorm.DB
}

func NewKnowledgeConfigRepo() *KnowledgeConfigRepo {
	return &KnowledgeConfigRepo{db: DB}
}

func (r *KnowledgeConfigRepo) List() ([]model.SystemSetting, error) {
	if r == nil || r.db == nil {
		return nil, gorm.ErrInvalidDB
	}
	var settings []model.SystemSetting
	err := r.db.Where("category = ?", KnowledgeBaseSettingCategory).Order("`key` ASC").Find(&settings).Error
	return settings, err
}

func (r *KnowledgeConfigRepo) GetValue(key string) string {
	if r == nil || r.db == nil {
		return ""
	}
	var setting model.SystemSetting
	if err := r.db.Where("category = ? AND `key` = ?", KnowledgeBaseSettingCategory, key).First(&setting).Error; err != nil {
		return ""
	}
	return setting.Value
}

func (r *KnowledgeConfigRepo) BatchUpsert(settings []model.SystemSetting) error {
	if r == nil || r.db == nil {
		return gorm.ErrInvalidDB
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i := range settings {
			settings[i].Category = KnowledgeBaseSettingCategory
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "category"}, {Name: "key"}},
				DoUpdates: clause.AssignmentColumns([]string{"value", "encrypted", "description", "updated_by"}),
			}).Create(&settings[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
