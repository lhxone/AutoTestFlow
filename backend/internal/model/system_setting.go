package model

import "time"

// SystemSetting 系统设置
type SystemSetting struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Category    string    `gorm:"size:32;uniqueIndex:uk_category_key;not null" json:"category"`
	Key         string    `gorm:"size:64;uniqueIndex:uk_category_key;not null" json:"key"`
	Value       string    `gorm:"type:text;not null" json:"value"`
	Encrypted   int8      `gorm:"default:0" json:"encrypted"`
	Description string    `gorm:"size:255;default:''" json:"description"`
	UpdatedBy   *uint64   `json:"updated_by"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (SystemSetting) TableName() string { return "system_setting" }
