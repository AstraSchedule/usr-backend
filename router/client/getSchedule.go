package client

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/middleware"
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
	ns := middleware.GetNamespace(c)
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
	schedule := db.GetScheduleNs(ns, school, grade, class)
	timetable := db.GetTimetableNs(ns, school, grade)
	subject := db.GetSubjectNs(ns, school, grade)
	clientConfig := db.GetClientConfigNs(ns, school, grade, class)
	// 自动任务查询失败时宁可返回 500：否则会静默丢掉全部规则，把错误的课表当成正确结果下发
	records, err := db.FetchAutorunRecordsNs(ns, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	classID := school + "/" + grade + "/" + class
	// 与自动任务同理：查询失败宁可 500，否则会下发缺失倒数日的响应，
	// 且倒数日时间戳不参与版本计算会让版本回退
	allCountdowns, err := db.FetchCountdownRecordsNs(ns, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	filteredCountdowns := service.FilterCountdownByScope(allCountdowns, classID)

	weekNumber := service.CalcWeekNumber(timetable.TimetableConfig.Start, now)
	// 单日/日期范围/cron 等条件可能在周内改变命中结果：把「下一次变化时刻」写进版本串，
	// 变化之前 304 缓存照常生效，越过变化点后版本必然不同，客户端下一次请求即拿到新配置
	boundary := service.VersionBoundary(records, school, grade, class, now)
	// 数据版本取作用域内所有会影响响应的时间戳的最大值，而不是只看 data_versions 行：
	// 管理端保存课表/作息/科目/客户端配置、自动任务与倒数日规则的新增编辑都只写各自的表，
	// 过去不推进版本，客户端重拉时命中 304、界面停留在旧配置（自动任务更是不写任何数据行，
	// 只能靠记录自身的 UpdatedAt 参与版本）。
	dataVersionTs := db.LatestTimestamp(
		db.GetDataVersionNs(ns, school, grade, class).Version, // 显式版本：客户端 PUT 接口写入
		schedule.UpdatedAt,
		timetable.UpdatedAt,
		subject.UpdatedAt,
		clientConfig.UpdatedAt,
		service.LatestApplicableRecordTimestamp(records, school, grade, class),
		service.LatestCountdownTimestamp(filteredCountdowns),
	)
	effectiveVersion := scheduleVersion(dataVersionTs, weekNumber, boundary)
	// 边缘缓存（AstraSchedule/esa-edge-cache）靠这两个头判断能否不回源直接回 304。
	// 304 与 200 两条路径都要带上：边缘回源命中源站 304 时，同样需要元信息来刷新自己的条目。
	c.Header(cacheVersionHeader, effectiveVersion)
	c.Header(cacheExpireHeader, strconv.FormatInt(scheduleExpireAt(boundary, now), 10))
	if clientDataVersion == dataVersionTs && clientWeekNumber == weekNumber && clientBoundary == boundary {
		c.Status(http.StatusNotModified) // 304
		return
	}
	_, _ = db.RefreshAutorunStatusesNs(ns, now)

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
	// 倒数日记录已在上方按作用域过滤

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

const (
	// cacheVersionHeader / cacheExpireHeader 是给 ESA 边缘函数（AstraSchedule/esa-edge-cache）用的
	// 纯增量约定：不改变响应体结构，客户端不需要理解，边缘拿不到时退化为直接回源。
	cacheVersionHeader = "X-Astra-Schedule-Version"
	cacheExpireHeader  = "X-Astra-Schedule-Expire"
)

// scheduleExpireAt 返回该课表响应下一次必然失效的绝对时刻（Unix 秒），供边缘缓存判断
// 手上的版本元信息还能不能用来回 304。
//
// 响应内容除了随数据版本变化，还随时间变化，所以取两者的较早值：
//   - boundary：自动任务（单日/日期范围/cron）条件翻转的时刻；
//   - 下一个本地零点：跨天后星期不同，daily_class 是按「今天」应用规则后展开的；
//     周次与多周轮换课表也在周一变化，而周一同样是零点。
//
// 少了零点这一项，边缘会在午夜之后继续用前一天的版本判定 304，客户端整天拿不到新课表。
func scheduleExpireAt(boundary int64, now time.Time) int64 {
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)
	expire := midnight.Unix()
	if boundary > now.Unix() && boundary < expire {
		expire = boundary
	}
	return expire
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
