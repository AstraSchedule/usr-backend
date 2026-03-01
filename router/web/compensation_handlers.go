package web

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/service"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/daizihan233/go-valence-cal"
	"github.com/gin-gonic/gin"
)

func CompensationFromHoliday(c *gin.Context) {
	year, err1 := strconv.Atoi(c.Param("year"))
	month, err2 := strconv.Atoi(c.Param("month"))
	day, err3 := strconv.Atoi(c.Param("day"))
	if err1 != nil || err2 != nil || err3 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "日期参数格式错误"})
		return
	}

	dateStr := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	if _, err := time.Parse("2006-01-02", dateStr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "日期参数格式错误"})
		return
	}

	workday, exists := valence.CompensationFromHoliday(dateStr)
	var compensation interface{}
	if exists {
		compensation = workday
	} else {
		compensation = nil
	}

	c.JSON(http.StatusOK, gin.H{
		"date":         dateStr,
		"compensation": compensation,
	})
}

func CompensationFromWorkday(c *gin.Context) {
	year, err1 := strconv.Atoi(c.Param("year"))
	month, err2 := strconv.Atoi(c.Param("month"))
	day, err3 := strconv.Atoi(c.Param("day"))
	if err1 != nil || err2 != nil || err3 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "日期参数格式错误"})
		return
	}

	dateStr := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	if _, err := time.Parse("2006-01-02", dateStr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "日期参数格式错误"})
		return
	}

	holiday, exists := valence.CompensationFromWorkday(dateStr)
	var compensation interface{}
	if exists {
		compensation = holiday
	} else {
		compensation = nil
	}

	c.JSON(http.StatusOK, gin.H{
		"date":         dateStr,
		"compensation": compensation,
	})
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
	dateStr := c.Query("date")
	scope := c.Query("scope")
	dateObj, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的日期格式，应为 YYYY-MM-DD"})
		return
	}
	school, grade, classNumber, ok := parseScope(scope)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 scope 或配置缺失"})
		return
	}
	schedule := db.GetSchedule(school, grade, classNumber)
	timetable := db.GetTimetable(school, grade)
	_, _ = db.RefreshAutorunStatuses(time.Now())
	records, _ := db.FetchAutorunRecords("")
	resolved := service.ApplyScheduleRules(schedule.DailyClasses, timetable.Timetable, records, school, grade, classNumber, dateObj)
	periods := service.BuildPeriodsForDate(resolved, timetable.Timetable, dateObj)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"periods": periods}})
}
