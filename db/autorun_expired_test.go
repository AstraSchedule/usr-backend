package db

import (
	"AstraScheduleServerGo/model/dbTable"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func autorunRecordOnDateNs(hashID, namespace, date string, disabled bool) dbTable.AutorunRecord {
	return dbTable.AutorunRecord{
		HashID:    hashID,
		Namespace: namespace,
		Name:      hashID,
		EType:     dbTable.AutorunTypeSchedule,
		Scope:     []string{"school/grade/class"},
		Disabled:  disabled,
		Entries: []dbTable.AutorunEntry{{
			ID:   "entry-1",
			When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: date},
			Action: map[string]interface{}{
				"schedule": map[string]interface{}{"periods": []interface{}{}},
			},
		}},
	}
}

// 已过期 = 所有条目都已结束；停用任务不算可清理对象；跨命名空间互不影响
func TestDeleteExpiredAutorunRecordsNs_OnlyExpiredAndEnabled(t *testing.T) {
	cleanupDB(t)
	database := GetDB()
	today := time.Date(2026, time.September, 26, 0, 0, 0, 0, time.Local)

	records := []dbTable.AutorunRecord{
		autorunRecordOnDateNs("expired", "ns1", "2026-09-01", false),
		autorunRecordOnDateNs("expired-disabled", "ns1", "2026-09-01", true),
		autorunRecordOnDateNs("upcoming", "ns1", "2026-10-01", false),
		autorunRecordOnDateNs("other-ns", "ns2", "2026-09-01", false),
	}
	for i := range records {
		require.NoError(t, database.Create(&records[i]).Error)
	}

	deleted, scopes, err := DeleteExpiredAutorunRecordsNs("ns1", today, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)
	assert.Equal(t, []string{"school/grade/class"}, scopes)

	remaining, err := FetchAutorunRecordsNs("ns1", "")
	require.NoError(t, err)
	ids := make([]string, 0, len(remaining))
	for _, record := range remaining {
		ids = append(ids, record.HashID)
	}
	assert.ElementsMatch(t, []string{"expired-disabled", "upcoming"}, ids)

	other, err := FetchAutorunRecordsNs("ns2", "")
	require.NoError(t, err)
	assert.Len(t, other, 1)
}

func TestDeleteExpiredAutorunRecordsNs_EmptyNamespaceKeepsData(t *testing.T) {
	cleanupDB(t)
	database := GetDB()
	record := autorunRecordOnDateNs("expired", "ns1", "2026-09-01", false)
	require.NoError(t, database.Create(&record).Error)

	deleted, scopes, err := DeleteExpiredAutorunRecordsNs("", time.Now(), 0)
	require.NoError(t, err)
	assert.Zero(t, deleted)
	assert.Empty(t, scopes)

	rows, err := FetchAutorunRecordsNs("ns1", "")
	require.NoError(t, err)
	assert.Len(t, rows, 1)
}

// 含停用条目的记录即便已过期也必须保留：TaskStatus 跳过停用条目，
// 删掉整条记录等于丢掉用户停用保存的配置
func TestDeleteExpiredAutorunRecordsNs_KeepsRecordsWithDisabledEntries(t *testing.T) {
	cleanupDB(t)
	database := GetDB()
	today := time.Date(2026, time.September, 26, 0, 0, 0, 0, time.Local)
	record := dbTable.AutorunRecord{
		HashID:    "mixed",
		Namespace: "ns1",
		Name:      "mixed",
		EType:     dbTable.AutorunTypeSchedule,
		Scope:     []string{"school/grade/class"},
		Entries: []dbTable.AutorunEntry{
			{
				ID:     "e1",
				When:   &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: "2026-09-01"},
				Action: map[string]interface{}{"schedule": map[string]interface{}{"periods": []interface{}{}}},
			},
			{
				ID:       "e2",
				Disabled: true,
				When:     &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: "2026-12-01"},
				Action:   map[string]interface{}{"schedule": map[string]interface{}{"periods": []interface{}{}}},
			},
		},
	}
	require.NoError(t, database.Create(&record).Error)

	deleted, scopes, err := DeleteExpiredAutorunRecordsNs("ns1", today, 0)
	require.NoError(t, err)
	assert.Zero(t, deleted)
	assert.Empty(t, scopes)

	rows, err := FetchAutorunRecordsNs("ns1", "")
	require.NoError(t, err)
	assert.Len(t, rows, 1)
}

// 保留期门槛：创建时间仍在保留期内的已过期任务，自动清理不动；手动清理（minAge=0）才清
func TestDeleteExpiredAutorunRecordsNs_KeepsRecordsWithinRetention(t *testing.T) {
	cleanupDB(t)
	database := GetDB()
	now := time.Now()
	record := autorunRecordOnDateNs("fresh", "ns1", "2026-09-01", false)
	require.NoError(t, database.Create(&record).Error)

	deleted, scopes, err := DeleteExpiredAutorunRecordsNs("ns1", now, 30*24*time.Hour)
	require.NoError(t, err)
	assert.Zero(t, deleted)
	assert.Empty(t, scopes)

	manual, _, err := DeleteExpiredAutorunRecordsNs("ns1", now, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), manual)
}

// 跨命名空间清理：每个命名空间单独汇报，供调用方按租户广播
func TestCleanExpiredAutorunRecords_AcrossNamespaces(t *testing.T) {
	cleanupDB(t)
	database := GetDB()
	today := time.Date(2026, time.September, 26, 0, 0, 0, 0, time.Local)
	for _, namespace := range []string{"ns1", "ns2"} {
		record := autorunRecordOnDateNs("expired-"+namespace, namespace, "2026-09-01", false)
		require.NoError(t, database.Create(&record).Error)
	}
	alive := autorunRecordOnDateNs("alive", "ns1", "2026-12-01", false)
	require.NoError(t, database.Create(&alive).Error)

	results, err := CleanExpiredAutorunRecords(today, 0)
	require.NoError(t, err)
	require.Len(t, results, 2)
	total := int64(0)
	for _, result := range results {
		total += result.Deleted
		assert.Equal(t, []string{"school/grade/class"}, result.Scopes)
	}
	assert.Equal(t, int64(2), total)

	rows, err := FetchAutorunRecordsNs("ns1", "")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "alive", rows[0].HashID)
}
