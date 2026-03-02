package model

type SrvConfig struct {
	APIKey APIKeyConfig `mapstructure:"apikey"`
	Secret SecretConfig `mapstructure:"secret"`
	Server ServerConfig `mapstructure:"server"`
	Db     DbConfig     `mapstructure:"db"`
	Log    LogConfig    `mapstructure:"log"`
	Run    RunConfig    `mapstructure:"run"`
}

type APIKeyConfig struct {
	Weather string `mapstructure:"weather"`
	APIHost string `mapstructure:"apihost"`
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
