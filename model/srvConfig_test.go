package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSrvConfig_Validate_EmptyConfig(t *testing.T) {
	cfg := SrvConfig{}
	err := cfg.Validate()
	assert.Error(t, err)
}

func TestDbConfig_Validate_EmptyType(t *testing.T) {
	dbCfg := DbConfig{}
	err := dbCfg.Validate()
	// Empty type defaults to mysql, which requires host/port/user/name
	assert.Error(t, err)
}

func TestDbConfig_Validate_MissingMysqlFields(t *testing.T) {
	dbCfg := DbConfig{Type: "mysql"}
	err := dbCfg.Validate()
	assert.Error(t, err)
}

func TestDbConfig_Validate_MysqlValid(t *testing.T) {
	dbCfg := DbConfig{
		Type: "mysql",
		Host: "localhost",
		Port: 3306,
		User: "root",
		Name: "testdb",
	}
	err := dbCfg.Validate()
	assert.NoError(t, err)
}

func TestDbConfig_Validate_SqliteMissingPath(t *testing.T) {
	dbCfg := DbConfig{Type: "sqlite"}
	err := dbCfg.Validate()
	assert.Error(t, err)
}

func TestDbConfig_Validate_SqliteValid(t *testing.T) {
	dbCfg := DbConfig{
		Type: "sqlite",
		Path: "test.db",
	}
	err := dbCfg.Validate()
	assert.NoError(t, err)
}

func TestDbConfig_Validate_UnknownType(t *testing.T) {
	dbCfg := DbConfig{Type: "postgres"}
	err := dbCfg.Validate()
	assert.Error(t, err)
}

func TestAPIKeyConfig_Validate_MissingAPIHost(t *testing.T) {
	apiCfg := APIKeyConfig{}
	err := apiCfg.Validate()
	assert.Error(t, err)
}

func TestAPIKeyConfig_Validate_JWTExpiresOutOfRange(t *testing.T) {
	apiCfg := APIKeyConfig{
		APIHost: "https://example.com",
		JWT: JWTAuthConfig{
			KID:           "key1",
			ProjectID:     "proj1",
			PrivateKeyPEM: "pem",
			Expires:       0,
		},
	}
	err := apiCfg.Validate()
	// Expires=0 defaults to 900, which is valid
	assert.NoError(t, err)
}

func TestAPIKeyConfig_Validate_JWTExpiresTooHigh(t *testing.T) {
	apiCfg := APIKeyConfig{
		APIHost: "https://example.com",
		JWT: JWTAuthConfig{
			KID:           "key1",
			ProjectID:     "proj1",
			PrivateKeyPEM: "pem",
			Expires:       90000,
		},
	}
	err := apiCfg.Validate()
	assert.Error(t, err)
}

func TestAPIKeyConfig_Validate_Valid(t *testing.T) {
	apiCfg := APIKeyConfig{
		APIHost: "https://example.com",
		JWT: JWTAuthConfig{
			KID:           "key1",
			ProjectID:     "proj1",
			PrivateKeyPEM: "pem",
			Expires:       3600,
		},
	}
	err := apiCfg.Validate()
	assert.NoError(t, err)
}

func TestAPIKeyConfig_HasAPIKey(t *testing.T) {
	apiCfg := APIKeyConfig{Weather: "key123"}
	assert.True(t, apiCfg.HasAPIKey())

	apiCfgEmpty := APIKeyConfig{}
	assert.False(t, apiCfgEmpty.HasAPIKey())
}

func TestSrvConfig_WebSocketEnabled(t *testing.T) {
	cfg := SrvConfig{Run: RunConfig{Serverless: false}}
	assert.True(t, cfg.WebSocketEnabled())

	cfgServerless := SrvConfig{Run: RunConfig{Serverless: true}}
	assert.False(t, cfgServerless.WebSocketEnabled())
}
