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

	authorized.PUT("/:school/:grade/:class", client.PutSchedule)
	router.GET("/:school/:grade/:class", client.GetSchedule)

	err := router.Run("0.0.0.0:9000")
	if err != nil {
		logrus.Fatal(err.Error())
		return
	}
}
