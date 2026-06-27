package dbTable

import "time"

type CountdownScheduleItem struct {
	Name     string `json:"name"`
	Date     string `json:"date"`
	Priority int    `json:"priority"`
}

type CountdownRecord struct {
	ID        string                  `gorm:"primaryKey;not null;size:64" json:"id"`
	Namespace string                  `gorm:"not null;size:128;default:default;index" json:"namespace"`
	Scope     []string                `gorm:"type:json;not null;serializer:json" json:"scope"`
	Schedules []CountdownScheduleItem `gorm:"type:json;not null;serializer:json" json:"schedules"`
	CreatedAt time.Time               `json:"created_at"`
	UpdatedAt time.Time               `json:"updated_at"`
}
