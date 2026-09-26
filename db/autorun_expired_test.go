package db

import (
	"AstraScheduleServerGo/model/dbTable"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func autorunRecordOnDate(hashID, date string, disabled bool) dbTable.AutorunRecord {
	return dbTable.AutorunRecord{
		HashID:   hashID,
		Name:     hashID,
		EType:    dbTable.AutorunTypeSchedule,
		Scope:    []string{"school/grade/class"},
		Disabled: disabled,
		Entries: []dbTable.AutorunEntry{{
			ID:   "entry-1",
			When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: date},
			Action: map[string]interface{}{
				"schedule": map[string]interface{}{"periods": []interface{}{}},
			},
		}},
	}
}

// 已过期 = 所有条目都已结束；停用任务不算可清理对象
func TestDeleteExpiredAutorunRecords_OnlyExpiredAndEnabled(t *testing.T) {
	cleanupDB(t)
	database := GetDB()
	today := time.Date(2026, time.September, 26, 0, 0, 0, 0, time.Local)

	records := []dbTable.AutorunRecord{
		autorunRecordOnDate("expired", "2026-09-01", false),
		autorunRecordOnDate("expired-disabled", "2026-09-01", true),
		autorunRecordOnDate("upcoming", "2026-10-01", false),
	}
	for i := range records {
		require.NoError(t, database.Create(&records[i]).Error)
	}

	deleted, scopes, err := DeleteExpiredAutorunRecords(today)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)
	assert.Equal(t, []string{"school/grade/class"}, scopes)

	remaining, err := FetchAutorunRecords("")
	require.NoError(t, err)
	ids := make([]string, 0, len(remaining))
	for _, record := range remaining {
		ids = append(ids, record.HashID)
	}
	assert.ElementsMatch(t, []string{"expired-disabled", "upcoming"}, ids)
}

func TestDeleteExpiredAutorunRecords_NothingToClean(t *testing.T) {
	cleanupDB(t)
	database := GetDB()
	future := autorunRecordOnDate("future", "2026-12-01", false)
	require.NoError(t, database.Create(&future).Error)

	deleted, scopes, err := DeleteExpiredAutorunRecords(time.Date(2026, time.September, 26, 0, 0, 0, 0, time.Local))
	require.NoError(t, err)
	assert.Zero(t, deleted)
	assert.Empty(t, scopes)

	rows, err := FetchAutorunRecords("")
	require.NoError(t, err)
	assert.Len(t, rows, 1)
}
