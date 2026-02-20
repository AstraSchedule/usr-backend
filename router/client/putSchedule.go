package client

import (
	"AstraScheduleServerGo/model"
	"AstraScheduleServerGo/model/dbTable"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
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
	tx := model.Db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	if err := tx.Where("school = ? AND grade = ? AND class = ?", school, grade, class).
		Assign("client_config_items", fullCC.ClientConfigItems).
		FirstOrCreate(&cC).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ClientConfig 写入失败：" + err.Error()}) // 500
		return
	}
	if err := tx.Where("school = ? AND grade = ? AND class = ?", school, grade, class).
		Assign("daily_classes", fullCC.DailyClasses).
		FirstOrCreate(&schedule).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Schedule 写入失败：" + err.Error()}) // 500
		return
	}
	if err := tx.Where("school = ? AND grade = ?", school, grade).
		Assign("subject_config", fullCC.SubjectConfig).
		FirstOrCreate(&subject).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Subject 写入失败：" + err.Error()}) // 500
		return
	}
	if err := tx.Where("school = ? AND grade = ?", school, grade).
		Assign("timetable", fullCC.TimetableConfig.Timetable).
		Assign("divider", fullCC.TimetableConfig.Divider).
		Assign("start_date", fullCC.TimetableConfig.Start).
		FirstOrCreate(&timetable).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Timetable 写入失败：" + err.Error()}) // 500
		return
	}
	if err := tx.Where("school = ? AND grade = ? AND class = ?", school, grade, class).
		Assign("version", dataVersion.Version).
		FirstOrCreate(&dataVersion).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DataVersion 写入失败：" + err.Error()}) // 500
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "事务提交失败：" + err.Error()}) // 500
		return
	}

	c.JSON(http.StatusOK, gin.H{ // 200
		"message": "课表更新成功",
		"version": dataVersion.Version.Unix(),
	})
}
