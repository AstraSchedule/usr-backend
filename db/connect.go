package db

import (
	"AstraScheduleServerGo/model"
	"fmt"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var (
	dbOnce sync.Once
	dbInst *gorm.DB
	dbErr  error
)

func ConnectDb() *gorm.DB {
	dbOnce.Do(func() {
		cfg := mysql.NewConfig()
		cfg.User = model.Configs.Db.User
		cfg.Passwd = model.Configs.Db.Pass
		cfg.Net = "tcp"
		cfg.Addr = fmt.Sprintf("%s:%d", model.Configs.Db.Host, model.Configs.Db.Port)
		cfg.DBName = model.Configs.Db.Name
		cfg.ParseTime = true
		cfg.Loc = time.Local
		cfg.Params = map[string]string{
			"charset": "utf8mb4",
		}

		dsn := cfg.FormatDSN()

		logrus.Infof("Connecting to database: %s@%s:%d/%s",
			model.Configs.Db.User,
			model.Configs.Db.Host,
			model.Configs.Db.Port,
			model.Configs.Db.Name)
		db, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
		if err != nil {
			logrus.Errorf("Failed to connect to database: %v", err)
			dbErr = fmt.Errorf("database connection failed: %w", err)
			return
		}
		dbInst = db
		model.Db = dbInst
		logrus.Info("Database connected successfully")
	})
	return dbInst
}

func GetDB() *gorm.DB {
	db := ConnectDb()
	if dbErr != nil {
		panic(dbErr)
	}
	return db
}
