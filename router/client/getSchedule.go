package client

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model"
	"AstraScheduleServerGo/service"
	"net/http"
	"strconv"
	"time"

	"github.com/dromara/carbon/v2"
	"github.com/gin-gonic/gin"
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
	serverDataVersion := db.GetLatestVersion(school, grade, class)
	if clientDataVersion.Eq(serverDataVersion) {
		c.Status(http.StatusNotModified) // 304
		return
	}
	_, _ = db.RefreshAutorunStatuses(time.Now())
	clientConfig := db.GetClientConfig(school, grade, class)
	schedule := db.GetSchedule(school, grade, class)
	subject := db.GetSubject(school, grade)
	timetable := db.GetTimetable(school, grade)
	records, _ := db.FetchAutorunRecords("")
	resolvedDailyClasses := service.ApplyScheduleRules(
		schedule.DailyClasses,
		timetable.TimetableConfig.Timetable,
		records,
		school,
		grade,
		class,
		time.Now(),
	)

	// 获取并过滤倒数日记录
	classID := school + "/" + grade + "/" + class
	allCountdowns, _ := db.FetchCountdownRecords("")
	filteredCountdowns := service.FilterCountdownByScope(allCountdowns, classID)

	fullResponse := model.FullResponseConfig{
		SupportWebsocket:  model.Configs.WebSocketEnabled(),
		Version:           strconv.FormatInt(serverDataVersion.Timestamp(), 10),
		DailyClasses:      resolvedDailyClasses,
		ClientConfigItems: clientConfig.ClientConfigItems,
		TimetableConfig:   timetable.TimetableConfig,
		SubjectConfig:     subject.SubjectConfig,
		CountdownRecords:  filteredCountdowns,
	}
	c.JSON(http.StatusOK, fullResponse)
}
