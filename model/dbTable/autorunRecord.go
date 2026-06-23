package dbTable

import "time"

type AutorunRecord struct {
	HashID     string                 `gorm:"primaryKey;not null;size:64" json:"hashid"`
	Namespace  string                 `gorm:"not null;size:128;default:default;index" json:"namespace"`
	EType      int                    `gorm:"not null;index" json:"etype"`
	Scope      []string               `gorm:"type:json;not null;serializer:json" json:"scope"`
	Parameters map[string]interface{} `gorm:"type:json;not null;serializer:json" json:"parameters"`
	Level      int                    `gorm:"not null" json:"level"`
	Status     int                    `gorm:"not null" json:"status"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}
