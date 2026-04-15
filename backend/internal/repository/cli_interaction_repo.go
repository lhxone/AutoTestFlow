package repository

import (
	"auto-test-flow/internal/model"
	"gorm.io/gorm"
)

type CLIInteractionRepo struct {
	db *gorm.DB
}

func NewCLIInteractionRepo() *CLIInteractionRepo {
	return &CLIInteractionRepo{db: DB}
}

func (r *CLIInteractionRepo) Create(interaction *model.CLIInteraction) error {
	return r.db.Create(interaction).Error
}

func (r *CLIInteractionRepo) GetByID(id uint) (*model.CLIInteraction, error) {
	var interaction model.CLIInteraction
	err := r.db.First(&interaction, id).Error
	if err != nil {
		return nil, err
	}
	return &interaction, nil
}

func (r *CLIInteractionRepo) GetByTaskID(taskID uint) ([]model.CLIInteraction, error) {
	var interactions []model.CLIInteraction
	err := r.db.Where("task_id = ?", taskID).Order("created_at ASC").Find(&interactions).Error
	return interactions, err
}

func (r *CLIInteractionRepo) GetPendingByTaskID(taskID uint) ([]model.CLIInteraction, error) {
	var interactions []model.CLIInteraction
	err := r.db.Where("task_id = ? AND status = ?", taskID, "pending").Order("created_at ASC").Find(&interactions).Error
	return interactions, err
}

func (r *CLIInteractionRepo) UpdateResponse(id, userID uint, status, response string) error {
	return r.db.Model(&model.CLIInteraction{}).Where("id = ?", id).Updates(map[string]any{
		"status":        status,
		"user_response": response,
		"user_id":       userID,
		"responded_at":  gorm.Expr("NOW()"),
	}).Error
}

func (r *CLIInteractionRepo) Delete(id uint) error {
	return r.db.Delete(&model.CLIInteraction{}, id).Error
}
