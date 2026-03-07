package config

import (
	"AstraScheduleServerGo/model"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func Load(path string) (*model.SrvConfig, error) {
	v := viper.New()
	v.SetConfigType("toml")
	v.SetConfigFile(path)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		logrus.Warnf("配置文件 %s 不存在\n", path)
		if err := writeDefaultConfig(path); err != nil {
			return nil, err
		}
		logrus.Warn("请修改 config.toml 在重新启动服务")
	}
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}

	var cfg model.SrvConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("配置校验失败: %w", err)
	}
	return &cfg, nil
}

func writeDefaultConfig(path string) error {
	err := os.Rename("config.template.toml", path)
	if err != nil {
		logrus.Fatal(err)
		return err
	}
	return nil
}
