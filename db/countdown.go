package db

import (
	"AstraScheduleServerGo/model/dbTable"

	"gorm.io/gorm/clause"
)

func FetchCountdownRecords(id string) ([]dbTable.CountdownRecord, error) {
	records := make([]dbTable.CountdownRecord, 0)
	q := GetDB().Model(&dbTable.CountdownRecord{})
	if id != "" {
		q = q.Where("id = ?", id)
	}
	err := q.Order("updated_at DESC").Find(&records).Error
	return records, err
}

func DeleteCountdownRecord(id string) (int64, error) {
	resp := GetDB().Where("id = ?", id).Delete(&dbTable.CountdownRecord{})
	return resp.RowsAffected, resp.Error
}

func UpsertCountdownRecord(record *dbTable.CountdownRecord) error {
	return GetDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(record).Error
}
