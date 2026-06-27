package web

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/middleware"
	"AstraScheduleServerGo/service"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/daizihan233/go-valence-cal"
	"github.com/gin-gonic/gin"
)

const (
	dateFormat     = "2006-01-02"
	invalidDateErr = "日期参数格式错误"
)

// parseDateFromParams 从 URL 参数中提取并校验年月日，返回格式化后的日期字符串
func parseDateFromParams(c *gin.Context) (string, bool) {
	year, err1 := strconv.Atoi(c.Param("year"))
	month, err2 := strconv.Atoi(c.Param("month"))
	day, err3 := strconv.Atoi(c.Param("day"))
	if err1 != nil || err2 != nil || err3 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": invalidDateErr})
		return "", false
	}

	dateStr := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	if _, err := time.Parse(dateFormat, dateStr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": invalidDateErr})
		return "", false
	}

	return dateStr, true
}

func compensationFromQuery(c *gin.Context, queryFn func(string) (string, bool)) {
	dateStr, ok := parseDateFromParams(c)
	if !ok {
		return
	}

	result, exists := queryFn(dateStr)
	var compensation any
	if exists {
		compensation = result
	}

	c.JSON(http.StatusOK, gin.H{
		"date":         dateStr,
		"compensation": compensation,
	})
}

func CompensationFromHoliday(c *gin.Context) {
	compensationFromQuery(c, valence.CompensationFromHoliday)
}

func CompensationFromWorkday(c *gin.Context) {
	compensationFromQuery(c, valence.CompensationFromWorkday)
}

func CompensationFromYear(c *gin.Context) {
	yearStr := c.Param("year")
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "年份参数格式错误"})
		return
	}

	pairs := valence.CompensationPairs(year)

	result := make([]gin.H, len(pairs))
	for idx, p := range pairs {
		result[idx] = gin.H{
			"holiday": p.Holiday,
			"workday": p.Workday,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"year":  year,
		"pairs": result,
	})
}

func GetScheduleByDate(c *gin.Context) {
	ns := middleware.GetNamespace(c)
	dateStr := c.Query("date")
	scope := c.Query("scope")
	dateObj, err := time.Parse(dateFormat, dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的日期格式，应为 YYYY-MM-DD"})
		return
	}
	school, grade, classNumber, ok := parseScope(scope)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 scope 或配置缺失"})
		return
	}
	schedule := db.GetScheduleNs(ns, school, grade, classNumber)
	timetable := db.GetTimetableNs(ns, school, grade)
	_, _ = db.RefreshAutorunStatusesNs(ns, time.Now())
	records, _ := db.FetchAutorunRecordsNs(ns, "")
	resolved := service.ApplyScheduleRules(schedule.DailyClasses, timetable.Timetable, records, school, grade, classNumber, dateObj)
	periods := service.BuildPeriodsForDate(resolved, timetable.Timetable, dateObj)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"periods": periods}})
}
