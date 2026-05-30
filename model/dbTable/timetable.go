package dbTable

type TimetableItem struct {
	TimeRange string      `json:"time_range"`
	Subject   interface{} `json:"subject"` // 可能是数字或字符串
}

type TimetableConfig struct {
	Timetable map[string]map[string]interface{} `json:"timetable" gorm:"type:json;not null;serializer:json"`
	Divider   map[string][]int                  `json:"divider" gorm:"type:json;not null;serializer:json"`
	Start     string                            `json:"start" gorm:"column:start_date"`
}

type Timetable struct {
	ID     uint   `gorm:"primaryKey;autoIncrement;not null"`
	School string `gorm:"uniqueIndex:idx_timetables_school_grade,priority:1;not null;size:50"`
	Grade  string `gorm:"uniqueIndex:idx_timetables_school_grade,priority:2;not null;size:50"`
	TimetableConfig
}
