package startup

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model"
	"AstraScheduleServerGo/model/dbTable"

	"github.com/sirupsen/logrus"
)

func ConnectDb() {
	model.Db = db.ConnectDb()
	model.Db.Debug()
}

func MigrateDb() {
	err := model.Db.AutoMigrate(
		&dbTable.Schedule{},
		&dbTable.ClientConfig{},
		&dbTable.Timetable{},
		&dbTable.Subject{},
		&dbTable.DataVersion{},
	)
	if err != nil {
		logrus.Fatal(err)
		return
	}
}
