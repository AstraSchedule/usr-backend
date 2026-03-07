package model

import (
	"fmt"
	"strings"
)

type SrvConfig struct {
	APIKey APIKeyConfig `mapstructure:"apikey"`
	Secret SecretConfig `mapstructure:"secret"`
	Server ServerConfig `mapstructure:"server"`
	Db     DbConfig     `mapstructure:"db"`
	Log    LogConfig    `mapstructure:"log"`
	Run    RunConfig    `mapstructure:"run"`
}

type APIKeyConfig struct {
	Weather string        `mapstructure:"weather"`
	APIHost string        `mapstructure:"apihost"`
	JWT     JWTAuthConfig `mapstructure:"jwt"`
}

type JWTAuthConfig struct {
	KID           string `mapstructure:"kid"`
	ProjectID     string `mapstructure:"project_id"`
	PrivateKeyPEM string `mapstructure:"private_key_pem"`
	Expires       int64  `mapstructure:"expires"`
}

type SecretConfig struct {
	Token string `mapstructure:"token"` // Basic Auth 密钥
}

type ServerConfig struct {
	Host   string   `mapstructure:"host"`
	Port   int      `mapstructure:"port"`
	Domain []string `mapstructure:"domain"` // CORS 允许的域名列表
}

type DbConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
	User string `mapstructure:"user"`
	Pass string `mapstructure:"pass"`
	Name string `mapstructure:"name"`
}

type LogConfig struct {
	Debug bool `mapstructure:"debug"`
}

type RunConfig struct {
	Serverless bool `mapstructure:"serverless"`
}

func (c SrvConfig) WebSocketEnabled() bool {
	return !c.Run.Serverless
}

func (c SrvConfig) Validate() error {
	if err := c.APIKey.Validate(); err != nil {
		return err
	}
	return nil
}

func (c APIKeyConfig) Validate() error {
	host := strings.TrimSpace(c.APIHost)
	expires := c.JWT.Expires

	if host == "" {
		return fmt.Errorf("apikey.apihost 不能为空")
	}

	if expires == 0 {
		expires = 900
	}
	if expires < 1 || expires > 86400 {
		return fmt.Errorf("apikey.jwt.expires 必须在 1~86400 秒之间")
	}
	return nil
}

func (c APIKeyConfig) HasAPIKey() bool {
	return strings.TrimSpace(c.Weather) != ""
}

func (c APIKeyConfig) HasJWT() bool {
	return strings.TrimSpace(c.JWT.KID) != "" &&
		strings.TrimSpace(c.JWT.ProjectID) != "" &&
		strings.TrimSpace(c.JWT.PrivateKeyPEM) != ""
}
