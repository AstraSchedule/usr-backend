package client

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 边缘缓存（AstraSchedule/esa-edge-cache）靠这两个头判断能否跳过回源直接回 304。
// 头缺失会让边缘完全不生效；过期时刻取错会让客户端在午夜后/规则翻转后继续吃旧课表。
func TestGetSchedule_EdgeCacheHeaders(t *testing.T) {
	ensureTestDB()

	router := setupTestRouter()
	router.GET("/:school/:grade/:class", GetSchedule)

	first := doClientRequest(t, router, http.MethodGet, "/edge-cache/2024/1")
	require.Equal(t, http.StatusOK, first.Code)

	version := first.Header().Get(cacheVersionHeader)
	require.NotEmpty(t, version, "200 响应必须带版本头")
	assert.Equal(t, version, scheduleVersionOf(t, first.Body.Bytes()), "头里的版本必须与响应体一致")

	rawExpire := first.Header().Get(cacheExpireHeader)
	require.NotEmpty(t, rawExpire, "200 响应必须带过期头")
	expire, err := strconv.ParseInt(rawExpire, 10, 64)
	require.NoError(t, err)

	now := time.Now()
	assert.Greater(t, expire, now.Unix(), "过期时刻必须在未来，否则边缘每次都要回源")
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)
	assert.LessOrEqual(t, expire, nextMidnight.Unix(), "过期时刻不得越过下一个本地零点")

	// 304 路径同样要带元信息：边缘回源命中源站 304 时靠它刷新自己的条目
	second := doClientRequest(t, router, http.MethodGet, "/edge-cache/2024/1?version="+version)
	require.Equal(t, http.StatusNotModified, second.Code)
	assert.Equal(t, version, second.Header().Get(cacheVersionHeader))
	assert.Equal(t, rawExpire, second.Header().Get(cacheExpireHeader))
}

func TestScheduleExpireAt_PrefersEarlierBoundary(t *testing.T) {
	now := time.Date(2026, time.September, 20, 15, 30, 0, 0, time.Local)
	midnight := time.Date(2026, time.September, 21, 0, 0, 0, 0, time.Local).Unix()

	assert.Equal(t, midnight, scheduleExpireAt(0, now), "没有自动任务边界时退化为下一个零点")
	assert.Equal(t, midnight, scheduleExpireAt(midnight+3600, now), "晚于零点的边界不采用")
	assert.Equal(t, now.Unix()+600, scheduleExpireAt(now.Unix()+600, now), "早于零点的边界优先")
	assert.Equal(t, midnight, scheduleExpireAt(now.Unix()-600, now), "已经过去的边界不采用")
}
