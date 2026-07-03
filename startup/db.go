package startup

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model"
	"AstraScheduleServerGo/model/dbTable"
	"os"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

func shouldSkipAutoMigrate() bool {
	if model.Configs.Db.Type == "sqlite" {
		return false // SQLite 由于无法从外网访问进行初始化，而且 AutoMigrate 足够快，所以总是执行
	}
	raw := strings.TrimSpace(os.Getenv("GIN_MODE"))
	if raw == "" {
		return false
	}
	if skip, err := strconv.ParseBool(raw); err == nil {
		return skip
	}
	// 兼容 Gin 常见模式值：release/debug/test
	if strings.EqualFold(raw, "release") {
		return true
	}
	return false
}

func MigrateDb() {
	if shouldSkipAutoMigrate() {
		logrus.Infof("检测到环境变量 GIN_MODE=%q，已跳过 AutoMigrate", os.Getenv("GIN_MODE"))
		return
	}

	logrus.Info("开始执行 AutoMigrate")

	// SQLite: Clean up orphan indexes from previous failed migrations
	if strings.EqualFold(model.Configs.Db.Type, "sqlite") {
		db.GetDB().Exec("DROP INDEX IF EXISTS idx_unique_school_grade_class")
		db.GetDB().Exec("DROP INDEX IF EXISTS idx_unique_school_grade")
	}

	// 清理旧版（无 namespace）索引，确保 AutoMigrate 能重建包含 namespace 的新索引
	db.GetDB().Exec("DROP INDEX IF EXISTS idx_schedules_school_grade_class")
	db.GetDB().Exec("DROP INDEX IF EXISTS idx_client_configs_school_grade_class")
	db.GetDB().Exec("DROP INDEX IF EXISTS idx_timetables_school_grade")
	db.GetDB().Exec("DROP INDEX IF EXISTS idx_subjects_school_grade")
	db.GetDB().Exec("DROP INDEX IF EXISTS idx_data_versions_school_grade_class")

	err := db.GetDB().AutoMigrate(
		&dbTable.Schedule{},
		&dbTable.ClientConfig{},
		&dbTable.Timetable{},
		&dbTable.Subject{},
		&dbTable.DataVersion{},
		&dbTable.AutorunRecord{},
		&dbTable.CountdownRecord{},
		&dbTable.User{},
	)
	if err != nil {
		logrus.Fatal(err)
		return
	}

	logrus.Info("AutoMigrate 执行完成")
}
