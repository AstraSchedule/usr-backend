package startup

import (
	"os"
	"path/filepath"
	"testing"

	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model"

	"github.com/stretchr/testify/assert"
)

func TestReadConfig_Success(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configContent := `
[server]
host = "127.0.0.1"
port = 9000
domain = ["http://localhost"]

[db]
type = "sqlite"
path = ":memory:"

[secret]
token = "test-token"

[apikey]
apihost = "https://api.test.com"
weather = "test-key"

[log]
debug = true

[run]
serverless = false
`
	configPath := filepath.Join(tmpDir, "config.toml")
	os.WriteFile(configPath, []byte(configContent), 0644)

	// Change to temp dir
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Clear any existing config
	model.Configs = model.SrvConfig{}

	ReadConfig()

	assert.Equal(t, "127.0.0.1", model.Configs.Server.Host)
	assert.Equal(t, 9000, model.Configs.Server.Port)
}

func TestSetLog_DebugMode(t *testing.T) {
	model.Configs = model.SrvConfig{
		Log: model.LogConfig{
			Debug: true,
		},
	}

	// Should not panic
	SetLog()
}

func TestSetLog_ReleaseMode(t *testing.T) {
	model.Configs = model.SrvConfig{
		Log: model.LogConfig{
			Debug: false,
		},
	}

	// Should not panic
	SetLog()
}

func TestMigrateDb_Success(t *testing.T) {
	// Setup test config
	model.Configs = model.SrvConfig{
		Db: model.DbConfig{
			Type: "sqlite",
			Path: ":memory:",
		},
	}

	// Initialize DB
	db.GetDB()

	// Should not panic
	MigrateDb()
}
