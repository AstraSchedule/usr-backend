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

	err := db.GetDB().AutoMigrate(
		&dbTable.Schedule{},
		&dbTable.ClientConfig{},
		&dbTable.Timetable{},
		&dbTable.Subject{},
		&dbTable.DataVersion{},
		&dbTable.AutorunRecord{},
		&dbTable.CountdownRecord{},
	)
	if err != nil {
		logrus.Fatal(err)
		return
	}

	logrus.Info("AutoMigrate 执行完成")
}
