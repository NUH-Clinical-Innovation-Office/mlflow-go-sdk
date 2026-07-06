package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	// Set required env vars
	os.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")
	os.Setenv("JWT_SECRET_KEY", "test-secret-test-secret-test-secret-32+")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("JWT_SECRET_KEY")
	}()

	cfg, err := Load()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "test-secret-test-secret-test-secret-32+", cfg.Auth.JWTSecretKey)
}

func TestLoad_RequiredFields(t *testing.T) {
	// Clear required env vars
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("JWT_SECRET_KEY")

	cfg, err := Load()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "DATABASE_URL is required")
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &Config{
				Database: DatabaseConfig{URL: "postgres://localhost/db"},
				Auth:     AuthConfig{JWTSecretKey: "this-is-a-32-byte-test-secret-32!"},
				Server:   ServerConfig{Port: 8080},
			},
			wantErr: false,
		},
		{
			name: "missing database url",
			cfg: &Config{
				Database: DatabaseConfig{URL: ""},
				Auth:     AuthConfig{JWTSecretKey: "this-is-a-32-byte-test-secret-32!"},
				Server:   ServerConfig{Port: 8080},
			},
			wantErr: true,
		},
		{
			name: "missing jwt secret",
			cfg: &Config{
				Database: DatabaseConfig{URL: "postgres://localhost/db"},
				Auth:     AuthConfig{JWTSecretKey: ""},
				Server:   ServerConfig{Port: 8080},
			},
			wantErr: true,
		},
		{
			name: "invalid port",
			cfg: &Config{
				Database: DatabaseConfig{URL: "postgres://localhost/db"},
				Auth:     AuthConfig{JWTSecretKey: "this-is-a-32-byte-test-secret-32!"},
				Server:   ServerConfig{Port: 0},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	os.Setenv("TEST_KEY", "test_value")
	defer os.Unsetenv("TEST_KEY")

	assert.Equal(t, "test_value", getEnv("TEST_KEY", "default"))
	assert.Equal(t, "default", getEnv("NONEXISTENT_KEY", "default"))
}

func TestGetEnvAsIntOrError(t *testing.T) {
	os.Setenv("TEST_INT", "42")
	defer os.Unsetenv("TEST_INT")

	val, err := getEnvAsIntOrError("TEST_INT", 0)
	assert.NoError(t, err)
	assert.Equal(t, 42, val)

	val, err = getEnvAsIntOrError("NONEXISTENT", 10)
	assert.NoError(t, err)
	assert.Equal(t, 10, val)

	os.Setenv("TEST_INT_INVALID", "not_a_number")
	defer os.Unsetenv("TEST_INT_INVALID")
	_, err = getEnvAsIntOrError("TEST_INT_INVALID", 0)
	assert.Error(t, err)
}

func TestGetEnvAsBoolOrError(t *testing.T) {
	os.Setenv("TEST_BOOL", "true")
	defer os.Unsetenv("TEST_BOOL")

	val, err := getEnvAsBoolOrError("TEST_BOOL", false)
	assert.NoError(t, err)
	assert.True(t, val)

	val, err = getEnvAsBoolOrError("NONEXISTENT", true)
	assert.NoError(t, err)
	assert.True(t, val)
}

func TestGetEnvAsDurationOrError(t *testing.T) {
	os.Setenv("TEST_DURATION", "30s")
	defer os.Unsetenv("TEST_DURATION")

	val, err := getEnvAsDurationOrError("TEST_DURATION", time.Minute)
	assert.NoError(t, err)
	assert.Equal(t, 30*time.Second, val)

	val, err = getEnvAsDurationOrError("NONEXISTENT", time.Hour)
	assert.NoError(t, err)
	assert.Equal(t, time.Hour, val)
}

func TestGetEnvAsFloatOrError(t *testing.T) {
	os.Setenv("TEST_FLOAT", "3.14")
	defer os.Unsetenv("TEST_FLOAT")

	val, err := getEnvAsFloatOrError("TEST_FLOAT", 1.0)
	assert.NoError(t, err)
	assert.InDelta(t, 3.14, val, 0.01)
}

func TestGetCommaSeparatedEnv(t *testing.T) {
	os.Setenv("TEST_LIST", "a,b,c")
	defer os.Unsetenv("TEST_LIST")

	result := getCommaSeparatedEnv("TEST_LIST", []string{})
	assert.Equal(t, []string{"a", "b", "c"}, result)

	result = getCommaSeparatedEnv("TEST_LIST_WITH_SPACES", []string{})
	assert.Equal(t, []string{}, result)

	os.Setenv("TEST_LIST_WITH_SPACES", " a , b , c ")
	result = getCommaSeparatedEnv("TEST_LIST_WITH_SPACES", []string{})
	assert.Equal(t, []string{"a", "b", "c"}, result)
}

func TestSwaggerConfig(t *testing.T) {
	t.Run("defaults to disabled", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://x:x@localhost/x")
		t.Setenv("JWT_SECRET_KEY", "this-is-a-32-byte-test-secret-32!")
		cfg, err := Load()
		require.NoError(t, err)
		assert.False(t, cfg.Swagger.Enabled)
	})

	t.Run("enabled when SWAGGER_ENABLED=true", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://x:x@localhost/x")
		t.Setenv("JWT_SECRET_KEY", "this-is-a-32-byte-test-secret-32!")
		t.Setenv("SWAGGER_ENABLED", "true")
		cfg, err := Load()
		require.NoError(t, err)
		assert.True(t, cfg.Swagger.Enabled)
	})
}
