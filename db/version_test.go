package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
