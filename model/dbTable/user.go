package dbTable

import "time"

type User struct {
	ID               uint      `gorm:"primaryKey;autoIncrement;not null"`
	Namespace        string    `gorm:"uniqueIndex:idx_users_namespace_username,priority:1;not null;size:128;default:default"`
	Username         string    `gorm:"uniqueIndex:idx_users_namespace_username,priority:2;not null;size:50"`
	PasswordHash     string    `gorm:"not null;size:255"`
	Role             string    `gorm:"not null;size:20;default:readonly"`
	Scope            string    `gorm:"size:255"`
	MustChangePwd    bool      `gorm:"not null;default:true"`
	MustChangeUsername bool     `gorm:"not null;default:false"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
