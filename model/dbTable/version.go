package dbTable

import "time"

type DataVersion struct {
	ID      uint      `gorm:"primaryKey;autoIncrement;not null"`
	School  string    `gorm:"index:idx_school_grade_class,priority:1;not null"`
	Grade   string    `gorm:"index:idx_school_grade_class,priority:2 not null"`
	Class   string    `gorm:"index:idx_school_grade_class,priority:3 not null"`
	Version time.Time `gorm:"not null;default:0"`
}
