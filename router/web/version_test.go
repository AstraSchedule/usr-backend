package web

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"AstraScheduleServerGo/router/client"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 管理端保存课表 → 客户端必须能拿到新课表。
// 这条链路此前是断的：PutScheduleConfig 只写 schedules 行、不推进 data_versions，
// 客户端收到 SyncConfig 重拉时命中 304，界面一直停在旧课表（线上真实故障）。
func TestPutScheduleConfig_AdvancesClientVersion(t *testing.T) {
	ensureTestDB()

	router := setupTestRouter()
	router.PUT("/web/config/:school/:grade/:class_number/schedule", withClaims(PutScheduleConfig))
	router.GET("/:school/:grade/:class", client.GetSchedule)

	const school, grade, class = "version-e2e", "2024", "3"
	dailyClassBody := func(subject string) map[string]interface{} {
		days := []interface{}{}
		for _, name := range []string{"日", "一", "二", "三", "四", "五", "六"} {
			days = append(days, map[string]interface{}{
				"Chinese":   name,
				"English":   "X",
				"timetable": "常日",
				"classList": []interface{}{[]interface{}{subject}},
			})
		}
		return map[string]interface{}{"daily_class": days}
	}
	schedulePath := "/web/config/" + school + "/" + grade + "/" + class + "/schedule"
	clientPath := "/" + school + "/" + grade + "/" + class

	// 首次保存并让客户端拉一次，取到当前版本
	require.Equal(t, http.StatusOK, doRequest(t, router, "PUT", schedulePath, dailyClassBody("数")).Code)
	first := doRequest(t, router, "GET", clientPath, nil)
	require.Equal(t, http.StatusOK, first.Code)
	version := clientVersionOf(t, first.Body.Bytes())

	// 未改动时缓存生效
	assert.Equal(t, http.StatusNotModified, doRequest(t, router, "GET", clientPath+"?version="+version, nil).Code)

	// 管理端再次保存（换了课程）：必须推进版本，客户端重拉拿到 200
	// 跨秒等待模拟 MySQL datetime 精度下限
	time.Sleep(1100 * time.Millisecond)
	require.Equal(t, http.StatusOK, doRequest(t, router, "PUT", schedulePath, dailyClassBody("语")).Code)

	after := doRequest(t, router, "GET", clientPath+"?version="+version, nil)
	assert.Equal(t, http.StatusOK, after.Code, "管理端改动课表后客户端必须收到 200 而不是 304")
	assert.NotEqual(t, version, clientVersionOf(t, after.Body.Bytes()))
}

func clientVersionOf(t *testing.T, body []byte) string {
	t.Helper()
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &payload))
	version, ok := payload["version"].(string)
	require.True(t, ok, "客户端响应里必须有字符串 version")
	return version
}
