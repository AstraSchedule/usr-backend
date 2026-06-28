package web

import (
	"fmt"
	"net/http"

	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model/dbTable"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func DropAstraTable(c *gin.Context) {
	tableName := c.Param("table")

	// Validate: check if table exists
	var count int64
	db.GetDB().Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&count)
	if count == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "表不存在"})
		return
	}

	db.GetDB().Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName))
	logrus.Infof("已删除表: %s", tableName)

	// Recreate by model name mapping
	modelMap := map[string]interface{}{
		"schedules":         &dbTable.Schedule{},
		"client_configs":    &dbTable.ClientConfig{},
		"timetables":        &dbTable.Timetable{},
		"subjects":          &dbTable.Subject{},
		"data_versions":     &dbTable.DataVersion{},
		"autorun_records":   &dbTable.AutorunRecord{},
		"countdown_records": &dbTable.CountdownRecord{},
		"users":             &dbTable.User{},
	}

	if model, ok := modelMap[tableName]; ok {
		db.GetDB().AutoMigrate(model)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  200,
		"message": fmt.Sprintf("表 %s 已删除并重建", tableName),
	})
}
