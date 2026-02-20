package client

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model"
	"net/http"
	"strconv"

	"github.com/dromara/carbon/v2"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func GetSchedule(c *gin.Context) {
	school := c.Param("school")
	grade := c.Param("grade")
	class := c.Param("class")
	version := c.Query("version") // 可能没有
	clientDataVersion := carbon.CreateFromTimestamp(0)
	if version != "" {
		cDVInt, err := strconv.ParseInt(version, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{ // 400
				"error": err.Error(),
			})
			return
		}
		clientDataVersion = carbon.CreateFromTimestamp(cDVInt)
	}
	logrus.Infof("school: %s, grade: %s, class: %s, version: %s", school, grade, class, version)
	serverDataVersion := db.GetLatestVersion(class, school, grade)
	if clientDataVersion.Eq(serverDataVersion) {
		c.Status(http.StatusNotModified) // 304
		return
	}
	clientConfig := db.GetClientConfig(school, grade, class)
	schedule := db.GetSchedule(school, grade, class)
	subject := db.GetSubject(school, grade)
	timetable := db.GetTimetable(school, grade)
	fullResponse := model.FullResponseConfig{
		SupportWebsocket:  false,
		Version:           strconv.FormatInt(serverDataVersion.Timestamp(), 10),
		DailyClasses:      schedule.DailyClasses,
		ClientConfigItems: clientConfig.ClientConfigItems,
		TimetableConfig:   timetable.TimetableConfig,
		SubjectConfig:     subject.SubjectConfig,
	}
	c.JSON(http.StatusOK, fullResponse)
}
