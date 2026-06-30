package test

import (
	"AstraScheduleServerGo/model"
	"AstraScheduleServerGo/model/dbTable"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassList_UnmarshalJSON_OldFormat(t *testing.T) {
	input := `["物","数","语"]`
	var cl dbTable.ClassList
	err := json.Unmarshal([]byte(input), &cl)
	require.NoError(t, err)
	assert.Equal(t, dbTable.ClassList{{"物"}, {"数"}, {"语"}}, cl)
}

func TestClassList_UnmarshalJSON_NewFormat(t *testing.T) {
	input := `[["物","数"],["语"]]`
	var cl dbTable.ClassList
	err := json.Unmarshal([]byte(input), &cl)
	require.NoError(t, err)
	assert.Equal(t, dbTable.ClassList{{"物", "数"}, {"语"}}, cl)
}

func TestClassList_UnmarshalJSON_MixedFormat(t *testing.T) {
	input := `["物",["数","语"],"化"]`
	var cl dbTable.ClassList
	err := json.Unmarshal([]byte(input), &cl)
	require.NoError(t, err)
	assert.Equal(t, dbTable.ClassList{{"物"}, {"数", "语"}, {"化"}}, cl)
}

func TestClassList_UnmarshalJSON_EmptyArray(t *testing.T) {
	input := `[]`
	var cl dbTable.ClassList
	err := json.Unmarshal([]byte(input), &cl)
	require.NoError(t, err)
	assert.Equal(t, dbTable.ClassList{}, cl)
}

func TestClassList_MarshalJSON(t *testing.T) {
	cl := dbTable.ClassList{{"物"}, {"数", "语"}}
	data, err := json.Marshal(cl)
	require.NoError(t, err)
	assert.Equal(t, `[["物"],["数","语"]]`, string(data))
}

func TestClassList_RoundTrip(t *testing.T) {
	original := dbTable.ClassList{{"数"}, {"语", "英"}, {"物"}, {"化"}}
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded dbTable.ClassList
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestSchedule_JSON_Serialization(t *testing.T) {
	schedule := dbTable.Schedule{
		ID:     1,
		School: "test-school",
		Grade:  "test-grade",
		Class:  "1班",
		DailyClasses: [7]dbTable.DailyClass{
			{Timetable: "常日", ClassList: dbTable.ClassList{{"数"}, {"语"}}},
			{},
			{},
			{},
			{},
			{},
			{},
		},
	}

	data, err := json.Marshal(schedule)
	require.NoError(t, err)

	var decoded dbTable.Schedule
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "test-school", decoded.School)
	assert.Equal(t, "1班", decoded.Class)
	assert.Equal(t, "常日", decoded.DailyClasses[0].Timetable)
	assert.Equal(t, dbTable.ClassList{{"数"}, {"语"}}, decoded.DailyClasses[0].ClassList)
}

func TestSrvConfig_Validate_EmptyConfig(t *testing.T) {
	cfg := model.SrvConfig{}
	err := cfg.Validate()
	assert.Error(t, err)
}

func TestDbConfig_Validate_EmptyType(t *testing.T) {
	dbCfg := model.DbConfig{}
	err := dbCfg.Validate()
	// Empty type defaults to mysql, which requires host/port/user/name
	assert.Error(t, err)
}

func TestDbConfig_Validate_MissingMysqlFields(t *testing.T) {
	dbCfg := model.DbConfig{Type: "mysql"}
	err := dbCfg.Validate()
	assert.Error(t, err)
}

func TestDbConfig_Validate_MysqlValid(t *testing.T) {
	dbCfg := model.DbConfig{
		Type: "mysql",
		Host: "localhost",
		Port: 3306,
		User: "root",
		Name: "testdb",
	}
	err := dbCfg.Validate()
	assert.NoError(t, err)
}

func TestDbConfig_Validate_SqliteMissingPath(t *testing.T) {
	dbCfg := model.DbConfig{Type: "sqlite"}
	err := dbCfg.Validate()
	assert.Error(t, err)
}

func TestDbConfig_Validate_SqliteValid(t *testing.T) {
	dbCfg := model.DbConfig{
		Type: "sqlite",
		Path: "test.db",
	}
	err := dbCfg.Validate()
	assert.NoError(t, err)
}

func TestDbConfig_Validate_UnknownType(t *testing.T) {
	dbCfg := model.DbConfig{Type: "postgres"}
	err := dbCfg.Validate()
	assert.Error(t, err)
}

func TestAPIKeyConfig_Validate_MissingAPIHost(t *testing.T) {
	apiCfg := model.APIKeyConfig{}
	err := apiCfg.Validate()
	assert.Error(t, err)
}

func TestAPIKeyConfig_Validate_JWTExpiresOutOfRange(t *testing.T) {
	apiCfg := model.APIKeyConfig{
		APIHost: "https://example.com",
		JWT: model.JWTAuthConfig{
			KID:            "key1",
			ProjectID:      "proj1",
			PrivateKeyPEM:  "pem",
			Expires:        0,
		},
	}
	err := apiCfg.Validate()
	// Expires=0 defaults to 900, which is valid
	assert.NoError(t, err)
}

func TestAPIKeyConfig_Validate_JWTExpiresTooHigh(t *testing.T) {
	apiCfg := model.APIKeyConfig{
		APIHost: "https://example.com",
		JWT: model.JWTAuthConfig{
			KID:            "key1",
			ProjectID:      "proj1",
			PrivateKeyPEM:  "pem",
			Expires:        90000,
		},
	}
	err := apiCfg.Validate()
	assert.Error(t, err)
}

func TestAPIKeyConfig_Validate_Valid(t *testing.T) {
	apiCfg := model.APIKeyConfig{
		APIHost: "https://example.com",
		JWT: model.JWTAuthConfig{
			KID:            "key1",
			ProjectID:      "proj1",
			PrivateKeyPEM:  "pem",
			Expires:        3600,
		},
	}
	err := apiCfg.Validate()
	assert.NoError(t, err)
}

func TestAPIKeyConfig_HasAPIKey(t *testing.T) {
	apiCfg := model.APIKeyConfig{Weather: "key123"}
	assert.True(t, apiCfg.HasAPIKey())

	apiCfgEmpty := model.APIKeyConfig{}
	assert.False(t, apiCfgEmpty.HasAPIKey())
}

func TestSrvConfig_WebSocketEnabled(t *testing.T) {
	cfg := model.SrvConfig{Run: model.RunConfig{Serverless: false}}
	assert.True(t, cfg.WebSocketEnabled())

	cfgServerless := model.SrvConfig{Run: model.RunConfig{Serverless: true}}
	assert.False(t, cfgServerless.WebSocketEnabled())
}
