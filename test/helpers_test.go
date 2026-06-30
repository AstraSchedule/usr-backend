package test

import (
	"AstraScheduleServerGo/model"
	"AstraScheduleServerGo/model/dbTable"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&dbTable.Schedule{},
		&dbTable.ClientConfig{},
		&dbTable.Timetable{},
		&dbTable.Subject{},
		&dbTable.DataVersion{},
		&dbTable.AutorunRecord{},
		&dbTable.CountdownRecord{},
	)
	require.NoError(t, err)

	return db
}

func setupTestConfig(t *testing.T) {
	t.Helper()
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
}

func teardownTestConfig() {
	model.Configs = model.SrvConfig{}
}

func seedSchedule(t *testing.T, db *gorm.DB, school, grade, class string) {
	t.Helper()
	schedule := &dbTable.Schedule{
		School: school,
		Grade:  grade,
		Class:  class,
		DailyClasses: [7]dbTable.DailyClass{
			{Timetable: "常日", ClassList: dbTable.ClassList{{"数"}, {"语"}, {"英"}}},
			{Timetable: "常日", ClassList: dbTable.ClassList{{"物"}, {"化"}, {"生"}}},
			{Timetable: "常日", ClassList: dbTable.ClassList{{"政"}, {"史"}, {"地"}}},
			{Timetable: "常日", ClassList: dbTable.ClassList{{"数"}, {"语"}, {"英"}}},
			{Timetable: "常日", ClassList: dbTable.ClassList{{"物"}, {"化"}, {"生"}}},
			{Timetable: "常日", ClassList: dbTable.ClassList{{"体"}, {"体"}, {"自"}}},
			{Timetable: "常日", ClassList: dbTable.ClassList{{}, {}, {}}},
		},
	}
	result := db.Save(schedule)
	require.NoError(t, result.Error)
}

func seedSubject(t *testing.T, db *gorm.DB, school, grade string) {
	t.Helper()
	subject := &dbTable.Subject{
		School: school,
		Grade:  grade,
		SubjectConfig: dbTable.SubjectConfig{
			SubjectName: map[string]string{
				"数": "数学",
				"语": "语文",
				"英": "英语",
				"物": "物理",
				"化": "化学",
				"生": "生物",
				"政": "政治",
				"史": "历史",
				"地": "地理",
				"体": "体育",
				"自": "自习",
			},
		},
	}
	result := db.Save(subject)
	require.NoError(t, result.Error)
}

func seedTimetable(t *testing.T, db *gorm.DB, school, grade string) {
	t.Helper()
	timetable := &dbTable.Timetable{
		School: school,
		Grade:  grade,
		TimetableConfig: dbTable.TimetableConfig{
			Timetable: map[string]map[string]interface{}{
				"常日": {
					"早上1": 1,
					"早上2": 2,
					"早上3": 3,
					"下午1": 4,
					"下午2": 5,
					"下午3": 6,
				},
			},
		},
	}
	result := db.Save(timetable)
	require.NoError(t, result.Error)
}

func seedClientConfig(t *testing.T, db *gorm.DB, school, grade, class string) {
	t.Helper()
	config := &dbTable.ClientConfig{
		School: school,
		Grade:  grade,
		Class:  class,
	}
	result := db.Save(config)
	require.NoError(t, result.Error)
}

func seedDataVersion(t *testing.T, db *gorm.DB, school, grade, class string) {
	t.Helper()
	version := &dbTable.DataVersion{
		School:  school,
		Grade:   grade,
		Class:   class,
		Version: time.Now(),
	}
	result := db.Save(version)
	require.NoError(t, result.Error)
}
