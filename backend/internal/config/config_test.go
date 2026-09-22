package config

import (
	"log/slog"
	"strings"
	"testing"
)

const validDatabaseURL = "postgres://ytauto:s3cret@localhost:5432/ytauto?sslmode=disable"

func fakeEnv(values map[string]string) LookupFunc {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func validEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL": validDatabaseURL,
		"REDIS_ADDR":   "localhost:6379",
	}
}

func TestLoadAppliesDefaultsWhenOptionalValuesAreMissing(t *testing.T) {
	cfg, err := Load(fakeEnv(validEnv()))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := Config{HTTPAddr: ":8080", DatabaseURL: validDatabaseURL, RedisAddr: "localhost:6379", LogLevel: slog.LevelInfo}
	if cfg != want {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestLoadUsesExplicitOptionalValues(t *testing.T) {
	env := validEnv()
	env["HTTP_ADDR"] = " :9090 "
	env["LOG_LEVEL"] = "debug"
	env["DATABASE_URL"] = "postgresql://db.internal/ytauto"

	cfg, err := Load(fakeEnv(env))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPAddr != ":9090" || cfg.LogLevel != slog.LevelDebug || cfg.DatabaseURL != "postgresql://db.internal/ytauto" {
		t.Fatalf("explicit values not applied: %+v", cfg)
	}
}

func TestLoadReportsEveryMissingRequiredValue(t *testing.T) {
	_, err := Load(fakeEnv(map[string]string{"REDIS_ADDR": "   "}))

	if err == nil {
		t.Fatal("expected an error")
	}
	for _, key := range []string{"DATABASE_URL is required", "REDIS_ADDR is required"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error %q does not mention %q", err, key)
		}
	}
}

func TestLoadRejectsInvalidDatabaseURLWithoutLeakingIt(t *testing.T) {
	cases := map[string]string{
		"wrong scheme":     "mysql://root:hunter2@localhost/db",
		"missing host":     "postgres:///ytauto",
		"unparseable":      "postgres://hunter2@%zz",
		"not a url at all": "hunter2",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			env := validEnv()
			env["DATABASE_URL"] = raw

			_, err := Load(fakeEnv(env))

			if err == nil || !strings.Contains(err.Error(), "DATABASE_URL must be a postgres:// URL") {
				t.Fatalf("expected DATABASE_URL validation error, got %v", err)
			}
			if strings.Contains(err.Error(), "hunter2") {
				t.Fatalf("error leaked the database URL: %v", err)
			}
		})
	}
}

func TestLoadRejectsUnknownLogLevel(t *testing.T) {
	env := validEnv()
	env["LOG_LEVEL"] = "verbose"

	_, err := Load(fakeEnv(env))

	if err == nil || !strings.Contains(err.Error(), `LOG_LEVEL "verbose"`) {
		t.Fatalf("expected LOG_LEVEL error, got %v", err)
	}
}
