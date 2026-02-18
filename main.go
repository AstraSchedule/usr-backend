package main

import (
	"AstraScheduleServerGo/config"

	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{})
	configs, err := config.Load("./config.toml")
	if err != nil {
		logrus.Fatal(err)
		return
	}
	if configs.Log.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
	logrus.Info("成功读取到配置文件")
	logrus.Debug(configs)
}
