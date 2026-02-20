package dbTable

type SubjectConfig struct {
	SubjectName map[string]string `json:"subject_name" gorm:"type:json;not null;serializer:json"`
}

type Subject struct {
	ID     uint   `gorm:"primaryKey;autoIncrement;not null"`
	School string `gorm:"index:idx_school_grade,priority:1;not null"`
	Grade  string `gorm:"index:idx_school_grade,priority:2;not null"`
	SubjectConfig
}
