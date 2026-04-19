package db

import (
	"AstraScheduleServerGo/model/dbTable"
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

func importSchedules(tx *gorm.DB, rows []dbTable.Schedule) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	for i := range rows {
		rows[i].ID = 0
	}
	err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "school"}, {Name: "grade"}, {Name: "class"}},
		DoUpdates: clause.AssignmentColumns([]string{"daily_classes"}),
	}).Create(&rows).Error
	if err != nil {
		return 0, err
	}
	return len(rows), nil
}

func importClientConfigs(tx *gorm.DB, rows []dbTable.ClientConfig) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	for i := range rows {
		rows[i].ID = 0
	}
	err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "school"}, {Name: "grade"}, {Name: "class"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"countdown_target",
			"weather_alert_override",
			"weather_alert_brief",
			"week_display",
			"banner_text",
			"css_style",
		}),
	}).Create(&rows).Error
	if err != nil {
		return 0, err
	}
	return len(rows), nil
}

func importTimetables(tx *gorm.DB, rows []dbTable.Timetable) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	for i := range rows {
		rows[i].ID = 0
	}
	err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "school"}, {Name: "grade"}},
		DoUpdates: clause.AssignmentColumns([]string{"timetable", "divider", "start_date"}),
	}).Create(&rows).Error
	if err != nil {
		return 0, err
	}
	return len(rows), nil
}

func importSubjects(tx *gorm.DB, rows []dbTable.Subject) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	for i := range rows {
		rows[i].ID = 0
	}
	err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "school"}, {Name: "grade"}},
		DoUpdates: clause.AssignmentColumns([]string{"subject_name"}),
	}).Create(&rows).Error
	if err != nil {
		return 0, err
	}
	return len(rows), nil
}

func importDataVersions(tx *gorm.DB, rows []dbTable.DataVersion) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	for i := range rows {
		rows[i].ID = 0
	}
	err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "school"}, {Name: "grade"}, {Name: "class"}},
		DoUpdates: clause.AssignmentColumns([]string{"version"}),
	}).Create(&rows).Error
	if err != nil {
		return 0, err
	}
	return len(rows), nil
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
