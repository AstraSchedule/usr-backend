package main

import (
	"AstraScheduleServerGo/model"
	"AstraScheduleServerGo/router/client"
	"AstraScheduleServerGo/router/web"
	"AstraScheduleServerGo/startup"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func main() {
	startup.StartInit()

	logrus.Infof("程序初始化流程结束，即将启动 HTTP 服务：%+v", model.Configs)

	router := gin.Default()
	router.Use(cors.New(cors.Config{
		AllowOrigins:     model.Configs.Server.Domain,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type", "Accept", "Origin", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	authorized := router.Group("/", gin.BasicAuth(gin.Accounts{
		"AstraSchedule":         model.Configs.Secret.Token,
		"ElectronClassSchedule": model.Configs.Secret.Token, // 兼容旧版本客户端
	}))

	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Hello World",
		})
	})

	// 完整更新课表
	authorized.PUT("/:school/:grade/:class", client.PutSchedule)
	// 获取完整课表
	router.GET("/:school/:grade/:class", client.GetSchedule)
	// 通过省份和城市查询天气
	router.GET("/api/weather/:name1/:name2", client.GetWeatherWithProvince)
	// 通过省份和城市查询天气
	router.GET("/api/weather/:name1", client.GetWeatherWithCity)
	// 通过 CF 头查询天气
	router.GET("/api/weather/", client.GetWeatherWithCFHeader)
	// WebSocket（当前仅占位）
	router.Any("/ws/:school/:grade/:class_number", client.WebSocketPlaceholder)
	// 广播（当前仅占位）
	authorized.POST("/api/broadcast/:school/:grade/:class_number", client.BroadcastSyncConfig)

	// 统计/菜单/结构
	router.GET("/web/statistic", web.GetStatistic)
	router.GET("/web/menu", web.GetMenu)
	router.GET("/web/structure", web.GetStructure)

	// 配置接口
	router.GET("/web/config/:school/:grade/subjects/options", web.GetSubjectsOptions)
	router.GET("/web/config/:school/:grade/subjects", web.GetSubjects)
	authorized.PUT("/web/config/:school/:grade/subjects", web.PutSubjects)

	router.GET("/web/config/:school/:grade/timetable/options", web.GetTimetableOptions)
	router.GET("/web/config/:school/:grade/timetable", web.GetTimetable)
	authorized.PUT("/web/config/:school/:grade/timetable", web.PutTimetable)

	router.GET("/web/config/:school/:grade/:class_number/schedule", web.GetScheduleConfig)
	authorized.PUT("/web/config/:school/:grade/:class_number/schedule", web.PutScheduleConfig)

	router.GET("/web/config/:school/:grade/:class_number/settings", web.GetSettings)
	authorized.PUT("/web/config/:school/:grade/:class_number/settings", web.PutSettings)

	// 自动任务
	router.GET("/web/autorun", web.GetAutorunStatus)
	router.GET("/web/autorun/hash/:hashid", web.GetAutorunHashStatus)
	authorized.DELETE("/web/autorun/:hashid", web.DeleteAutorunRecord)
	authorized.PUT("/web/autorun/compensation", web.PutCompensationRule)
	authorized.PUT("/web/autorun/timetable", web.PutTimetableRule)
	authorized.PUT("/web/autorun/schedule", web.PutScheduleRule)
	authorized.PUT("/web/autorun/all", web.PutAllRule)

	// 调休计算（预留）
	router.GET("/web/autorun/compensation/holiday/:year/:month/:day", web.CompensationFromHoliday)
	router.GET("/web/autorun/compensation/workday/:year/:month/:day", web.CompensationFromWorkday)
	router.GET("/web/autorun/compensation/year/:year", web.CompensationFromYear)

	// 按日期出课节
	router.GET("/web/schedule/by-date", web.GetScheduleByDate)

	err := router.Run("0.0.0.0:9000")
	if err != nil {
		logrus.Fatal(err.Error())
		return
	}
}
