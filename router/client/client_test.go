package client

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model"
	"AstraScheduleServerGo/model/dbTable"
	"AstraScheduleServerGo/testutil"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testDBInitialized = false

func ensureTestDB() {
	if testDBInitialized {
		return
	}
	testutil.InitTestDB()
	db.GetDB().AutoMigrate(
		&dbTable.Schedule{},
		&dbTable.ClientConfig{},
		&dbTable.Timetable{},
		&dbTable.Subject{},
		&dbTable.DataVersion{},
		&dbTable.AutorunRecord{},
		&dbTable.CountdownRecord{},
	)
	testDBInitialized = true
}

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	return router
}

// doClientRequest 在 router 上执行无请求体的请求并返回 recorder，消除重复样板。
func doClientRequest(t *testing.T, router *gin.Engine, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req, err := http.NewRequest(method, path, nil)
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
		return nil
	}
	router.ServeHTTP(w, req)
	return w
}

// GetSchedule tests

func TestGetSchedule_Empty(t *testing.T) {
	ensureTestDB()

	router := setupTestRouter()
	router.GET("/:school/:grade/:class", GetSchedule)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/school1/grade1/class1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 契约：响应必须包含客户端消费的全部顶层字段（desktop/js/index.js 与 renderer.js 依赖）
	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp, "supportWebSocket")
	assert.Contains(t, resp, "version")
	assert.Contains(t, resp, "daily_class")
	assert.Contains(t, resp, "countdown_target")
	assert.Contains(t, resp, "startup_behavior")
	assert.Contains(t, resp, "banner_text")
	assert.Contains(t, resp, "css_style")
	assert.Contains(t, resp, "temperature_colors")
	assert.Contains(t, resp, "timetable")
	assert.Contains(t, resp, "divider")
	assert.Contains(t, resp, "subject_name")
	assert.Contains(t, resp, "countdown_records")

	// daily_class 必须为 7 项的扁平化数组（desktop 按 index 取星期几）
	dailyClass, ok := resp["daily_class"].([]interface{})
	assert.True(t, ok, "daily_class 应为数组")
	assert.Equal(t, 7, len(dailyClass))
	for _, day := range dailyClass {
		d, ok := day.(map[string]interface{})
		assert.True(t, ok)
		assert.Contains(t, d, "Chinese")
		assert.Contains(t, d, "English")
		assert.Contains(t, d, "classList")
		assert.Contains(t, d, "timetable")
	}

	// 空数据时嵌套 map 应为空对象而非 null（客户端解构不崩溃）
	assert.IsType(t, map[string]interface{}{}, resp["timetable"])
	assert.IsType(t, map[string]interface{}{}, resp["divider"])
	assert.IsType(t, map[string]interface{}{}, resp["subject_name"])
	assert.IsType(t, []interface{}{}, resp["countdown_records"])
}

func TestGetSchedule_DataContract(t *testing.T) {
	ensureTestDB()

	database := db.GetDB()
	// 使用独立 scope，避免与其它测试共享数据
	database.Save(&dbTable.Schedule{
		School: "contract", Grade: "2024", Class: "1",
		DailyClasses: [7]dbTable.DailyClass{
			{Timetable: "常日", ClassList: dbTable.ClassList{{"数", "代"}, {"语"}, {"英"}}},
		},
	})
	database.Save(&dbTable.Subject{
		School: "contract", Grade: "2024",
		SubjectConfig: dbTable.SubjectConfig{SubjectName: map[string]string{"数": "数学", "语": "语文"}},
	})
	database.Save(&dbTable.Timetable{
		School: "contract", Grade: "2024",
		TimetableConfig: dbTable.TimetableConfig{
			Timetable: map[string]map[string]interface{}{"常日": {"早上1": 1, "早上2": 2}},
			Divider:   map[string][]int{"常日": {1}},
		},
	})
	database.Save(&dbTable.CountdownRecord{
		ID: "contract-cd", Scope: []string{"ALL"},
		Schedules: []dbTable.CountdownScheduleItem{{Name: "期末", Date: "2026-01-01", Priority: 1}},
	})

	router := setupTestRouter()
	router.GET("/:school/:grade/:class", GetSchedule)

	w := doClientRequest(t, router, "GET", "/contract/2024/1")

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	// 多周轮换：week 1 取第一个课程，扁平化 classList
	dailyClass := resp["daily_class"].([]interface{})
	day0 := dailyClass[0].(map[string]interface{})
	assert.Equal(t, "常日", day0["timetable"])
	assert.Equal(t, []interface{}{"数", "语", "英"}, day0["classList"])

	// 同一周内数据版本未变化时仍应返回 304。
	v := resp["version"].(string)
	w2 := doClientRequest(t, router, "GET", "/contract/2024/1?version="+v)
	assert.Equal(t, http.StatusNotModified, w2.Code)

	// subject_name / timetable / divider 映射
	assert.Equal(t, "数学", resp["subject_name"].(map[string]interface{})["数"])
	assert.Contains(t, resp["timetable"].(map[string]interface{}), "常日")
	assert.Equal(t, []interface{}{float64(1)}, resp["divider"].(map[string]interface{})["常日"])

	// 倒数日按 scope 过滤后返回
	countdowns := resp["countdown_records"].([]interface{})
	assert.Equal(t, 1, len(countdowns))
}

