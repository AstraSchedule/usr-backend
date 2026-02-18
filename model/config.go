package model

type Config struct {
	APIKey APIKeyConfig `mapstructure:"apikey"`
	Secret SecretConfig `mapstructure:"secret"`
	Server ServerConfig `mapstructure:"server"`
	Log    LogConfig    `mapstructure:"log"`
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

type LogConfig struct {
	Debug bool `mapstructure:"debug"`
}
