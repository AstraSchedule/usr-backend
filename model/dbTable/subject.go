package dbTable

type SubjectConfig struct {
	SubjectName map[string]string `json:"subject_name" gorm:"type:json;not null;serializer:json"`
}

type Subject struct {
	ID        uint   `gorm:"primaryKey;autoIncrement;not null"`
	Namespace string `gorm:"uniqueIndex:idx_subjects_school_grade,priority:1;not null;size:128;default:default"`
	School    string `gorm:"uniqueIndex:idx_subjects_school_grade,priority:2;not null;size:50"`
	Grade     string `gorm:"uniqueIndex:idx_subjects_school_grade,priority:3;not null;size:50"`
	SubjectConfig
}
