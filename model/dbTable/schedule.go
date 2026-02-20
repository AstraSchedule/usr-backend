package dbTable

type DailyClass struct {
	Chinese   string   `json:"chinese"`
	English   string   `json:"english"`
	ClassList []string `json:"classList" gorm:"type:json"`
	Timetable string   `json:"timetable"`
}

type Schedule struct {
	ID           uint          `gorm:"primaryKey;autoIncrement;not null"`
	School       string        `gorm:"index:idx_school_grade_class,priority:1 not null"`
	Grade        string        `gorm:"index:idx_school_grade_class,priority:2 not null"`
	Class        string        `gorm:"index:idx_school_grade_class,priority:3 not null"`
	DailyClasses [7]DailyClass `gorm:"type:json;not nul;serializer:json" json:"daily_class"`
}
