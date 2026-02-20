package dbTable

type ClientConfigItems struct {
	CountdownTarget string            `json:"countdown_target"`
	WeekDisplay     bool              `json:"week_display"`
	BannerText      string            `json:"banner_text"`
	CSSStyle        map[string]string `json:"css_style" gorm:"type:json;not null;serializer:json"`
}

type ClientConfig struct {
	ID     uint   `gorm:"primaryKey;autoIncrement;not null"`
	School string `gorm:"uniqueIndex:idx_school_grade_class,priority:1;not null"`
	Grade  string `gorm:"uniqueIndex:idx_school_grade_class,priority:2;not null"`
	Class  string `gorm:"uniqueIndex:idx_school_grade_class,priority:3;not null"`
	ClientConfigItems
}
