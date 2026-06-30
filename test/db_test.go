package test

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model"
	"AstraScheduleServerGo/model/dbTable"
	"AstraScheduleServerGo/startup"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	gormsqlite "github.com/libtnb/sqlite"
)

func TestMain(m *testing.M) {
	// Set up config for db package to use SQLite
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

	// Initialize the database connection and create tables
	db.GetDB()
	startup.MigrateDb()

	os.Exit(m.Run())
}

func setupDBSingleton(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(gormsqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = database.AutoMigrate(
		&dbTable.Schedule{},
		&dbTable.ClientConfig{},
		&dbTable.Timetable{},
		&dbTable.Subject{},
		&dbTable.DataVersion{},
		&dbTable.AutorunRecord{},
		&dbTable.CountdownRecord{},
	)
	require.NoError(t, err)

	return database
}

func TestGetSchedule_Found(t *testing.T) {
	database := db.GetDB()
	schedule := &dbTable.Schedule{
		School: "school1",
		Grade:  "grade1",
		Class:  "class1",
		DailyClasses: [7]dbTable.DailyClass{
			{Timetable: "常日", ClassList: dbTable.ClassList{{"数"}}},
		},
	}
	database.Save(schedule)

	result := db.GetSchedule("school1", "grade1", "class1")
	assert.NotNil(t, result)
	assert.Equal(t, "school1", result.School)
	assert.Equal(t, "常日", result.DailyClasses[0].Timetable)
}

func TestGetSchedule_NotFound(t *testing.T) {
	result := db.GetSchedule("nonexistent", "grade", "class")
	assert.NotNil(t, result)
	// GORM returns empty struct, not nil
	assert.Equal(t, "", result.School)
}

func TestGetSubject_Found(t *testing.T) {
	database := db.GetDB()
	subject := &dbTable.Subject{
		School: "school1",
		Grade:  "grade1",
		SubjectConfig: dbTable.SubjectConfig{
			SubjectName: map[string]string{
				"数": "数学",
			},
		},
	}
	database.Save(subject)

	result := db.GetSubject("school1", "grade1")
	assert.NotNil(t, result)
	assert.Equal(t, "数学", result.SubjectName["数"])
}

func TestGetTimetable_Found(t *testing.T) {
	database := db.GetDB()
	timetable := &dbTable.Timetable{
		School: "school1",
		Grade:  "grade1",
		TimetableConfig: dbTable.TimetableConfig{
			Timetable: map[string]map[string]interface{}{
				"常日": {"早上1": 1},
			},
		},
	}
	database.Save(timetable)

	result := db.GetTimetable("school1", "grade1")
	assert.NotNil(t, result)
	assert.Contains(t, result.Timetable, "常日")
}

func TestGetClientConfig_Found(t *testing.T) {
	database := db.GetDB()
	config := &dbTable.ClientConfig{
		School: "school1",
		Grade:  "grade1",
		Class:  "class1",
	}
	database.Save(config)

	result := db.GetClientConfig("school1", "grade1", "class1")
	assert.NotNil(t, result)
	assert.Equal(t, "school1", result.School)
}

func TestGetLatestVersion_Found(t *testing.T) {
	database := db.GetDB()
	now := time.Now()
	version := &dbTable.DataVersion{
		School:  "school1",
		Grade:   "grade1",
		Class:   "class1",
		Version: now,
	}
	database.Save(version)

	result := db.GetLatestVersion("school1", "grade1", "class1")
	assert.NotNil(t, result)
}

func TestUpsertAndFetchAutorunRecord(t *testing.T) {
	database := setupDBSingleton(t)
	_ = database

	record := &dbTable.AutorunRecord{
		HashID: "hash1",
		EType:  2,
		Scope:  []string{"ALL"},
		Parameters: map[string]interface{}{
			"date": "2025-10-15",
			"rule": map[string]interface{}{
				"schedule": map[string]interface{}{
					"periods": []interface{}{
						map[string]interface{}{"no": 1, "subject": "数"},
					},
				},
			},
		},
		Level:  1,
		Status: 0,
	}

	err := db.UpsertAutorunRecord(record)
	assert.NoError(t, err)

	records, err := db.FetchAutorunRecords("")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(records), 1)
}

func TestDeleteAutorunRecord(t *testing.T) {
	database := setupDBSingleton(t)
	_ = database

	record := &dbTable.AutorunRecord{
		HashID: "hash-delete",
		EType:  2,
		Scope:  []string{"ALL"},
		Parameters: map[string]interface{}{
			"date": "2025-10-15",
		},
		Level: 1,
	}
	db.UpsertAutorunRecord(record)

	count, err := db.DeleteAutorunRecord("hash-delete")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestUpsertAndFetchCountdownRecord(t *testing.T) {
	database := setupDBSingleton(t)
	_ = database

	record := &dbTable.CountdownRecord{
		ID:    "countdown-1",
		Scope: []string{"ALL"},
		Schedules: []dbTable.CountdownScheduleItem{
			{Name: "期末考试", Date: "2025-12-20", Priority: 1},
		},
	}

	err := db.UpsertCountdownRecord(record)
	assert.NoError(t, err)

	records, err := db.FetchCountdownRecords("")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(records), 1)
}

func TestDeleteCountdownRecord(t *testing.T) {
	database := setupDBSingleton(t)
	_ = database

	record := &dbTable.CountdownRecord{
		ID:    "countdown-delete",
		Scope: []string{"ALL"},
		Schedules: []dbTable.CountdownScheduleItem{
			{Name: "运动会", Date: "2025-11-01", Priority: 1},
		},
	}
	db.UpsertCountdownRecord(record)

	count, err := db.DeleteCountdownRecord("countdown-delete")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
}
