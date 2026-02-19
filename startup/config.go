package startup

import (
	"AstraScheduleServerGo/config"
	"AstraScheduleServerGo/model"

	"github.com/sirupsen/logrus"
)

func ReadConfig() {
	configs, err := config.Load("./config.toml")
	if err != nil {
		logrus.Fatal(err)
	}
	model.Configs = *configs
}
