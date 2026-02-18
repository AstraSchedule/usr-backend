package config

import (
	"AstraScheduleServerGo/model"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func Load(path string) (*model.Config, error) {
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

	var cfg model.Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 可选：校验必填字段
	if cfg.APIKey.Weather == "" {
		logrus.Warn("apikey.weather 未配置，天气功能将不可用")
	}
	if cfg.Secret.Token == "" {
		logrus.Warn("secret.token 未配置，敏感操作无需认证即可被访问")
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
