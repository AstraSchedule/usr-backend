package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_NoConfigFile(t *testing.T) {
	// Change to a temp directory with no config files
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Clear any env vars that might interfere
	os.Unsetenv("ASTRA_SERVER_HOST")
	os.Unsetenv("ASTRA_DB_TYPE")

	cfg, err := Load("")
	// Should not error even without config file (uses env vars / defaults)
	// The Validate() will fail because required fields are missing
	if err != nil {
		assert.Contains(t, err.Error(), "配置校验失败")
	}
	_ = cfg
}

func TestLoad_TomlFile(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `
[server]
host = "0.0.0.0"
port = 8080
domain = ["http://test.com"]

[db]
type = "sqlite"
path = ":memory:"

[secret]
token = "test-token"

[apikey]
apihost = "https://api.test.com"
weather = "weather-key"

[log]
debug = true

[run]
serverless = false
`
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	cfg, err := Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "sqlite", cfg.Db.Type)
	assert.Equal(t, ":memory:", cfg.Db.Path)
	assert.Equal(t, "test-token", cfg.Secret.Token)
}

func TestLoad_YamlFile(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `
server:
  host: "127.0.0.1"
  port: 9000
  domain:
    - "http://localhost"
db:
  type: "sqlite"
  path: ":memory:"
secret:
  token: "yaml-token"
apikey:
  apihost: "https://api.test.com"
  weather: "yaml-key"
log:
  debug: false
run:
  serverless: false
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	cfg, err := Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, 9000, cfg.Server.Port)
	assert.Equal(t, "yaml-token", cfg.Secret.Token)
}

func TestLoad_EnvVars(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Set environment variables
	os.Setenv("ASTRA_SERVER_HOST", "env-host")
	os.Setenv("ASTRA_SERVER_PORT", "3000")
	os.Setenv("ASTRA_DB_TYPE", "sqlite")
	os.Setenv("ASTRA_DB_PATH", ":memory:")
	os.Setenv("ASTRA_SECRET_TOKEN", "env-token")
	os.Setenv("ASTRA_APIKEY_APIHOST", "https://env.api.com")
	os.Setenv("ASTRA_APIKEY_WEATHER", "env-weather-key")
	defer func() {
		os.Unsetenv("ASTRA_SERVER_HOST")
		os.Unsetenv("ASTRA_SERVER_PORT")
		os.Unsetenv("ASTRA_DB_TYPE")
		os.Unsetenv("ASTRA_DB_PATH")
		os.Unsetenv("ASTRA_SECRET_TOKEN")
		os.Unsetenv("ASTRA_APIKEY_APIHOST")
		os.Unsetenv("ASTRA_APIKEY_WEATHER")
	}()

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "env-host", cfg.Server.Host)
	assert.Equal(t, 3000, cfg.Server.Port)
	assert.Equal(t, "env-token", cfg.Secret.Token)
}

func TestLoad_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create invalid config
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte("invalid = [config"), 0644)
	require.NoError(t, err)

	_, err = Load(configPath)
	// Should error due to invalid config
	assert.Error(t, err)
}

func TestLoad_YmlFile(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `
server:
  host: "yml-host"
  port: 4000
  domain:
    - "http://yml.test"
db:
  type: "sqlite"
  path: ":memory:"
secret:
  token: "yml-token"
apikey:
  apihost: "https://yml.api.com"
  weather: "yml-key"
log:
  debug: false
run:
  serverless: false
`
	configPath := filepath.Join(tmpDir, "config.yml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	cfg, err := Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "yml-host", cfg.Server.Host)
	assert.Equal(t, 4000, cfg.Server.Port)
}

func TestLoad_JsonFile(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `{
  "server": {
    "host": "json-host",
    "port": 5000,
    "domain": ["http://json.test"]
  },
  "db": {
    "type": "sqlite",
    "path": ":memory:"
  },
  "secret": {
    "token": "json-token"
  },
  "apikey": {
    "apihost": "https://json.api.com",
    "weather": "json-key"
  },
  "log": {
    "debug": false
  },
  "run": {
    "serverless": false
  }
}`
	configPath := filepath.Join(tmpDir, "config.json")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	cfg, err := Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "json-host", cfg.Server.Host)
	assert.Equal(t, 5000, cfg.Server.Port)
}

func TestLoad_DomainOverride(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Set domain via env var
	os.Setenv("ASTRA_SERVER_DOMAIN", "http://env1.com, http://env2.com")
	os.Setenv("ASTRA_SERVER_HOST", "host")
	os.Setenv("ASTRA_SERVER_PORT", "80")
	os.Setenv("ASTRA_DB_TYPE", "sqlite")
	os.Setenv("ASTRA_DB_PATH", ":memory:")
	os.Setenv("ASTRA_SECRET_TOKEN", "token")
	os.Setenv("ASTRA_APIKEY_APIHOST", "https://api.com")
	defer func() {
		os.Unsetenv("ASTRA_SERVER_DOMAIN")
		os.Unsetenv("ASTRA_SERVER_HOST")
		os.Unsetenv("ASTRA_SERVER_PORT")
		os.Unsetenv("ASTRA_DB_TYPE")
		os.Unsetenv("ASTRA_DB_PATH")
		os.Unsetenv("ASTRA_SECRET_TOKEN")
		os.Unsetenv("ASTRA_APIKEY_APIHOST")
	}()

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, 2, len(cfg.Server.Domain))
}
