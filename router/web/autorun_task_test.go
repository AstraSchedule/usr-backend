package web

import (
	"encoding/json"
	"net/http"
	"testing"

	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/middleware"
	"AstraScheduleServerGo/model/dbTable"
	"AstraScheduleServerGo/service"
	"AstraScheduleServerGo/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scopedUser 写入指定作用域的非 admin 用户 claims，用于验证统一任务接口的作用域校验
func scopedUser(t *testing.T, username, scope string) gin.HandlerFunc {
	t.Helper()
	user := testutil.CreateUser(t, db.GetDB(), username, "test123", "school_w", scope)
	claims := &service.JWTClaims{UserID: user.ID, Username: user.Username, Role: user.Role, Scope: user.Scope}
	return func(c *gin.Context) {
		c.Set(middleware.UserClaimsKey, claims)
		c.Next()
	}
}

func weeklyEntry(id string, everyWeeks, offset int, timetableID string) map[string]interface{} {
	return map[string]interface{}{
		"id": id,
		"when": map[string]interface{}{
			"kind": "weekly", "everyWeeks": everyWeeks, "weekOffset": offset,
		},
		"action": map[string]interface{}{"timetableId": timetableID},
	}
}

func taskRouter(t *testing.T) *gin.Engine {
	t.Helper()
	router := setupTestRouter()
	router.PUT("/web/autorun/task", adminOnly(t), PutAutorunTask)
	router.GET("/web/autorun/hash/:hashid", GetAutorunHashStatus)
	router.GET("/web/autorun", GetAutorunStatus)
	return router
}

func timetableTaskBody(entries []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"name": "轮换作息", "type": dbTable.AutorunTypeTimetable, "scope": []string{"ALL"},
		"priority": 10, "entries": entries,
	}
}

