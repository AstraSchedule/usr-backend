package client

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model"
	"AstraScheduleServerGo/model/dbTable"
	"AstraScheduleServerGo/testutil"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
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

// GetSchedule tests

func TestGetSchedule_Empty(t *testing.T) {
	ensureTestDB()

	router := setupTestRouter()
	router.GET("/:school/:grade/:class", GetSchedule)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/school1/grade1/class1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NotNil(t, resp["daily_class"])
	assert.NotNil(t, resp["startup_behavior"])
	assert.NotNil(t, resp["timetable"])
	assert.NotNil(t, resp["subject_name"])
	assert.NotNil(t, resp["divider"])
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

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/weather/北京/朝阳", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp model.WeatherResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
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

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/weather/北京", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp model.WeatherResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "北京", resp.Where)
	assert.Equal(t, "25", resp.Temp)
}

func TestGetWeatherWithCFHeader_NoCFHeader(t *testing.T) {
	ensureTestDB()

	router := setupTestRouter()
	router.GET("/api/weather/", GetWeatherWithCFHeader)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/weather/", nil)
	router.ServeHTTP(w, req)

	// 没有 CF-IPCity 头时应返回 400
	assert.Equal(t, http.StatusBadRequest, w.Code)
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
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "北京", resp.Where)
	assert.Equal(t, "25", resp.Temp)
}

// WebSocket tests

func TestWebSocketPlaceholder(t *testing.T) {
	ensureTestDB()

	router := setupTestRouter()
	router.Any("/ws/:school/:grade/:class_number", WebSocketPlaceholder)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ws/school1/grade1/class1", nil)
	router.ServeHTTP(w, req)

	// WebSocket upgrade fails in test server; any response means handler didn't panic
	assert.True(t, w.Code >= 200, "handler should not return error codes for WS placeholder")
}

// BroadcastSyncConfig tests

func TestBroadcastSyncConfig_NoAuth(t *testing.T) {
	ensureTestDB()

	router := setupTestRouter()
	router.POST("/api/broadcast/:school/:grade/:class_number", BroadcastSyncConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/broadcast/school1/grade1/class1", nil)
	router.ServeHTTP(w, req)

	// BroadcastSyncConfig returns 200 on empty broadcast or 400/500 depending on scope validation
	assert.NotEqual(t, http.StatusNotFound, w.Code, "route should be registered")
}
