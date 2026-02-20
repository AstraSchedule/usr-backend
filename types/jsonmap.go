package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type JSONMap map[string]string

func (m *JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(*m) // ⭐ 注意解引用
}

func (m *JSONMap) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	data, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("JSONMap.Scan: unexpected type %T", value)
	}
	return json.Unmarshal(data, m)
}
