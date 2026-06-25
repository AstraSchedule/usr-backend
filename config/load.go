package config

import (
	"AstraScheduleServerGo/model"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// configCandidates 按优先级从高到低列出候选配置文件
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

	// 1. 设置环境变量支持（最高优先级）
	v.SetEnvPrefix("ASTRA")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 2. 按优先级尝试读取配置文件
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

	// 如果指定了具体路径且候选列表中没有，尝试读取指定路径
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
		logrus.Warn("未找到任何配置文件，仅使用环境变量和默认值")
		if path != "" {
			writeDefaultConfig(path)
		}
	}

	// 3. 解析到结构体
	var cfg model.SrvConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 4. 后处理：env var 中的逗号分隔字符串转切片
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

func writeDefaultConfig(path string) {
	if err := godotenv.Write(map[string]string{}, path); err != nil {
		// .env 写不了就试 toml
		if _, err := os.Stat("config.template.toml"); err == nil {
			if err := os.Rename("config.template.toml", path); err != nil {
				logrus.Errorf("生成默认配置失败: %v", err)
				return
			}
		}
	}
	logrus.Infof("已生成默认配置文件: %s，请修改后重新启动", path)
}