func TestPutAutorunTask_WeeklyRotation(t *testing.T) {
	ensureTestDB()
	router := taskRouter(t)

	body := timetableTaskBody([]map[string]interface{}{
		weeklyEntry("e1", 2, 0, "exam"),
		weeklyEntry("e2", 2, 1, "常日"),
	})
	w := doRequest(t, router, "PUT", "/web/autorun/task", body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	item := fetchAutorunDetail(t, router, w)
	assert.Equal(t, "轮换作息", item["name"])
	assert.Equal(t, "TIMETABLE", item["type"])
	assert.Equal(t, true, item["enabled"])

	entries, ok := item["entries"].([]interface{})
	require.True(t, ok, "详情应返回 entries 数组")
	require.Len(t, entries, 2)
	first, ok := entries[0].(map[string]interface{})
	require.True(t, ok)
	when, ok := first["when"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "weekly", when["kind"])
	assert.Equal(t, float64(2), when["everyWeeks"])
	// v1 契约：content 取首条条目的内容
	assert.Equal(t, "exam", autorunContent(t, item)["timetableId"])
}

func TestPutAutorunTask_ClientConfigAndDisabledTask(t *testing.T) {
	ensureTestDB()
	router := taskRouter(t)

	body := map[string]interface{}{
		"name": "考试周置顶", "type": dbTable.AutorunTypeClientConfig, "scope": []string{"ALL"},
		"priority": 3, "enabled": false,
		"entries": []map[string]interface{}{
			{
				"when":   map[string]interface{}{"kind": "event", "event": "class_start", "period": 1},
				"action": map[string]interface{}{"settings": map[string]interface{}{"isWindowAlwaysOnTop": true, "isDuringClassHidden": false}},
			},
		},
	}
	w := doRequest(t, router, "PUT", "/web/autorun/task", body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	item := fetchAutorunDetail(t, router, w)
	assert.Equal(t, "CLIENT_CONFIG", item["type"])
	assert.Equal(t, false, item["enabled"])
	entries := item["entries"].([]interface{})
	require.Len(t, entries, 1)
	entry := entries[0].(map[string]interface{})
	assert.Equal(t, true, entry["enabled"], "条目 enabled 缺省为 true")
	action, ok := entry["action"].(map[string]interface{})
	require.True(t, ok)
	settings, ok := action["settings"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, settings["isWindowAlwaysOnTop"])
}

func TestPutAutorunTask_InvalidPayloads(t *testing.T) {
	ensureTestDB()
	router := taskRouter(t)

	clientConfigBody := func(settings map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{
			"type": dbTable.AutorunTypeClientConfig, "scope": []string{"ALL"},
			"entries": []map[string]interface{}{{"action": map[string]interface{}{"settings": settings}}},
		}
	}
	whenBody := func(when map[string]interface{}) map[string]interface{} {
		return timetableTaskBody([]map[string]interface{}{{"when": when, "action": map[string]interface{}{"timetableId": "exam"}}})
	}

	cases := []struct {
		name string
		body map[string]interface{}
	}{
		{"空条目", timetableTaskBody([]map[string]interface{}{})},
		{"类型越界", map[string]interface{}{"type": 99, "scope": []string{"ALL"}, "entries": []map[string]interface{}{weeklyEntry("e1", 2, 0, "exam")}}},
		{"作息表为空", timetableTaskBody([]map[string]interface{}{{"action": map[string]interface{}{"timetableId": ""}}})},
		{"单日日期非法", whenBody(map[string]interface{}{"kind": "date", "date": "not-a-date"})},
		{"范围起始日非法", whenBody(map[string]interface{}{"kind": "range", "startDate": "x", "endDate": "2026-09-01"})},
		{"范围结束日非法", whenBody(map[string]interface{}{"kind": "range", "startDate": "2026-09-01", "endDate": "x"})},
		{"范围起止倒置", whenBody(map[string]interface{}{"kind": "range", "startDate": "2026-09-10", "endDate": "2026-09-01"})},
		{"周期周数非法", whenBody(map[string]interface{}{"kind": "weekly", "everyWeeks": 0})},
		{"周期偏移越界", whenBody(map[string]interface{}{"kind": "weekly", "everyWeeks": 2, "weekOffset": 2})},
		{"周期起始日非法", whenBody(map[string]interface{}{"kind": "weekly", "everyWeeks": 2, "startDate": "x"})},
		{"周期结束日非法", whenBody(map[string]interface{}{"kind": "weekly", "everyWeeks": 2, "endDate": "x"})},
		{"cron 非法", whenBody(map[string]interface{}{"kind": "cron", "cron": "bad"})},
		{"cron 负数时长", whenBody(map[string]interface{}{"kind": "cron", "cron": "0 8 * * *", "duration": -1})},
		{"事件不支持", whenBody(map[string]interface{}{"kind": "event", "event": "unknown"})},
		{"事件节次非法", whenBody(map[string]interface{}{"kind": "event", "event": "class_start", "period": 0})},
		{"条件类型不支持", whenBody(map[string]interface{}{"kind": "unknown"})},
		{"星期越界", whenBody(map[string]interface{}{"kind": "date", "date": "2026-09-01", "weekdays": []int{9}})},
		{"空内容", timetableTaskBody([]map[string]interface{}{{"action": map[string]interface{}{}}})},
		{"客户端配置项不支持", clientConfigBody(map[string]interface{}{"isUnknown": true})},
		{"客户端配置值非布尔", clientConfigBody(map[string]interface{}{"isAlwaysMinimized": "yes"})},
		{"客户端配置为空对象", clientConfigBody(map[string]interface{}{})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doRequest(t, router, "PUT", "/web/autorun/task", tc.body)
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
}

func TestPutAutorunTask_ScheduleAndCompensation(t *testing.T) {
	ensureTestDB()
	router := taskRouter(t)

	scheduleBody := map[string]interface{}{
		"type": dbTable.AutorunTypeSchedule, "scope": []string{"ALL"}, "priority": 1,
		"entries": []map[string]interface{}{
			{
				"when": map[string]interface{}{"kind": "range", "startDate": "2026-09-07", "endDate": "2026-09-11"},
				"action": map[string]interface{}{"schedule": map[string]interface{}{"periods": []interface{}{
					map[string]interface{}{"no": 1, "subject": "期末"},
				}}},
			},
		},
	}
	w := doRequest(t, router, "PUT", "/web/autorun/task", scheduleBody)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	allBody := map[string]interface{}{
		"type": dbTable.AutorunTypeAll, "scope": []string{"ALL"}, "priority": 1,
		"entries": []map[string]interface{}{
			{
				"action": map[string]interface{}{
					"timetableId": "exam",
					"schedule":    map[string]interface{}{"periods": []interface{}{map[string]interface{}{"no": 1, "subject": "考试"}}},
				},
			},
		},
	}
	w = doRequest(t, router, "PUT", "/web/autorun/task", allBody)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	compensationBody := map[string]interface{}{
		"type": dbTable.AutorunTypeCompensation, "scope": []string{"ALL"}, "priority": 1,
		"entries": []map[string]interface{}{
			{
				"when":   map[string]interface{}{"kind": "date", "date": "2026-10-02"},
				"action": map[string]interface{}{"useDate": "2026-09-29"},
			},
		},
	}
	w = doRequest(t, router, "PUT", "/web/autorun/task", compensationBody)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	item := fetchAutorunDetail(t, router, w)
	content := autorunContent(t, item)
	assert.Equal(t, "2026-10-02", content["date"], "v1 契约：单日条目的日期并回 content")
	assert.Equal(t, "2026-09-29", content["useDate"])
}

func TestPutAutorunTask_UpdateKeepsIDAndCreatedAt(t *testing.T) {
	ensureTestDB()
	router := taskRouter(t)

	body := timetableTaskBody([]map[string]interface{}{weeklyEntry("e1", 2, 0, "exam")})
	body["id"] = "fixed-task-id"
	w := doRequest(t, router, "PUT", "/web/autorun/task", body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var created dbTable.AutorunRecord
	require.NoError(t, db.GetDB().Where("hash_id = ?", "fixed-task-id").First(&created).Error)

	body["entries"] = []map[string]interface{}{weeklyEntry("e1", 2, 1, "常日")}
	w = doRequest(t, router, "PUT", "/web/autorun/task", body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var updated dbTable.AutorunRecord
	require.NoError(t, db.GetDB().Where("hash_id = ?", "fixed-task-id").First(&updated).Error)
	assert.Equal(t, created.CreatedAt.Unix(), updated.CreatedAt.Unix(), "更新不应重置创建时间")
	require.Len(t, updated.Entries, 1)
	assert.Equal(t, 1, updated.Entries[0].When.WeekOffset)
}

func TestPutAutorunTask_ScopePermission(t *testing.T) {
	ensureTestDB()

	router := setupTestRouter()
	router.PUT("/web/autorun/task", scopedUser(t, "autorun-scope-user", "s1"), PutAutorunTask)

	allowed := map[string]interface{}{
		"type": dbTable.AutorunTypeTimetable, "scope": []string{"s1/g1"}, "priority": 1,
		"entries": []map[string]interface{}{weeklyEntry("e1", 2, 0, "exam")},
	}
	w := doRequest(t, router, "PUT", "/web/autorun/task", allowed)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	for name, scope := range map[string][]string{"他校作用域": {"s2/g1"}, "ALL 级规则": {"ALL"}} {
		denied := map[string]interface{}{
			"type": dbTable.AutorunTypeTimetable, "scope": scope, "priority": 1,
			"entries": []map[string]interface{}{weeklyEntry("e1", 2, 0, "exam")},
		}
		w = doRequest(t, router, "PUT", "/web/autorun/task", denied)
		assert.Equal(t, http.StatusForbidden, w.Code, name)
	}
}

func TestGetAutorunStatus_IncludesEntriesAndName(t *testing.T) {
	ensureTestDB()

	require.NoError(t, db.GetDB().Save(&dbTable.AutorunRecord{
		HashID: "entries-hash", Name: "整周替换", EType: dbTable.AutorunTypeTimetable, Scope: []string{"ALL"}, Level: 1,
		Entries: []dbTable.AutorunEntry{{
			ID: "e1", When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenRange, StartDate: "2026-09-07", EndDate: "2026-09-11"},
			Action: map[string]interface{}{"timetableId": "exam"},
		}},
	}).Error)

	router := setupTestRouter()
	router.GET("/web/autorun", GetAutorunStatus)

	w := doRequest(t, router, "GET", "/web/autorun", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].([]interface{})
	require.NotEmpty(t, data)

	var found map[string]interface{}
	for _, raw := range data {
		item := raw.(map[string]interface{})
		if item["id"] == "entries-hash" {
			found = item
		}
	}
	require.NotNil(t, found, "列表中应包含刚写入的任务")
	assert.Equal(t, "整周替换", found["name"])
	assert.Equal(t, true, found["enabled"])
	entries, ok := found["entries"].([]interface{})
	require.True(t, ok)
	require.Len(t, entries, 1)
}
