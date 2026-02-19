package dbTable

import "time"

type TimetableItem struct {
	TimeRange string      `json:"time_range"`
	Subject   interface{} `json:"subject"` // 可能是数字或字符串
}

type TimetableConfig struct {
	Timetable map[string]map[string]interface{} `json:"timetable" gorm:"type:json;not null"`
	Divider   map[string][]int                  `json:"divider" gorm:"type:json;not null"`
	Start     time.Time                         `json:"start" gorm:"column:start_date"`
}

type Timetable struct {
	ID     uint   `gorm:"primaryKey;autoIncrement;not null"`
	School string `gorm:"index:idx_school_grade,priority:1;not null"`
	Grade  string `gorm:"index:idx_school_grade,priority:2;not null"`
	TimetableConfig
}
