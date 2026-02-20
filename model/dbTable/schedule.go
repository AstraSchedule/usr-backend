package dbTable

type DailyClass struct {
	Chinese   string   `json:"Chinese"`
	English   string   `json:"English"`
	ClassList []string `json:"classList" gorm:"type:json"`
	Timetable string   `json:"timetable"`
}

type Schedule struct {
	ID           uint          `gorm:"primaryKey;autoIncrement;not null"`
	School       string        `gorm:"uniqueIndex:idx_unique_school_grade_class,priority:1;not null;size:50"`
	Grade        string        `gorm:"uniqueIndex:idx_unique_school_grade_class,priority:2;not null;size:50"`
	Class        string        `gorm:"uniqueIndex:idx_unique_school_grade_class,priority:3;not null;size:50"`
	DailyClasses [7]DailyClass `gorm:"type:json;not nul;serializer:json" json:"daily_class"`
}