func TestScheduleVersionChangesAcrossWeeks(t *testing.T) {
	dataVersion := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC).Unix()
	assert.NotEqual(t, scheduleVersion(dataVersion, 1, 0), scheduleVersion(dataVersion, 2, 0))
	assert.NotEqual(t, scheduleVersion(100, 2, 0), scheduleVersion(101, 1, 0))
	// 变化点（下一次配置变更时刻）同样参与版本比较
	assert.NotEqual(t, scheduleVersion(100, 2, 1000), scheduleVersion(100, 2, 2000))

	parsedDataVersion, parsedWeekNumber, parsedBoundary, err := parseScheduleVersion(scheduleVersion(100, 2, 0))
	assert.NoError(t, err)
	assert.Equal(t, int64(100), parsedDataVersion)
	assert.Equal(t, 2, parsedWeekNumber)
	assert.Equal(t, int64(0), parsedBoundary)

	_, _, parsedBoundary, err = parseScheduleVersion(scheduleVersion(100, 2, 42))
	assert.NoError(t, err)
	assert.Equal(t, int64(42), parsedBoundary)

	_, _, _, err = parseScheduleVersion("1:2:3:4")
	assert.Error(t, err)
	_, _, _, err = parseScheduleVersion("1:2:not-a-number")
	assert.Error(t, err)
}

// 有后续变化点的自动任务（这里用「明天生效的单日调休」）会把变化点写进版本，
// 客户端越过该时刻后版本必然不同，从而在周内拿到新配置而不是一直吃 304。
func TestGetSchedule_BoundaryInvalidatesCache(t *testing.T) {
	ensureTestDB()

	now := time.Now()
	database := db.GetDB()
	database.Save(&dbTable.DataVersion{
		School: "dyn", Grade: "2024", Class: "1",
		Version: now,
	})
	database.Save(&dbTable.AutorunRecord{
		HashID: "dyn-rule", EType: dbTable.AutorunTypeTimetable, Scope: []string{"dyn"}, Level: 1,
		Entries: []dbTable.AutorunEntry{{
			ID: "e1", When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: now.AddDate(0, 0, 1).Format("2006-01-02")},
			Action: map[string]interface{}{"timetableId": "exam"},
		}},
	})

	router := setupTestRouter()
	router.GET("/:school/:grade/:class", GetSchedule)

	w := doClientRequest(t, router, "GET", "/dyn/2024/1")
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	version, _ := resp["version"].(string)
	assert.Regexp(t, `^\d+:\d+:\d+$`, version, "存在后续变化点时版本应带变化点")
}

// 已过期的单日规则不会再改变课表：版本里不应出现变化点，否则历史任务会永久破坏 304 缓存
func TestGetSchedule_ExpiredRuleKeepsCache(t *testing.T) {
	ensureTestDB()

	now := time.Now()
	database := db.GetDB()
	database.Save(&dbTable.DataVersion{
		School: "cached", Grade: "2024", Class: "1",
		Version: now,
	})
	database.Save(&dbTable.AutorunRecord{
		HashID: "expired-rule", EType: dbTable.AutorunTypeTimetable, Scope: []string{"cached"}, Level: 1,
		Entries: []dbTable.AutorunEntry{{
			ID: "e1", When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: now.AddDate(0, 0, -30).Format("2006-01-02")},
			Action: map[string]interface{}{"timetableId": "exam"},
		}},
	})

	router := setupTestRouter()
	router.GET("/:school/:grade/:class", GetSchedule)

	w := doClientRequest(t, router, "GET", "/cached/2024/1")
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	version, _ := resp["version"].(string)
	assert.Regexp(t, `^\d+:\d+$`, version, "过期规则不应产生变化点")

	// 再请求一次仍然命中 304
	w2 := doClientRequest(t, router, "GET", "/cached/2024/1?version="+version)
	assert.Equal(t, http.StatusNotModified, w2.Code)
}

