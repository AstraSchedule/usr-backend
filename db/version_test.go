package db

import (
	"testing"
	"time"

	"AstraScheduleServerGo/model/dbTable"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 零值 time.Time 的 Unix() 是 -62135596800（0001-01-01）。历史上它被直接当作版本下发，
// 客户端原样回传后比较恒等，304 会把旧配置一直钉住，同时离线缓存也把它判为非法版本。
func TestLatestTimestamp_ZeroTimeBecomesZero(t *testing.T) {
	assert.Equal(t, int64(0), LatestTimestamp())
	assert.Equal(t, int64(0), LatestTimestamp(time.Time{}, time.Time{}))
}

func TestLatestTimestamp_PicksNewest(t *testing.T) {
	older := time.Unix(1700000000, 0)
	newer := time.Unix(1800000000, 0)

	assert.Equal(t, int64(1800000000), LatestTimestamp(older, newer))
	assert.Equal(t, int64(1800000000), LatestTimestamp(newer, older, time.Time{}))
}

// 没有任何 data_versions 行时（只用管理端配过课表的班级），版本必须是 0 而不是负时间戳
func TestGetLatestVersion_MissingRowIsNotNegative(t *testing.T) {
	cleanupDB(t)

	assert.Equal(t, int64(0), GetLatestVersion("no-such-school", "2024", "1").Timestamp())
}

// 自动任务的 status 只是管理端列表用的派生缓存：客户端响应在读取时按时间重新求值，
// 不依赖该列。因此状态翻转不得推进 UpdatedAt，否则每次翻转都会推进课表版本、
// 让所有客户端多做一次无意义的重新拉取。
func TestRefreshAutorunStatuses_KeepsUpdatedAt(t *testing.T) {
	cleanupDB(t)

	stale := time.Now().Add(-24 * time.Hour).Truncate(time.Second)
	record := dbTable.AutorunRecord{
		HashID:    "version-status-flip",
		EType:     dbTable.AutorunTypeCompensation,
		Scope:     []string{"school/2024/1"},
		Status:    2, // 与按时间推导的结果（无启用条目 -> 0）不同，确保触发一次状态写入
		CreatedAt: stale,
		UpdatedAt: stale,
	}
	require.NoError(t, GetDB().Create(&record).Error)

	updated, err := RefreshAutorunStatuses(time.Now())
	require.NoError(t, err)
	assert.Equal(t, int64(1), updated, "状态应从 2 翻转为 0")

	var after dbTable.AutorunRecord
	require.NoError(t, GetDB().Where("hash_id = ?", record.HashID).Take(&after).Error)
	assert.Equal(t, 0, after.Status)
	assert.True(t, after.UpdatedAt.Equal(stale), "状态翻转不得改动 UpdatedAt")
}
