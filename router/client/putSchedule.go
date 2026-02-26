package client

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model"
	"AstraScheduleServerGo/model/dbTable"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm/clause"
)

func PutSchedule(c *gin.Context) {
	school := c.Param("school")
	grade := c.Param("grade")
	class := c.Param("class")
	fullCC := &model.FullClientConfig{}
	err := c.ShouldBind(fullCC)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}) // 400
		return
	}
	logrus.Infof("收到如下请求：%+v", fullCC)
	cC := dbTable.ClientConfig{
		School:            school,
		Grade:             grade,
		Class:             class,
		ClientConfigItems: fullCC.ClientConfigItems,
	}
	schedule := dbTable.Schedule{
		School:       school,
		Grade:        grade,
		Class:        class,
		DailyClasses: fullCC.DailyClasses,
	}
	subject := dbTable.Subject{
		School:        school,
		Grade:         grade,
		SubjectConfig: fullCC.SubjectConfig,
	}
	timetable := dbTable.Timetable{
		School:          school,
		Grade:           grade,
		TimetableConfig: fullCC.TimetableConfig,
	}
	dataVersion := dbTable.DataVersion{
		School:  school,
		Grade:   grade,
		Class:   class,
		Version: time.Now(),
	}
	tx := db.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "school"}, {Name: "grade"}, {Name: "class"}},
		UpdateAll: true,
	}).Create(&cC)
	tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "school"}, {Name: "grade"}, {Name: "class"}},
		UpdateAll: true,
	}).Create(&schedule)
	tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "school"}, {Name: "grade"}, {Name: "class"}},
		UpdateAll: true,
	}).Create(&dataVersion)
	tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "school"}, {Name: "grade"}},
		UpdateAll: true,
	}).Create(&subject)
	tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "school"}, {Name: "grade"}},
		UpdateAll: true,
	}).Create(&timetable)

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "事务提交失败：" + err.Error()}) // 500
		return
	}

	c.JSON(http.StatusOK, gin.H{ // 200
		"message": "课表更新成功",
		"version": strconv.FormatInt(dataVersion.Version.Unix(), 10),
	})
}