func TestGetSchedule_NotModified(t *testing.T) {
	ensureTestDB()

	database := db.GetDB()
	database.Save(&dbTable.DataVersion{
		School: "nms", Grade: "2024", Class: "1",
		Version: time.Now(),
	})

	router := setupTestRouter()
	router.GET("/:school/:grade/:class", GetSchedule)

	// 不带 version -> 200
	w := doClientRequest(t, router, "GET", "/nms/2024/1")
	assert.Equal(t, http.StatusOK, w.Code)

	// 带与服务器一致的 version -> 304（desktop 依赖增量同步）
	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	v := resp["version"].(string)

	w2 := doClientRequest(t, router, "GET", "/nms/2024/1?version="+v)
	assert.Equal(t, http.StatusNotModified, w2.Code)
}

func TestGetSchedule_WithVersion(t *testing.T) {
	ensureTestDB()

	router := setupTestRouter()
	router.GET("/:school/:grade/:class", GetSchedule)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/school1/grade1/class1?version=0", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetSchedule_InvalidVersion(t *testing.T) {
	ensureTestDB()

	router := setupTestRouter()
	router.GET("/:school/:grade/:class", GetSchedule)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/school1/grade1/class1?version=invalid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// GetWeather tests — 使用 mock HTTP server 替代真实和风天气 API

func setupMockWeatherServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// 城市查询
	mux.HandleFunc("/geo/v2/city/lookup", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": "200",
			"location": []map[string]interface{}{
				{"id": "101010100", "lat": "39.904", "lon": "116.407", "name": "北京"},
			},
		})
	})

	// 实时天气
	mux.HandleFunc("/v7/weather/now", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"now": map[string]string{
				"temp": "25", "text": "晴", "windDir": "北风", "windScale": "3",
			},
		})
	})

	// 天气预警
	mux.HandleFunc("/weatheralert/v1/current/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"alerts": []interface{}{},
		})
	})

	// TLS mock server — 生产代码硬编码 https://
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	// 注入跳过 TLS 验证的 resty 客户端
	origFactory := newRestyClient
	newRestyClient = func() *resty.Client {
		c := resty.New().SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
		return c
	}
	t.Cleanup(func() { newRestyClient = origFactory })

	return srv
}

func TestGetWeatherWithProvince_Success(t *testing.T) {
	ensureTestDB()
	mock := setupMockWeatherServer(t)

	// 指向 mock server（去掉 scheme）
	origHost := model.Configs.APIKey.APIHost
	model.Configs.APIKey.APIHost = strings.TrimPrefix(mock.URL, "https://")
	t.Cleanup(func() { model.Configs.APIKey.APIHost = origHost })

	router := setupTestRouter()
	router.GET("/api/weather/:name1/:name2", GetWeatherWithProvince)

	w := doClientRequest(t, router, "GET", "/api/weather/北京/朝阳")

	assert.Equal(t, http.StatusOK, w.Code)
	var resp model.WeatherResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "北京", resp.Where)
	assert.Equal(t, "25", resp.Temp)
	assert.Equal(t, "晴", resp.Weat)
}

func TestGetWeatherWithCity_Success(t *testing.T) {
	ensureTestDB()
	mock := setupMockWeatherServer(t)

	origHost := model.Configs.APIKey.APIHost
	model.Configs.APIKey.APIHost = strings.TrimPrefix(mock.URL, "https://")
	t.Cleanup(func() { model.Configs.APIKey.APIHost = origHost })

	router := setupTestRouter()
	router.GET("/api/weather/:name1", GetWeatherWithCity)

	w := doClientRequest(t, router, "GET", "/api/weather/北京")

	assert.Equal(t, http.StatusOK, w.Code)
	var resp model.WeatherResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "北京", resp.Where)
	assert.Equal(t, "25", resp.Temp)
}

