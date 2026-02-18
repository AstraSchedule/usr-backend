package model

type DailyClass struct {
	Chinese   string   `json:"chinese"`
	English   string   `json:"english"`
	ClassList []string `json:"class_list" gorm:"type:json"`
	Timetable string   `json:"timetable"`
}

type Schedule struct {
	ID           uint          `gorm:"primaryKey" json:"id"`
	School       string        `gorm:"index:idx_school_grade_class,priority:1"`
	Grade        string        `gorm:"index:idx_school_grade_class,priority:2"`
	Class        string        `gorm:"index:idx_school_grade_class,priority:3"`
	DailyClasses [7]DailyClass `gorm:"type:json" json:"daily_class"`
}
