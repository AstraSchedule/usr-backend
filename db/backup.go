package db

import (
	"AstraScheduleServerGo/model/dbTable"
	"reflect"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BackupMeta struct {
	SchemaVersion int    `json:"schema_version"`
	GeneratedAt   string `json:"generated_at"`
}

type BackupPayload struct {
	Meta            BackupMeta                `json:"meta"`
	Schedules       []dbTable.Schedule        `json:"schedules"`
	ClientConfigs   []dbTable.ClientConfig    `json:"client_configs"`
	Timetables      []dbTable.Timetable       `json:"timetables"`
	Subjects        []dbTable.Subject         `json:"subjects"`
	DataVersions    []dbTable.DataVersion     `json:"data_versions"`
	AutorunRecords  []dbTable.AutorunRecord   `json:"autorun_records"`
	CountdownRecord []dbTable.CountdownRecord `json:"countdown_records"`
}

type BackupImportResult struct {
	Imported map[string]int `json:"imported"`
	Total    int            `json:"total"`
}

const (
	orderByIDAsc        = "id ASC"
	orderByCreatedAtAsc = "created_at ASC"
)

// resetIDsToZero 将切片中所有元素的 ID 字段重置为 0，让数据库自动分配新 ID
func resetIDsToZero(slice any) {
	v := reflect.ValueOf(slice)
	if v.Kind() != reflect.Slice {
		return
	}
	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		if elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		if elem.Kind() != reflect.Struct {
			continue
		}
		idField := elem.FieldByName("ID")
		if !idField.IsValid() || !idField.CanSet() {
			continue
		}
		switch idField.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			idField.SetInt(0)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			idField.SetUint(0)
		}
	}
}

func ExportBackup() (*BackupPayload, error) {
	dbConn := GetDB()
	payload := &BackupPayload{
		Meta: BackupMeta{
			SchemaVersion: 1,
			GeneratedAt:   time.Now().Format(time.RFC3339),
		},
	}

	if err := dbConn.Order(orderByIDAsc).Find(&payload.Schedules).Error; err != nil {
		return nil, err
	}
	if err := dbConn.Order(orderByIDAsc).Find(&payload.ClientConfigs).Error; err != nil {
		return nil, err
	}
	if err := dbConn.Order(orderByIDAsc).Find(&payload.Timetables).Error; err != nil {
		return nil, err
	}
	if err := dbConn.Order(orderByIDAsc).Find(&payload.Subjects).Error; err != nil {
		return nil, err
	}
	if err := dbConn.Order(orderByIDAsc).Find(&payload.DataVersions).Error; err != nil {
		return nil, err
	}
	if err := dbConn.Order(orderByCreatedAtAsc).Find(&payload.AutorunRecords).Error; err != nil {
		return nil, err
	}
	if err := dbConn.Order(orderByCreatedAtAsc).Find(&payload.CountdownRecord).Error; err != nil {
		return nil, err
	}

	return payload, nil
}

func ImportBackup(payload *BackupPayload) (*BackupImportResult, error) {
	if payload == nil {
		return nil, gorm.ErrInvalidData
	}

	tx := GetDB().Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}

	result := &BackupImportResult{Imported: map[string]int{}}

	rollback := func(err error) (*BackupImportResult, error) {
		tx.Rollback()
		return nil, err
	}

	type importStep struct {
		name string
		run  func(*gorm.DB) (int, error)
	}

	steps := []importStep{
		{name: "schedules", run: func(tx *gorm.DB) (int, error) { return importSchedules(tx, payload.Schedules) }},
		{name: "client_configs", run: func(tx *gorm.DB) (int, error) { return importClientConfigs(tx, payload.ClientConfigs) }},
		{name: "timetables", run: func(tx *gorm.DB) (int, error) { return importTimetables(tx, payload.Timetables) }},
		{name: "subjects", run: func(tx *gorm.DB) (int, error) { return importSubjects(tx, payload.Subjects) }},
		{name: "data_versions", run: func(tx *gorm.DB) (int, error) { return importDataVersions(tx, payload.DataVersions) }},
		{name: "autorun_records", run: func(tx *gorm.DB) (int, error) { return importAutorunRecords(tx, payload.AutorunRecords) }},
		{name: "countdown_records", run: func(tx *gorm.DB) (int, error) { return importCountdownRecords(tx, payload.CountdownRecord) }},
	}

	for _, step := range steps {
		n, err := step.run(tx)
		if err != nil {
			return rollback(err)
		}
		result.Imported[step.name] = n
		result.Total += n
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return result, nil
}

// onConflictSpec 定义导入数据时的 OnConflict 配置
type onConflictSpec struct {
	columns    []clause.Column
	updateCols []string
}

// importRows 通用的批量导入函数，重置 ID 后通过 OnConflict 进行 upsert
func importRows[T any](tx *gorm.DB, rows []T, spec onConflictSpec) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	// 使用 reflect 将所有行的 ID 字段重置为 0，让数据库自动分配新 ID
	resetIDsToZero(rows)
	if err := tx.Clauses(clause.OnConflict{
		Columns:   spec.columns,
		DoUpdates: clause.AssignmentColumns(spec.updateCols),
	}).Create(&rows).Error; err != nil {
		return 0, err
	}
	return len(rows), nil
}

func importSchedules(tx *gorm.DB, rows []dbTable.Schedule) (int, error) {
	return importRows(tx, rows, onConflictSpec{
		columns:    []clause.Column{{Name: "school"}, {Name: "grade"}, {Name: "class"}},
		updateCols: []string{"daily_classes"},
	})
}

func importClientConfigs(tx *gorm.DB, rows []dbTable.ClientConfig) (int, error) {
	return importRows(tx, rows, onConflictSpec{
		columns: []clause.Column{{Name: "school"}, {Name: "grade"}, {Name: "class"}},
		updateCols: []string{
			"countdown_target",
			"weather_alert_override",
			"weather_alert_brief",
			"week_display",
			"banner_text",
			"css_style",
		},
	})
}

func importTimetables(tx *gorm.DB, rows []dbTable.Timetable) (int, error) {
	return importRows(tx, rows, onConflictSpec{
		columns:    []clause.Column{{Name: "school"}, {Name: "grade"}},
		updateCols: []string{"timetable", "divider", "start_date"},
	})
}

func importSubjects(tx *gorm.DB, rows []dbTable.Subject) (int, error) {
	return importRows(tx, rows, onConflictSpec{
		columns:    []clause.Column{{Name: "school"}, {Name: "grade"}},
		updateCols: []string{"subject_name"},
	})
}

func importDataVersions(tx *gorm.DB, rows []dbTable.DataVersion) (int, error) {
	return importRows(tx, rows, onConflictSpec{
		columns:    []clause.Column{{Name: "school"}, {Name: "grade"}, {Name: "class"}},
		updateCols: []string{"version"},
	})
}

func importAutorunRecords(tx *gorm.DB, rows []dbTable.AutorunRecord) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "hash_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"e_type",
			"scope",
			"parameters",
			"level",
			"status",
			"created_at",
			"updated_at",
		}),
	}).Create(&rows).Error
	if err != nil {
		return 0, err
	}
	return len(rows), nil
}

func importCountdownRecords(tx *gorm.DB, rows []dbTable.CountdownRecord) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"scope",
			"schedules",
			"created_at",
			"updated_at",
		}),
	}).Create(&rows).Error
	if err != nil {
		return 0, err
	}
	return len(rows), nil
}