func TestGetWeatherWithCFHeader_NoCFHeader(t *testing.T) {
	ensureTestDB()

	router := setupTestRouter()
	router.GET("/api/weather/", GetWeatherWithCFHeader)

	w := doClientRequest(t, router, "GET", "/api/weather/")

	// 没有 CF-IPCity 头时应返回 400
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetWeatherWithCFHeader_NoCredential(t *testing.T) {
	ensureTestDB()

	// 未配置天气认证时返回 403（不发起上游请求，也不计入统计）
	origAPIKey := model.Configs.APIKey
	model.Configs.APIKey = model.APIKeyConfig{}
	t.Cleanup(func() { model.Configs.APIKey = origAPIKey })

	router := setupTestRouter()
	router.GET("/api/weather/", GetWeatherWithCFHeader)

	// 使用未被其它测试缓存的城市，避免命中城市查询缓存走重试路径
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/weather/", nil)
	req.Header.Set("CF-IPCity", "上海")
	req.Header.Set("CF-Region", "上海")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetWeatherWithCFHeader_Success(t *testing.T) {
	ensureTestDB()
	mock := setupMockWeatherServer(t)

	origHost := model.Configs.APIKey.APIHost
	model.Configs.APIKey.APIHost = strings.TrimPrefix(mock.URL, "https://")
	t.Cleanup(func() { model.Configs.APIKey.APIHost = origHost })

	router := setupTestRouter()
	router.GET("/api/weather/", GetWeatherWithCFHeader)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/weather/", nil)
	req.Header.Set("CF-IPCity", "北京")
	req.Header.Set("CF-Region", "北京")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp model.WeatherResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "北京", resp.Where)
	assert.Equal(t, "25", resp.Temp)
}

// WebSocket tests

func TestWebSocketPlaceholder(t *testing.T) {
	ensureTestDB()

	router := setupTestRouter()
	router.Any("/ws/:school/:grade/:class_number", WebSocketPlaceholder)

	// 契约：普通 HTTP 请求（无 Upgrade 头）必须返回 400
	w := doClientRequest(t, router, "GET", "/ws/school1/grade1/class1")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWebSocketPlaceholder_Serverless(t *testing.T) {
	ensureTestDB()

	// serverless 模式下 WebSocket 必须被禁用（返回 501）
	origServerless := model.Configs.Run.Serverless
	model.Configs.Run.Serverless = true
	t.Cleanup(func() { model.Configs.Run.Serverless = origServerless })

	router := setupTestRouter()
	router.Any("/ws/:school/:grade/:class_number", WebSocketPlaceholder)

	w := doClientRequest(t, router, "GET", "/ws/school1/grade1/class1")
	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

// BroadcastSync 内部广播测试（外部 /api/broadcast 入口已废弃移除）

func TestBroadcastSync_ServerlessDisabled(t *testing.T) {
	ensureTestDB()

	orig := model.Configs.Run.Serverless
	model.Configs.Run.Serverless = true
	t.Cleanup(func() { model.Configs.Run.Serverless = orig })

	// serverless 模式禁用 WebSocket，内部广播必须直接跳过
	assert.Equal(t, 0, BroadcastSync("school1", "grade1"))
	assert.Equal(t, 0, BroadcastSyncSchool("school1"))
	assert.Equal(t, 0, BroadcastSyncAll())
}

func TestBroadcastSync_NoClients(t *testing.T) {
	ensureTestDB()

	orig := model.Configs.Run.Serverless
	model.Configs.Run.Serverless = false
	t.Cleanup(func() { model.Configs.Run.Serverless = orig })

	assert.Equal(t, 0, BroadcastSync("nobody", "here"))
	assert.Equal(t, 0, BroadcastSyncSchool("nobody"))
	assert.Equal(t, 0, BroadcastSyncAll())
}

func TestBroadcastSync_DeliversToConnectedClient(t *testing.T) {
	ensureTestDB()

	orig := model.Configs.Run.Serverless
	model.Configs.Run.Serverless = false
	t.Cleanup(func() { model.Configs.Run.Serverless = orig })

	// 起真实 WebSocket 服务：升级成功后把连接注册进 hub，模拟在线客户端
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		scope := wsScope{School: "39", Grade: "2023"}
		clientWsHub.add(scope, "1", conn)
		defer clientWsHub.remove(scope, conn)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// 等待服务端完成 hub 注册
	deadline := time.Now().Add(2 * time.Second)
	for clientWsHub.count(wsScope{School: "39", Grade: "2023"}) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	require.Equal(t, 1, clientWsHub.count(wsScope{School: "39", Grade: "2023"}))

	// 年级级广播应送达
	assert.Equal(t, 1, BroadcastSync("39", "2023"))
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, "SyncConfig", string(msg))

	// 学校级广播应送达
	assert.Equal(t, 1, BroadcastSyncSchool("39"))
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, msg, err = conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, "SyncConfig", string(msg))

	// 全量广播应送达；无关学校不受影响
	assert.Equal(t, 1, BroadcastSyncAll())
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, msg, err = conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, "SyncConfig", string(msg))
	assert.Equal(t, 0, BroadcastSync("other", "school"))
}

