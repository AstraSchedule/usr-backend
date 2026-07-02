package web

import (
	"net/http"
	"time"

	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model/dbTable"
	"AstraScheduleServerGo/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

type RegisterTenantRequest struct {
	Subdomain string `json:"subdomain" binding:"required"`
	Username  string `json:"username" binding:"required"`
	Password  string `json:"password" binding:"required"`
	School    string `json:"school" binding:"required"`
	Grade     string `json:"grade" binding:"required"`
	Class     string `json:"class" binding:"required"`
}

// RegisterTenant 内部接口：一键创建新租户（管理员 + 学校结构）
func RegisterTenant(c *gin.Context) {
	var req RegisterTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "参数不完整"})
		return
	}

	namespace := "cn/getastra/" + req.Subdomain

	// 1. 创建管理员用户
	hash, err := service.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "密码哈希失败"})
		return
	}

	admin := &dbTable.User{
		Namespace:          namespace,
		Username:           req.Username,
		PasswordHash:       hash,
		Role:               "admin",
		MustChangePwd:      true,
		MustChangeUsername: true,
	}
	if err := db.GetDB().Clauses(clause.OnConflict{DoNothing: true}).Create(admin).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "创建管理员失败: " + err.Error()})
		return
	}

	// 2. 创建学校（幂等）
	db.GetDB().Clauses(clause.OnConflict{DoNothing: true}).Create(&dbTable.Subject{
		Namespace: namespace,
		School:    req.School,
		Grade:     req.Grade,
		SubjectConfig: dbTable.SubjectConfig{
			SubjectName: map[string]string{
				"课": "课程", "自": "自习", "英": "英语", "语": "语文",
				"数": "数学", "物": "物理", "化": "化学", "体": "体育",
				"史": "历史", "政": "政治", "班": "班会",
			},
		},
	})

	// 3. 创建作息表
	db.GetDB().Clauses(clause.OnConflict{DoNothing: true}).Create(&dbTable.Timetable{
		Namespace: namespace,
		School:    req.School,
		Grade:     req.Grade,
		TimetableConfig: dbTable.TimetableConfig{
			Timetable: map[string]map[string]interface{}{
				"常日": {"00:00-00:00": 0, "00:01-23:59": "常日"},
				"没课": {"00:00-00:00": 0, "00:01-23:59": "没课"},
			},
			Divider: map[string][]int{"常日": {}, "没课": {}},
			Start:   time.Now().Format("2006-01-02"),
		},
	})

	// 4. 创建课表
	db.GetDB().Clauses(clause.OnConflict{DoNothing: true}).Create(&dbTable.Schedule{
		Namespace: namespace,
		School:    req.School,
		Grade:     req.Grade,
		Class:     req.Class,
		DailyClasses: [7]dbTable.DailyClass{
			{Chinese: "日", English: "SUN", Timetable: "没课", ClassList: dbTable.ClassList{[]string{"课"}}},
			{Chinese: "一", English: "MON", Timetable: "常日", ClassList: dbTable.ClassList{[]string{"课"}, []string{"课"}}},
			{Chinese: "二", English: "TUE", Timetable: "常日", ClassList: dbTable.ClassList{[]string{"课"}, []string{"课"}}},
			{Chinese: "三", English: "WED", Timetable: "常日", ClassList: dbTable.ClassList{[]string{"课"}, []string{"课"}}},
			{Chinese: "四", English: "THR", Timetable: "常日", ClassList: dbTable.ClassList{[]string{"课"}, []string{"课"}}},
			{Chinese: "五", English: "FRI", Timetable: "常日", ClassList: dbTable.ClassList{[]string{"课"}, []string{"课"}}},
			{Chinese: "六", English: "SAT", Timetable: "没课", ClassList: dbTable.ClassList{[]string{"课"}}},
		},
	})

	// 5. 创建客户端配置
	db.GetDB().Clauses(clause.OnConflict{DoNothing: true}).Create(&dbTable.ClientConfig{
		Namespace: namespace,
		School:    req.School,
		Grade:     req.Grade,
		Class:     req.Class,
		ClientConfigItems: dbTable.ClientConfigItems{
			CountdownTarget:      "hidden",
			WeatherAlertOverride: true,
			WeatherAlertBrief:    true,
			WeekDisplay:          true,
			StartupBehavior:      "normal",
		},
	})

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "租户创建成功",
		"namespace": namespace,
	})
}
