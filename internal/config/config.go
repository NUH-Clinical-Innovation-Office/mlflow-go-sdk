// Package config handles application configuration loading and validation.
package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration
type Config struct {
	Server        ServerConfig
	Database      DatabaseConfig
	Auth          AuthConfig
	Logging       LoggingConfig
	Observability ObservabilityConfig
	RateLimit     RateLimitConfig
	CORS          CORSConfig
	Swagger       SwaggerConfig
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	TrustedProxies  []net.IPNet
}

// DatabaseConfig contains database connection settings
type DatabaseConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	ConnMaxLifetime time.Duration
}

// AuthConfig contains authentication settings
type AuthConfig struct {
	JWTSecretKey     string
	JWTExpireMinutes int
	BcryptCost       int
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level  string
	Format string
}

// ObservabilityConfig contains observability settings
type ObservabilityConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string
	SamplingRatio  float64
	TracingEnabled bool
	OTLPInsecure   bool
}

// RateLimitConfig contains rate limiting settings
type RateLimitConfig struct {
	Requests int
	Duration time.Duration
}

// CORSConfig contains CORS settings
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// SwaggerConfig contains swagger UI settings
type SwaggerConfig struct {
	Enabled bool
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	p := &envParser{}

	cfg := &Config{
		Server: ServerConfig{
			Host:            getEnv("SERVER_HOST", "0.0.0.0"),
			Port:            p.int("SERVER_PORT", 8080),
			ReadTimeout:     p.duration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:    p.duration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:     p.duration("SERVER_IDLE_TIMEOUT", 120*time.Second),
			ShutdownTimeout: p.duration("SERVER_SHUTDOWN_TIMEOUT", 10*time.Second),
			TrustedProxies:  parseTrustedProxies(getEnv("TRUSTED_PROXIES", "")),
		},
		Database: DatabaseConfig{
			URL:             getEnv("DATABASE_URL", ""),
			MaxConns:        int32(p.int("DATABASE_MAX_CONNS", 25)),
			MinConns:        int32(p.int("DATABASE_MIN_CONNS", 2)),
			ConnMaxLifetime: p.duration("DATABASE_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Auth: AuthConfig{
			JWTSecretKey:     getEnv("JWT_SECRET_KEY", ""),
			JWTExpireMinutes: p.int("JWT_EXPIRE_MINUTES", 1440),
			BcryptCost:       p.int("BCRYPT_COST", 12),
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
		Observability: ObservabilityConfig{
			TracingEnabled: p.bool("TRACING_ENABLED", true),
			ServiceName:    getEnv("SERVICE_NAME", "go-backend-template"),
			ServiceVersion: getEnv("SERVICE_VERSION", "1.0.0"),
			Environment:    getEnv("ENVIRONMENT", "development"),
			SamplingRatio:  p.float("OTEL_TRACE_SAMPLING_RATIO", 1.0),
			OTLPEndpoint:   getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
			OTLPInsecure:   p.bool("OTEL_EXPORTER_OTLP_INSECURE", true),
		},
		RateLimit: RateLimitConfig{
			Requests: p.int("RATE_LIMIT_REQUESTS", 10),
			Duration: p.duration("RATE_LIMIT_DURATION", time.Minute),
		},
		CORS: CORSConfig{
			AllowedOrigins:   getCommaSeparatedEnv("CORS_ALLOWED_ORIGINS", []string{}),
			AllowedMethods:   getCommaSeparatedEnv("CORS_ALLOWED_METHODS", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
			AllowedHeaders:   getCommaSeparatedEnv("CORS_ALLOWED_HEADERS", []string{"Accept", "Authorization", "Content-Type"}),
			AllowCredentials: p.bool("CORS_ALLOW_CREDENTIALS", true),
			MaxAge:           p.int("CORS_MAX_AGE", 3600),
		},
		Swagger: SwaggerConfig{
			Enabled: p.bool("SWAGGER_ENABLED", false),
		},
	}

	if p.err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", p.err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// envParser accumulates the first parse error encountered.
type envParser struct {
	err error
}

func (p *envParser) int(key string, defaultValue int) int {
	if p.err != nil {
		return defaultValue
	}
	value, err := getEnvAsIntOrError(key, defaultValue)
	if err != nil {
		p.err = err
	}
	return value
}

func (p *envParser) bool(key string, defaultValue bool) bool {
	if p.err != nil {
		return defaultValue
	}
	value, err := getEnvAsBoolOrError(key, defaultValue)
	if err != nil {
		p.err = err
	}
	return value
}

func (p *envParser) duration(key string, defaultValue time.Duration) time.Duration {
	if p.err != nil {
		return defaultValue
	}
	value, err := getEnvAsDurationOrError(key, defaultValue)
	if err != nil {
		p.err = err
	}
	return value
}

func (p *envParser) float(key string, defaultValue float64) float64 {
	if p.err != nil {
		return defaultValue
	}
	value, err := getEnvAsFloatOrError(key, defaultValue)
	if err != nil {
		p.err = err
	}
	return value
}

// Validate checks that required configuration is present
func (c *Config) Validate() error {
	if c.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.Auth.JWTSecretKey == "" {
		return fmt.Errorf("JWT_SECRET_KEY is required")
	}
	if len(c.Auth.JWTSecretKey) < 32 {
		return fmt.Errorf("JWT_SECRET_KEY must be at least 32 bytes (got %d); use a long random secret in production", len(c.Auth.JWTSecretKey))
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid SERVER_PORT: %d", c.Server.Port)
	}
	if c.CORS.AllowCredentials {
		for _, o := range c.CORS.AllowedOrigins {
			if o == "*" {
				return fmt.Errorf("CORS_ALLOWED_ORIGINS cannot contain '*' when CORS_ALLOW_CREDENTIALS=true (CORS spec forbids the combination)")
			}
		}
	}
	return nil
}

// parseTrustedProxies splits a comma-separated list of CIDRs into
// []*net.IPNet. Empty input returns nil. Invalid entries are silently
// dropped (operator can spot the empty list and fix the env).
func parseTrustedProxies(raw string) []net.IPNet {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]net.IPNet, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		_, n, err := net.ParseCIDR(p)
		if err != nil {
			continue
		}
		out = append(out, *n)
	}
	return out
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsIntOrError(key string, defaultValue int) (int, error) {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue, fmt.Errorf("%s=%q is not a valid integer", key, valueStr)
	}
	return value, nil
}

func getEnvAsBoolOrError(key string, defaultValue bool) (bool, error) {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue, nil
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue, fmt.Errorf("%s=%q is not a valid boolean", key, valueStr)
	}
	return value, nil
}

func getEnvAsDurationOrError(key string, defaultValue time.Duration) (time.Duration, error) {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue, nil
	}
	value, err := time.ParseDuration(valueStr)
	if err != nil {
		return defaultValue, fmt.Errorf("%s=%q is not a valid duration", key, valueStr)
	}
	return value, nil
}

func getEnvAsFloatOrError(key string, defaultValue float64) (float64, error) {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue, nil
	}
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return defaultValue, fmt.Errorf("%s=%q is not a valid float", key, valueStr)
	}
	return value, nil
}

func getCommaSeparatedEnv(key string, defaultValue []string) []string {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	parts := strings.Split(valueStr, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	if len(result) == 0 {
		return defaultValue
	}
	return result
}