// 自动任务 v2：客户端配置规则随课表响应下发，范围条件按日期命中
func TestGetSchedule_ClientConfigRulesAndRangeRule(t *testing.T) {
	ensureTestDB()

	database := db.GetDB()
	// 使用独立 scope，避免与其它测试共享数据
	database.Save(&dbTable.Schedule{
		School: "autorun", Grade: "2024", Class: "1",
		DailyClasses: [7]dbTable.DailyClass{
			{Timetable: "常日", ClassList: dbTable.ClassList{{"数"}}},
		},
	})
	database.Save(&dbTable.Timetable{
		School: "autorun", Grade: "2024",
		TimetableConfig: dbTable.TimetableConfig{
			Timetable: map[string]map[string]interface{}{"常日": {"08:00-08:40": 0}, "exam": {"09:00-09:40": 0}},
			Divider:   map[string][]int{"常日": {}},
			Start:     "2020-09-01",
		},
	})
	// 覆盖极宽的日期范围：断言不受当前日期影响
	database.Save(&dbTable.AutorunRecord{
		HashID: "range-rule", EType: dbTable.AutorunTypeTimetable, Scope: []string{"autorun"}, Level: 1,
		Entries: []dbTable.AutorunEntry{{
			ID: "e1", When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenRange, StartDate: "2000-01-01", EndDate: "2099-12-31"},
			Action: map[string]interface{}{"timetableId": "exam"},
		}},
	})
	database.Save(&dbTable.AutorunRecord{
		HashID: "cfg-rule", EType: dbTable.AutorunTypeClientConfig, Scope: []string{"autorun/2024/1"}, Level: 5,
		Entries: []dbTable.AutorunEntry{{
			ID: "e1", When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenEvent, Event: dbTable.AutorunEventClassStart, Period: 1},
			Action: map[string]interface{}{"settings": map[string]interface{}{"isWindowAlwaysOnTop": true}},
		}},
	})
	database.Save(&dbTable.AutorunRecord{
		HashID: "cfg-other-class", EType: dbTable.AutorunTypeClientConfig, Scope: []string{"autorun/2024/2"}, Level: 5,
		Entries: []dbTable.AutorunEntry{{ID: "e1", Action: map[string]interface{}{"settings": map[string]interface{}{"isAlwaysMinimized": true}}}},
	})

	router := setupTestRouter()
	router.GET("/:school/:grade/:class", GetSchedule)

	// 处理函数在请求开始时取 now，这里把请求前后的星期都视为合法：
	// 避免断言时重新读时间在跨零点时抖动
	before := time.Now().Weekday()
	w := doClientRequest(t, router, "GET", "/autorun/2024/1")
	after := time.Now().Weekday()
	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	// 新增的调度基准字段
	assert.Contains(t, resp, "week_number")
	assert.Equal(t, "2020-09-01", resp["term_start"])

	// 日期范围条件命中：作息表替换只作用于「今天」对应的星期（与 v1 行为一致）
	dailyClass, ok := resp["daily_class"].([]interface{})
	require.True(t, ok)
	require.Len(t, dailyClass, 7)
	appliedDays := make([]int, 0, 2)
	for idx, day := range dailyClass {
		if day.(map[string]interface{})["timetable"] == "exam" {
			appliedDays = append(appliedDays, idx)
		}
	}
	require.Len(t, appliedDays, 1, "应恰好替换一天的作息表")
	assert.Contains(t, []int{int(before), int(after)}, appliedDays[0], "被替换的应当是请求当天")

	// 客户端配置规则只下发本班作用域命中的条目
	rules, ok := resp["client_config_rules"].([]interface{})
	require.True(t, ok)
	require.Len(t, rules, 1)
	rule := rules[0].(map[string]interface{})
	assert.Equal(t, "cfg-rule", rule["taskId"])
	assert.Equal(t, float64(5), rule["priority"])
	settings, ok := rule["settings"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, settings["isWindowAlwaysOnTop"])
}
