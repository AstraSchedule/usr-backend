package db

import (
	"AstraScheduleServerGo/model/dbTable"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 备份导入的跨租户保护：ID 已属于其它命名空间时必须跳过，绝不覆盖对方记录
func TestImportAutorunRecords_SkipsRowsOwnedByOtherNamespace(t *testing.T) {
	cleanupDB(t)

	// 租户 A 已存在一条记录
	require.NoError(t, GetDB().Create(&dbTable.AutorunRecord{
		HashID: "shared-hash", Namespace: "tenant-a", EType: dbTable.AutorunTypeTimetable,
		Scope: []string{"ALL"}, Level: 1, Status: 1,
		Parameters: map[string]interface{}{"rule": map[string]interface{}{"date": "2026-09-01", "timetableId": "exam-a"}},
	}).Error)

	// 一份「ID 哈希不含 namespace」的旧备份被导入到租户 B：同一个 ID 撞上了 A 的记录
	n, err := importAutorunRecords(GetDB(), []dbTable.AutorunRecord{{
		HashID: "shared-hash", Namespace: "tenant-b", EType: dbTable.AutorunTypeTimetable,
		Scope: []string{"ALL"}, Level: 9, Status: 0,
		Parameters: map[string]interface{}{"rule": map[string]interface{}{"date": "2026-09-02", "timetableId": "exam-b"}},
	}}, "overwrite")
	require.NoError(t, err)
	assert.Equal(t, 0, n, "跨租户的行不应被导入")

	// 租户 A 的记录必须原样保留
	rows, err := FetchAutorunRecordsNs("tenant-a", "shared-hash")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "tenant-a", rows[0].Namespace)
	assert.Equal(t, 1, rows[0].Level, "优先级不应被导入内容覆盖")
	rule, ok := rows[0].Parameters["rule"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "exam-a", rule["timetableId"])
}

// 同命名空间的导入仍然正常覆盖（回归：不要因为保护把正常导入也挡掉）
func TestImportAutorunRecords_OverwritesSameNamespace(t *testing.T) {
	cleanupDB(t)

	require.NoError(t, GetDB().Create(&dbTable.AutorunRecord{
		HashID: "same-hash", Namespace: "tenant-a", EType: dbTable.AutorunTypeTimetable,
		Scope: []string{"ALL"}, Level: 1,
		Parameters: map[string]interface{}{"rule": map[string]interface{}{"date": "2026-09-01", "timetableId": "old"}},
	}).Error)

	n, err := importAutorunRecords(GetDB(), []dbTable.AutorunRecord{{
		HashID: "same-hash", Namespace: "tenant-a", EType: dbTable.AutorunTypeTimetable,
		Scope: []string{"ALL"}, Level: 5,
		Parameters: map[string]interface{}{"rule": map[string]interface{}{"date": "2026-09-01", "timetableId": "new"}},
	}}, "overwrite")
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	rows, err := FetchAutorunRecordsNs("tenant-a", "same-hash")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 5, rows[0].Level)
	rule, ok := rows[0].Parameters["rule"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "new", rule["timetableId"])
}

// 倒数日记录同样是「内容派生 ID + 单列主键」，需要同样的保护
func TestImportCountdownRecords_SkipsRowsOwnedByOtherNamespace(t *testing.T) {
	cleanupDB(t)

	require.NoError(t, GetDB().Create(&dbTable.CountdownRecord{
		ID: "cd-shared", Namespace: "tenant-a", Scope: []string{"ALL"},
		Schedules: []dbTable.CountdownScheduleItem{{Name: "期末", Date: "2026-01-01", Priority: 1}},
	}).Error)

	n, err := importCountdownRecords(GetDB(), []dbTable.CountdownRecord{{
		ID: "cd-shared", Namespace: "tenant-b", Scope: []string{"ALL"},
		Schedules: []dbTable.CountdownScheduleItem{{Name: "高考", Date: "2026-06-07", Priority: 2}},
	}}, "overwrite")
	require.NoError(t, err)
	assert.Equal(t, 0, n)

	rows, err := FetchCountdownRecordsNs("tenant-a", "cd-shared")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "tenant-a", rows[0].Namespace)
	require.Len(t, rows[0].Schedules, 1)
	assert.Equal(t, "期末", rows[0].Schedules[0].Name)
}
