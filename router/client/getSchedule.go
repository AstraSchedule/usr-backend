package client

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/middleware"
	"AstraScheduleServerGo/model"
	"AstraScheduleServerGo/model/dbTable"
	"AstraScheduleServerGo/service"
	"net/http"
	"strconv"
	"time"

	"github.com/dromara/carbon/v2"
	"github.com/gin-gonic/gin"
)

func GetSchedule(c *gin.Context) {
	ns := middleware.GetNamespace(c)
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
	serverDataVersion := db.GetLatestVersionNs(ns, school, grade, class)
	if clientDataVersion.Eq(serverDataVersion) {
		c.Status(http.StatusNotModified) // 304
		return
	}
	_, _ = db.RefreshAutorunStatusesNs(ns, time.Now())
	clientConfig := db.GetClientConfigNs(ns, school, grade, class)

	// 如果数据库中没有 temperature_colors 配置或 stops 为空，使用默认值
	if len(clientConfig.TemperatureColors.Stops) == 0 {
		clientConfig.TemperatureColors = dbTable.TemperatureColorsConfig{
			UseGradient: clientConfig.TemperatureColors.UseGradient,
			Stops: []dbTable.TemperatureStop{
				{Temp: 20, Color: "#66CCFF"},
				{Temp: 30, Color: "#5FBC21"},
				{Temp: 36, Color: "#FF8C00"},
				{Temp: 100, Color: "#EE0000"},
			},
		}
	}
	schedule := db.GetScheduleNs(ns, school, grade, class)
	subject := db.GetSubjectNs(ns, school, grade)
	timetable := db.GetTimetableNs(ns, school, grade)
	records, _ := db.FetchAutorunRecordsNs(ns, "")
	resolvedDailyClasses := service.ApplyScheduleRules(
		schedule.DailyClasses,
		timetable.TimetableConfig.Timetable,
		records,
		school,
		grade,
		class,
		time.Now(),
	)

	// 根据当前周数解析多周轮换课程，生成扁平的 classList
	weekNumber := service.CalcWeekNumber(timetable.TimetableConfig.Start, time.Now())
	type dailyClassFlat struct {
		Chinese   string   `json:"Chinese"`
		English   string   `json:"English"`
		ClassList []string `json:"classList"`
		Timetable string   `json:"timetable"`
	}
	flatDailyClasses := make([]dailyClassFlat, 7)
	for i := range resolvedDailyClasses {
		flatDailyClasses[i] = dailyClassFlat{
			Chinese:   resolvedDailyClasses[i].Chinese,
			English:   resolvedDailyClasses[i].English,
			ClassList: service.ResolveClassList(resolvedDailyClasses[i].ClassList, weekNumber),
			Timetable: resolvedDailyClasses[i].Timetable,
		}
	}

	// 获取并过滤倒数日记录
	classID := school + "/" + grade + "/" + class
	allCountdowns, _ := db.FetchCountdownRecordsNs(ns, "")
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
	// 确保嵌套 map 不为 nil，避免客户端收到 null
	timetableMap := fullResponse.TimetableConfig.Timetable
	if timetableMap == nil {
		timetableMap = map[string]map[string]interface{}{}
	}
	dividerMap := fullResponse.TimetableConfig.Divider
	if dividerMap == nil {
		dividerMap = map[string][]int{}
	}
	subjectNameMap := fullResponse.SubjectConfig.SubjectName
	if subjectNameMap == nil {
		subjectNameMap = map[string]string{}
	}

	// 覆盖 daily_class 为扁平化格式，展开嵌套结构到顶层
	fullResponseMap := map[string]interface{}{
		"supportWebSocket":       fullResponse.SupportWebsocket,
		"version":                fullResponse.Version,
		"daily_class":            flatDailyClasses,
		"countdown_target":       fullResponse.CountdownTarget,
		"weather_alert_override": fullResponse.WeatherAlertOverride,
		"weather_alert_brief":    fullResponse.WeatherAlertBrief,
		"week_display":           fullResponse.WeekDisplay,
		"banner_text":            fullResponse.BannerText,
		"css_style":              fullResponse.CSSStyle,
		"startup_behavior":       fullResponse.StartupBehavior,
		"temperature_colors":     fullResponse.TemperatureColors,
		"timetable":              timetableMap,
		"divider":                dividerMap,
		"subject_name":           subjectNameMap,
		"countdown":              fullResponse.CountdownRecords,
	}
	c.JSON(http.StatusOK, fullResponseMap)
}
