package config

import (
	"AstraScheduleServerGo/model"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var configCandidates = []struct {
	path string
	typ  string
}{
	{"config.toml", "toml"},
	{"config.yaml", "yaml"},
	{"config.yml", "yaml"},
	{"config.json", "json"},
	{".env", "env"},
}

func Load(path string) (*model.SrvConfig, error) {
	v := viper.New()

	v.SetEnvPrefix("ASTRA")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	loaded := false
	for _, c := range configCandidates {
		if _, err := os.Stat(c.path); err == nil {
			v.SetConfigFile(c.path)
			v.SetConfigType(c.typ)
			if err := v.ReadInConfig(); err != nil {
				logrus.Warnf("读取 %s 失败: %v", c.path, err)
				continue
			}
			logrus.Infof("已加载配置文件: %s", v.ConfigFileUsed())
			loaded = true
			break
		}
	}

	if !loaded && path != "" {
		if _, err := os.Stat(path); err == nil {
			ext := strings.TrimPrefix(path, ".")
			if idx := strings.LastIndex(ext, "."); idx >= 0 {
				ext = ext[idx+1:]
			}
			v.SetConfigFile(path)
			v.SetConfigType(ext)
			if err := v.ReadInConfig(); err == nil {
				logrus.Infof("已加载配置文件: %s", v.ConfigFileUsed())
				loaded = true
			}
		}
	}

	if !loaded {
		logrus.Info("未找到配置文件，使用环境变量配置")
	}

	// 手动注入环境变量，确保 Unmarshal 能读取
	envKeys := []string{
		"apikey.apihost", "apikey.weather",
		"apikey.jwt.kid", "apikey.jwt.project_id", "apikey.jwt.private_key_pem", "apikey.jwt.expires",
		"secret.token",
		"server.host", "server.port", "server.domain",
		"db.type", "db.host", "db.port", "db.user", "db.pass", "db.name", "db.path",
		"log.debug",
		"run.serverless",
	}
	for _, key := range envKeys {
		if v.GetString(key) != "" {
			v.Set(key, v.GetString(key))
		}
	}

	var cfg model.SrvConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	if envDomain := v.GetString("server.domain"); envDomain != "" && len(cfg.Server.Domain) <= 1 {
		parts := strings.Split(envDomain, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		cfg.Server.Domain = parts
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("配置校验失败: %w", err)
	}
	return &cfg, nil
}
