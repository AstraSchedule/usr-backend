package dbTable

import "time"

type DataVersion struct {
	ID      uint      `gorm:"primaryKey;autoIncrement;not null"`
	School  string    `gorm:"uniqueIndex:idx_unique_school_grade_class,priority:1;not null;size:50"`
	Grade   string    `gorm:"uniqueIndex:idx_unique_school_grade_class,priority:2;not null;size:50"`
	Class   string    `gorm:"uniqueIndex:idx_unique_school_grade_class,priority:3;not null;size:50"`
	Version time.Time `gorm:"not null"`
}
