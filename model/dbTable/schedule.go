package dbTable

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// ClassList 兼容旧版一维数组和新版二维数组格式
// 旧版: ["物", "数"]
// 新版: [["物"], ["数"]]
type ClassList [][]string

func (c *ClassList) UnmarshalJSON(data []byte) error {
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	arr, ok := raw.([]interface{})
	if !ok {
		*c = [][]string{}
		return nil
	}
	result := make([][]string, 0, len(arr))
	for _, item := range arr {
		result = append(result, parseClassListItem(item))
	}
	*c = result
	return nil
}

// parseClassListItem 将单个 JSON 元素解析为 []string
// 支持旧版字符串格式和新版数组格式
func parseClassListItem(item interface{}) []string {
	switch elem := item.(type) {
	case []interface{}:
		row := make([]string, 0, len(elem))
		for _, e := range elem {
			if s, ok := e.(string); ok {
				row = append(row, s)
			}
		}
		return row
	case string:
		return []string{elem}
	default:
		return []string{}
	}
}

func (c ClassList) MarshalJSON() ([]byte, error) {
	return json.Marshal([][]string(c))
}

func (c *ClassList) Scan(value interface{}) error {
	if value == nil {
		*c = [][]string{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("ClassList.Scan: expected []byte, got %T", value)
	}
	return c.UnmarshalJSON(bytes)
}

func (c ClassList) Value() (driver.Value, error) {
	return json.Marshal([][]string(c))
}

type DailyClass struct {
	Chinese   string    `json:"Chinese"`
	English   string    `json:"English"`
	ClassList ClassList `json:"classList" gorm:"type:json;serializer:json"`
	Timetable string    `json:"timetable"`
}

type Schedule struct {
	ID           uint          `gorm:"primaryKey;autoIncrement;not null"`
	Namespace    string        `gorm:"uniqueIndex:idx_schedules_school_grade_class,priority:1;not null;size:128;default:default"`
	School       string        `gorm:"uniqueIndex:idx_schedules_school_grade_class,priority:2;not null;size:50"`
	Grade        string        `gorm:"uniqueIndex:idx_schedules_school_grade_class,priority:3;not null;size:50"`
	Class        string        `gorm:"uniqueIndex:idx_schedules_school_grade_class,priority:4;not null;size:50"`
	DailyClasses [7]DailyClass `gorm:"type:json;not null;serializer:json" json:"daily_class"`
}
