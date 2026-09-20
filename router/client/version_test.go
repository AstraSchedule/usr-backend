package client

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model/dbTable"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 管理端保存课表（PUT /web/config/:school/:grade/:class/schedule）只写 schedules 行、
// 不写 data_versions 行。版本必须能从数据行自身的 UpdatedAt 推进，否则客户端收到
// SyncConfig 后重拉会命中 304，界面一直停在旧课表（线上真实故障）。
func TestGetSchedule_VersionFollowsDataUpdates(t *testing.T) {
	ensureTestDB()
	router := setupTestRouter()
	router.GET("/:school/:grade/:class", GetSchedule)

	const school, grade, class = "version-school", "2024", "1"
	require.NoError(t, db.GetDB().Create(&dbTable.Schedule{
		Namespace: "default", School: school, Grade: grade, Class: class,
	}).Error)

	// 首次拉取：没有 data_versions 行，版本的数据部分必须是 0，而不是零值时间的负时间戳
	first := doClientRequest(t, router, http.MethodGet, "/"+school+"/"+grade+"/"+class)
	require.Equal(t, http.StatusOK, first.Code)
	version := scheduleVersionOf(t, first.Body.Bytes())
	require.NotContains(t, version, "-", "数据版本不得为负数")

	// 客户端带着同一版本再来：数据没变，缓存必须生效（304）
	same := doClientRequest(t, router, http.MethodGet, "/"+school+"/"+grade+"/"+class+"?version="+version)
	assert.Equal(t, http.StatusNotModified, same.Code)

	// 管理端改动课表：与 PutScheduleConfig 等效地更新 schedules 行，GORM 会推进 UpdatedAt。
	// 跨秒等待模拟 MySQL 的 datetime 精度下限（同一秒内的两次写入时间戳可能相同）。
	time.Sleep(1100 * time.Millisecond)
	var row dbTable.Schedule
	require.NoError(t, db.GetDB().Where("namespace = ? AND school = ? AND grade = ? AND class = ?", "default", school, grade, class).Take(&row).Error)
	row.DailyClasses[0] = dbTable.DailyClass{Chinese: "一", English: "MON"}
	require.NoError(t, db.GetDB().Save(&row).Error)

	// 关键断言：数据变了就必须重新下发，不能再 304
	after := doClientRequest(t, router, http.MethodGet, "/"+school+"/"+grade+"/"+class+"?version="+version)
	assert.Equal(t, http.StatusOK, after.Code, "课表数据更新后必须返回 200 而不是 304")
	assert.NotEqual(t, version, scheduleVersionOf(t, after.Body.Bytes()))
}

// 自动任务规则的增删改不写任何数据行，只能靠记录自身的 UpdatedAt 推进版本
func TestGetSchedule_VersionFollowsAutorunRecordEdits(t *testing.T) {
	ensureTestDB()
	router := setupTestRouter()
	router.GET("/:school/:grade/:class", GetSchedule)

	const school, grade, class = "version-autorun", "2024", "2"
	require.NoError(t, db.GetDB().Create(&dbTable.Schedule{Namespace: "default", School: school, Grade: grade, Class: class}).Error)

	first := doClientRequest(t, router, http.MethodGet, "/"+school+"/"+grade+"/"+class)
	require.Equal(t, http.StatusOK, first.Code)
	version := scheduleVersionOf(t, first.Body.Bytes())

	time.Sleep(1100 * time.Millisecond)
	require.NoError(t, db.GetDB().Create(&dbTable.AutorunRecord{
		Namespace: "default",
		EType:     1,
		Scope:     []string{school + "/" + grade + "/" + class},
		Entries:   []dbTable.AutorunEntry{{}},
		UpdatedAt: time.Now(),
	}).Error)

	after := doClientRequest(t, router, http.MethodGet, "/"+school+"/"+grade+"/"+class+"?version="+version)
	assert.Equal(t, http.StatusOK, after.Code, "自动任务规则改动后必须返回 200 而不是 304")
}

func scheduleVersionOf(t *testing.T, body []byte) string {
	t.Helper()
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &payload))
	version, ok := payload["version"].(string)
	require.True(t, ok, "响应里必须有字符串 version")
	return version
}
