// Package config loads and validates process configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
)

const (
	defaultHTTPAddr = ":8080"
	defaultLogLevel = "info"
)

// LookupFunc has the signature of os.LookupEnv so tests can supply a fake environment.
type LookupFunc func(key string) (string, bool)

// Config is the validated configuration shared by the API and worker processes.
type Config struct {
	HTTPAddr    string
	DatabaseURL string
	RedisAddr   string
	LogLevel    slog.Level
}

// Load reads configuration through lookup and reports every invalid or missing value at once,
// so a misconfigured deploy fails fast with one complete message instead of one error per restart.
func Load(lookup LookupFunc) (Config, error) {
	databaseURL, databaseErr := loadDatabaseURL(lookup)
	redisAddr, redisErr := require(lookup, "REDIS_ADDR")
	logLevel, logLevelErr := loadLogLevel(lookup)

	if err := errors.Join(databaseErr, redisErr, logLevelErr); err != nil {
		return Config{}, fmt.Errorf("invalid configuration: %w", err)
	}

	return Config{
		HTTPAddr:    valueOrDefault(lookup, "HTTP_ADDR", defaultHTTPAddr),
		DatabaseURL: databaseURL,
		RedisAddr:   redisAddr,
		LogLevel:    logLevel,
	}, nil
}

func require(lookup LookupFunc, key string) (string, error) {
	value, _ := lookup(key)
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func valueOrDefault(lookup LookupFunc, key, fallback string) string {
	value, err := require(lookup, key)
	if err != nil {
		return fallback
	}
	return value
}

// loadDatabaseURL never includes the raw value in its errors because the URL carries the DB password.
func loadDatabaseURL(lookup LookupFunc) (string, error) {
	raw, err := require(lookup, "DATABASE_URL")
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(raw)
	if err != nil || !isPostgresScheme(parsed.Scheme) || parsed.Host == "" {
		return "", errors.New("DATABASE_URL must be a postgres:// URL with a host")
	}
	return raw, nil
}

func isPostgresScheme(scheme string) bool {
	return scheme == "postgres" || scheme == "postgresql"
}

func loadLogLevel(lookup LookupFunc) (slog.Level, error) {
	raw := valueOrDefault(lookup, "LOG_LEVEL", defaultLogLevel)
	var level slog.Level
	if err := level.UnmarshalText([]byte(raw)); err != nil {
		return 0, fmt.Errorf("LOG_LEVEL %q must be one of debug, info, warn, error", raw)
	}
	return level, nil
}
