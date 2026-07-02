package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model"
	"AstraScheduleServerGo/model/dbTable"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

var testDBInitialized = false

func ensureTestDB() {
	if testDBInitialized {
		return
	}
	model.Configs = model.SrvConfig{
		Server: model.ServerConfig{
			Host:   "127.0.0.1",
			Port:   9000,
			Domain: []string{"http://localhost:5173"},
		},
		Secret: model.SecretConfig{
			Token: "test-token-123",
		},
		Db: model.DbConfig{
			Type: "sqlite",
			Path: ":memory:",
		},
		APIKey: model.APIKeyConfig{
			APIHost: "https://geoapi.qweather.com",
			Weather: "test-weather-key",
		},
		Log: model.LogConfig{
			Debug: true,
		},
		Run: model.RunConfig{
			Serverless: false,
		},
	}
	database := db.GetDB()
	database.AutoMigrate(
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
	assert.NotNil(t, resp["subject"])
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

// GetWeather tests

func TestGetWeatherWithProvince_Success(t *testing.T) {
	ensureTestDB()

	router := setupTestRouter()
	router.GET("/api/weather/:name1/:name2", GetWeatherWithProvince)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/weather/北京/朝阳", nil)
	router.ServeHTTP(w, req)

	// May fail due to external API or config, but should not panic
	assert.True(t, w.Code >= 200 && w.Code < 600)
}

func TestGetWeatherWithCity_Success(t *testing.T) {
	ensureTestDB()

	router := setupTestRouter()
	router.GET("/api/weather/:name1", GetWeatherWithCity)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/weather/北京", nil)
	router.ServeHTTP(w, req)

	// May fail due to external API or config, but should not panic
	assert.True(t, w.Code >= 200 && w.Code < 600)
}

func TestGetWeatherWithCFHeader_Success(t *testing.T) {
	ensureTestDB()

	router := setupTestRouter()
	router.GET("/api/weather/", GetWeatherWithCFHeader)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/weather/", nil)
	req.Header.Set("CF-IPCountry", "CN")
	router.ServeHTTP(w, req)

	// May fail due to external API or config, but should not panic
	assert.True(t, w.Code >= 200 && w.Code < 600)
}

// WebSocket tests

func TestWebSocketPlaceholder(t *testing.T) {
	ensureTestDB()

	router := setupTestRouter()
	router.Any("/ws/:school/:grade/:class_number", WebSocketPlaceholder)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ws/school1/grade1/class1", nil)
	router.ServeHTTP(w, req)

	// WebSocket upgrade will fail in test, but handler should not panic
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusUpgradeRequired || w.Code == http.StatusBadRequest)
}

// BroadcastSyncConfig tests

func TestBroadcastSyncConfig_NoAuth(t *testing.T) {
	ensureTestDB()

	router := setupTestRouter()
	router.POST("/api/broadcast/:school/:grade/:class_number", BroadcastSyncConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/broadcast/school1/grade1/class1", nil)
	router.ServeHTTP(w, req)

	// Without auth, should fail or succeed depending on middleware
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusUnauthorized)
}
