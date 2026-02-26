package startup

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model/dbTable"
	"os"
	"strconv"

	"github.com/sirupsen/logrus"
)

func ConnectDb() {
	db.GetDB().Debug()
}

func MigrateDb() {
	if skipMigrate, err := strconv.ParseBool(os.Getenv("GIN_MODE")); err == nil && skipMigrate {
		logrus.Info("检测到环境变量 GIN_MODE=true，已跳过 AutoMigrate")
		return
	}

	err := db.GetDB().AutoMigrate(
		&dbTable.Schedule{},
		&dbTable.ClientConfig{},
		&dbTable.Timetable{},
		&dbTable.Subject{},
		&dbTable.DataVersion{},
		&dbTable.AutorunRecord{},
	)
	if err != nil {
		logrus.Fatal(err)
		return
	}
}
