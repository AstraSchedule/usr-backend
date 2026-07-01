package model

import (
	"fmt"
	"strings"
)

type SrvConfig struct {
	APIKey   APIKeyConfig   `mapstructure:"apikey"`
	Secret   SecretConfig   `mapstructure:"secret"`
	Internal InternalConfig `mapstructure:"internal"`
	Server   ServerConfig   `mapstructure:"server"`
	Db       DbConfig       `mapstructure:"db"`
	Log      LogConfig      `mapstructure:"log"`
	Run      RunConfig      `mapstructure:"run"`
}

type InternalConfig struct {
	Secret string `mapstructure:"secret"`
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
	Type string `mapstructure:"type"`
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
	User string `mapstructure:"user"`
	Pass string `mapstructure:"pass"`
	Name string `mapstructure:"name"`
	Path string `mapstructure:"path"`
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
	if err := c.Db.Validate(); err != nil {
		return err
	}
	return nil
}

func (c DbConfig) Validate() error {
	dbType := strings.ToLower(strings.TrimSpace(c.Type))
	if dbType == "" {
		dbType = "mysql"
	}

	switch dbType {
	case "mysql":
		if strings.TrimSpace(c.Host) == "" {
			return fmt.Errorf("db.host 不能为空")
		}
		if c.Port <= 0 {
			return fmt.Errorf("db.port 必须大于 0")
		}
		if strings.TrimSpace(c.User) == "" {
			return fmt.Errorf("db.user 不能为空")
		}
		if strings.TrimSpace(c.Name) == "" {
			return fmt.Errorf("db.name 不能为空")
		}
	case "sqlite":
		if strings.TrimSpace(c.Path) == "" {
			return fmt.Errorf("db.path 不能为空（sqlite 模式）")
		}
	default:
		return fmt.Errorf("db.type 仅支持 mysql 或 sqlite")
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
