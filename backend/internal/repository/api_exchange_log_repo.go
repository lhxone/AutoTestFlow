package repository

import (
	"encoding/json"

	"auto-test-flow/internal/model"

	"gorm.io/gorm"
)

type APIExchangeLogRepo struct {
	db *gorm.DB
}

func NewAPIExchangeLogRepo() *APIExchangeLogRepo {
	return &APIExchangeLogRepo{db: DB}
}

func (r *APIExchangeLogRepo) Create(log *model.APIExchangeLog) error {
	return r.db.Create(log).Error
}

func JSONValue(value any) model.JSON {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return model.JSON(data)
}
