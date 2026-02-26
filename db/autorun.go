package db

import (
	"AstraScheduleServerGo/model/dbTable"
	"time"

	"gorm.io/gorm/clause"
)

func FetchAutorunRecords(hashid string) ([]dbTable.AutorunRecord, error) {
	records := make([]dbTable.AutorunRecord, 0)
	q := GetDB().Model(&dbTable.AutorunRecord{})
	if hashid != "" {
		q = q.Where("hash_id = ?", hashid)
	}
	err := q.Find(&records).Error
	return records, err
}

func DeleteAutorunRecord(hashid string) (int64, error) {
	resp := GetDB().Where("hash_id = ?", hashid).Delete(&dbTable.AutorunRecord{})
	return resp.RowsAffected, resp.Error
}

func UpsertAutorunRecord(record *dbTable.AutorunRecord) error {
	return GetDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "hash_id"}},
		UpdateAll: true,
	}).Create(record).Error
}

func deriveStatusForRecord(etype int, parameters map[string]interface{}, today time.Time) int {
	if etype < 0 || etype > 3 {
		return 0
	}
	var rule map[string]interface{}
	if rv, ok := parameters["rule"].(map[string]interface{}); ok {
		rule = rv
	} else {
		rule = parameters
	}
	dateStr, _ := rule["date"].(string)
	if dateStr == "" {
		return 0
	}
	day, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return 0
	}
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	ruleDate := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	if todayDate.Before(ruleDate) {
		return 0
	}
	if todayDate.Equal(ruleDate) {
		return 1
	}
	return 2
}

func RefreshAutorunStatuses(today time.Time) (int64, error) {
	records, err := FetchAutorunRecords("")
	if err != nil {
		return 0, err
	}
	updated := int64(0)
	for i := range records {
		newStatus := deriveStatusForRecord(records[i].EType, records[i].Parameters, today)
		if newStatus == records[i].Status {
			continue
		}
		if err := GetDB().Model(&dbTable.AutorunRecord{}).
			Where("hash_id = ?", records[i].HashID).
			Update("status", newStatus).Error; err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}
