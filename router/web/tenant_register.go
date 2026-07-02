package web

import (
	"net/http"
	"time"

	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/middleware"
	"AstraScheduleServerGo/model/dbTable"
	"AstraScheduleServerGo/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

// RegisterTenant 从已验证的 JWT 中读取注册信息，一键创建新租户
func RegisterTenant(c *gin.Context) {
	claims := middleware.GetRegClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "缺少注册令牌"})
		return
	}

	namespace := "cn/getastra/" + claims.Subdomain

	// 1. 创建管理员用户
	hash, err := service.HashPassword(claims.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "密码哈希失败"})
		return
	}

	admin := &dbTable.User{
		Namespace:          namespace,
		Username:           claims.Username,
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
		School:    claims.School,
		Grade:     claims.Grade,
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
		School:    claims.School,
		Grade:     claims.Grade,
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
		School:    claims.School,
		Grade:     claims.Grade,
		Class:     claims.Class,
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
		School:    claims.School,
		Grade:     claims.Grade,
		Class:     claims.Class,
		ClientConfigItems: dbTable.ClientConfigItems{
			CountdownTarget:      "hidden",
			WeatherAlertOverride: true,
			WeatherAlertBrief:    true,
			WeekDisplay:          true,
			StartupBehavior:      "normal",
		},
	})

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"message":  "租户创建成功",
		"namespace": namespace,
	})
}
