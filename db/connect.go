package db

import (
	"AstraScheduleServerGo/model"
	"fmt"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func ConnectDb() *gorm.DB {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		model.Configs.Db.User,
		model.Configs.Db.Pass,
		model.Configs.Db.Host,
		model.Configs.Db.Port,
		model.Configs.Db.Name,
	)
	logrus.Infof("Will connect to database: %s", dsn)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		logrus.Fatal(err)
	}
	return db
}
