package web

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model/dbTable"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

func CreateSchool(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "学校名称不能为空"})
		return
	}

	// 检查是否已存在
	var count int64
	db.GetDB().Model(&dbTable.Schedule{}).Where("school = ?", req.Name).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"detail": "学校已存在"})
		return
	}

	// 创建一个空的默认记录来注册学校
	schedule := dbTable.Schedule{
		School: req.Name,
		Grade:  "default",
		Class:  "default",
		DailyClasses: [7]dbTable.DailyClass{
			{Chinese: "日", English: "SUN", Timetable: "没课"},
			{Chinese: "一", English: "MON", Timetable: "常日"},
			{Chinese: "二", English: "TUE", Timetable: "常日"},
			{Chinese: "三", English: "WED", Timetable: "常日"},
			{Chinese: "四", English: "THR", Timetable: "常日"},
			{Chinese: "五", English: "FRI", Timetable: "常日"},
			{Chinese: "六", English: "SAT", Timetable: "没课"},
		},
	}
	db.GetDB().Clauses(clause.OnConflict{DoNothing: true}).Create(&schedule)

	c.JSON(http.StatusOK, gin.H{"status": 200, "message": "学校创建成功"})
}

func DeleteSchool(c *gin.Context) {
	school := c.Param("school")
	if school == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "学校名称不能为空"})
		return
	}

	tx := db.GetDB().Begin()
	defer func() { if r := recover(); r != nil { tx.Rollback() } }()

	tx.Where("school = ?", school).Delete(&dbTable.Schedule{})
	tx.Where("school = ?", school).Delete(&dbTable.ClientConfig{})
	tx.Where("school = ?", school).Delete(&dbTable.DataVersion{})
	tx.Where("school = ?", school).Delete(&dbTable.Subject{})
	tx.Where("school = ?", school).Delete(&dbTable.Timetable{})
	tx.Where("school = ?", school).Delete(&dbTable.AutorunRecord{})

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": 200, "message": "学校已删除"})
}

func CreateGrade(c *gin.Context) {
	school := c.Param("school")
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "年级名称不能为空"})
		return
	}

	var count int64
	db.GetDB().Model(&dbTable.Schedule{}).Where("school = ? AND grade = ?", school, req.Name).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"detail": "年级已存在"})
		return
	}

	schedule := dbTable.Schedule{
		School: school,
		Grade:  req.Name,
		Class:  "default",
		DailyClasses: [7]dbTable.DailyClass{
			{Chinese: "日", English: "SUN", Timetable: "没课"},
			{Chinese: "一", English: "MON", Timetable: "常日"},
			{Chinese: "二", English: "TUE", Timetable: "常日"},
			{Chinese: "三", English: "WED", Timetable: "常日"},
			{Chinese: "四", English: "THR", Timetable: "常日"},
			{Chinese: "五", English: "FRI", Timetable: "常日"},
			{Chinese: "六", English: "SAT", Timetable: "没课"},
		},
	}
	db.GetDB().Clauses(clause.OnConflict{DoNothing: true}).Create(&schedule)

	// 创建默认科目和作息表
	subject := dbTable.Subject{
		School: school,
		Grade:  req.Name,
		SubjectConfig: dbTable.SubjectConfig{
			SubjectName: map[string]string{"课": "课程"},
		},
	}
	db.GetDB().Clauses(clause.OnConflict{DoNothing: true}).Create(&subject)

	timetable := dbTable.Timetable{
		School: school,
		Grade:  req.Name,
		TimetableConfig: dbTable.TimetableConfig{
			Timetable: map[string]map[string]interface{}{
				"常日": {"00:00-00:00": 0, "00:01-23:59": "尽量不要在部署阶段修改"},
				"没课": {"00:00-00:00": 0, "00:01-23:59": "没课"},
			},
			Divider: map[string][]int{"常日": {}, "没课": {}},
			Start:   time.Now().Format("2006-01-02"),
		},
	}
	db.GetDB().Clauses(clause.OnConflict{DoNothing: true}).Create(&timetable)

	c.JSON(http.StatusOK, gin.H{"status": 200, "message": "年级创建成功"})
}

func DeleteGrade(c *gin.Context) {
	school := c.Param("school")
	grade := c.Param("grade")

	tx := db.GetDB().Begin()
	defer func() { if r := recover(); r != nil { tx.Rollback() } }()

	tx.Where("school = ? AND grade = ?", school, grade).Delete(&dbTable.Schedule{})
	tx.Where("school = ? AND grade = ?", school, grade).Delete(&dbTable.ClientConfig{})
	tx.Where("school = ? AND grade = ?", school, grade).Delete(&dbTable.DataVersion{})
	tx.Where("school = ? AND grade = ?", school, grade).Delete(&dbTable.Subject{})
	tx.Where("school = ? AND grade = ?", school, grade).Delete(&dbTable.Timetable{})
	// 删除该年级下所有班级的自动任务
	tx.Where("school = ? AND grade = ?", school, grade).Delete(&dbTable.AutorunRecord{})

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": 200, "message": "年级已删除"})
}

func CreateClass(c *gin.Context) {
	school := c.Param("school")
	grade := c.Param("grade")
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "班级名称不能为空"})
		return
	}

	var count int64
	db.GetDB().Model(&dbTable.Schedule{}).Where("school = ? AND grade = ? AND class = ?", school, grade, req.Name).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"detail": "班级已存在"})
		return
	}

	schedule := dbTable.Schedule{
		School: school,
		Grade:  grade,
		Class:  req.Name,
		DailyClasses: [7]dbTable.DailyClass{
			{Chinese: "日", English: "SUN", Timetable: "没课"},
			{Chinese: "一", English: "MON", Timetable: "常日"},
			{Chinese: "二", English: "TUE", Timetable: "常日"},
			{Chinese: "三", English: "WED", Timetable: "常日"},
			{Chinese: "四", English: "THR", Timetable: "常日"},
			{Chinese: "五", English: "FRI", Timetable: "常日"},
			{Chinese: "六", English: "SAT", Timetable: "没课"},
		},
	}
	db.GetDB().Clauses(clause.OnConflict{DoNothing: true}).Create(&schedule)

	c.JSON(http.StatusOK, gin.H{"status": 200, "message": "班级创建成功"})
}

func DeleteClass(c *gin.Context) {
	school := c.Param("school")
	grade := c.Param("grade")
	classNumber := c.Param("class_number")

	tx := db.GetDB().Begin()
	defer func() { if r := recover(); r != nil { tx.Rollback() } }()

	tx.Where("school = ? AND grade = ? AND class = ?", school, grade, classNumber).Delete(&dbTable.Schedule{})
	tx.Where("school = ? AND grade = ? AND class = ?", school, grade, classNumber).Delete(&dbTable.ClientConfig{})
	tx.Where("school = ? AND grade = ? AND class = ?", school, grade, classNumber).Delete(&dbTable.DataVersion{})

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": 200, "message": "班级已删除"})
}
