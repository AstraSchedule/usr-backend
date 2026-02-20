package main

import (
	"AstraScheduleServerGo/model"
	"AstraScheduleServerGo/router/client"
	"AstraScheduleServerGo/startup"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func main() {
	startup.StartInit()

	logrus.Infof("程序初始化流程结束，即将启动 HTTP 服务：%+v", model.Configs)

	router := gin.Default()
	authorized := router.Group("/", gin.BasicAuth(gin.Accounts{
		"AstraSchedule":         model.Configs.Secret.Token,
		"ElectronClassSchedule": model.Configs.Secret.Token, // 兼容旧版本客户端
	}))
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

	err := router.Run("0.0.0.0:9000")
	if err != nil {
		logrus.Fatal(err.Error())
		return
	}
}
