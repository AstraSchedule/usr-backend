package client

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model"
	"AstraScheduleServerGo/model/dbTable"
	"AstraScheduleServerGo/service"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func GetSchedule(c *gin.Context) {
	school := c.Param("school")
	grade := c.Param("grade")
	class := c.Param("class")
	version := c.Query("version") // 可能没有
	clientDataVersion := int64(0)
	clientWeekNumber := 0
	clientBoundary := int64(0)
	if version != "" {
		var err error
		clientDataVersion, clientWeekNumber, clientBoundary, err = parseScheduleVersion(version)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{ // 400
				"error": err.Error(),
			})
			return
		}
	}
	now := time.Now()
	serverDataVersion := db.GetLatestVersion(school, grade, class)
	schedule := db.GetSchedule(school, grade, class)
	timetable := db.GetTimetable(school, grade)
	// 自动任务查询失败时宁可返回 500：否则会静默丢掉全部规则，把错误的课表当成正确结果下发
	records, err := db.FetchAutorunRecords("")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	weekNumber := service.CalcWeekNumber(timetable.TimetableConfig.Start, now)
	// 单日/日期范围/cron 等条件可能在周内改变命中结果：把「下一次变化时刻」写进版本串，
	// 变化之前 304 缓存照常生效，越过变化点后版本必然不同，客户端下一次请求即拿到新配置
	boundary := service.VersionBoundary(records, school, grade, class, now)
	effectiveVersion := scheduleVersion(serverDataVersion.Timestamp(), weekNumber, boundary)
	if clientDataVersion == serverDataVersion.Timestamp() && clientWeekNumber == weekNumber && clientBoundary == boundary {
		c.Status(http.StatusNotModified) // 304
		return
	}
	_, _ = db.RefreshAutorunStatuses(now)
	clientConfig := db.GetClientConfig(school, grade, class)

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
	subject := db.GetSubject(school, grade)
	resolvedDailyClasses := service.ApplyScheduleRulesCtx(
		schedule.DailyClasses,
		timetable.TimetableConfig.Timetable,
		records,
		school,
		grade,
		class,
		service.RuleContext{Now: now, TermStart: timetable.TimetableConfig.Start},
	)
	// 客户端配置规则：服务端按生效域过滤，时间条件由桌面端本地调度求值
	clientConfigRules := service.CollectClientConfigRules(records, school, grade, class)

	// 根据当前周数解析多周轮换课程，生成扁平的 classList
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
	allCountdowns, _ := db.FetchCountdownRecords("")
	filteredCountdowns := service.FilterCountdownByScope(allCountdowns, classID)

	fullResponse := model.FullResponseConfig{
		SupportWebsocket:  model.Configs.WebSocketEnabled(),
		Version:           effectiveVersion,
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
		"countdown_records":      fullResponse.CountdownRecords,
		// 桌面端本地调度客户端配置规则所需的时间基准与规则集
		"week_number":         weekNumber,
		"term_start":          timetable.TimetableConfig.Start,
		"client_config_rules": clientConfigRules,
	}
	c.JSON(http.StatusOK, fullResponseMap)
}

// scheduleVersion 生成客户端缓存版本：dataVersion:weekNumber[:boundary]
// boundary 是「该班课表下一次可能变化」的时刻；不存在后续变化点时省略该段，304 缓存长期有效。
func scheduleVersion(dataVersion int64, weekNumber int, boundary int64) string {
	if weekNumber < 1 {
		weekNumber = 1
	}
	version := strconv.FormatInt(dataVersion, 10) + ":" + strconv.Itoa(weekNumber)
	if boundary > 0 {
		version += ":" + strconv.FormatInt(boundary, 10)
	}
	return version
}

func parseScheduleVersion(version string) (int64, int, int64, error) {
	parts := strings.Split(version, ":")
	if len(parts) == 1 {
		// 兼容旧客户端发送的纯数据版本；week=0 保证不会误命中新的复合版本。
		dataVersion, err := strconv.ParseInt(parts[0], 10, 64)
		return dataVersion, 0, 0, err
	}
	if len(parts) > 3 {
		return 0, 0, 0, strconv.ErrSyntax
	}
	dataVersion, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	weekNumber, err := strconv.Atoi(parts[1])
	if err != nil || weekNumber < 1 {
		if err == nil {
			err = strconv.ErrSyntax
		}
		return 0, 0, 0, err
	}
	boundary := int64(0)
	if len(parts) == 3 {
		boundary, err = strconv.ParseInt(parts[2], 10, 64)
		if err != nil || boundary < 0 {
			return 0, 0, 0, strconv.ErrSyntax
		}
	}
	return dataVersion, weekNumber, boundary, nil
}
