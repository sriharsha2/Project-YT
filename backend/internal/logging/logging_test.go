package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func decodeLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("log output is not JSON: %v (%q)", err, buf.String())
	}
	return line
}

func TestNewRedactsSensitiveAttributesAndKeepsOthers(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo)

	logger.Info("connected",
		"refresh_token", "1//abc", "Authorization", "Bearer x", "db_password", "p",
		"client_secret", "s", "session_cookie", "c", "DATABASE_URL", "postgres://u:p@h/d", "channel", "UC123")

	line := decodeLine(t, &buf)
	sensitiveKeys := []string{
		"refresh_token", "Authorization", "db_password", "client_secret", "session_cookie", "DATABASE_URL",
	}
	for _, key := range sensitiveKeys {
		if line[key] != RedactedValue {
			t.Errorf("%s = %v, want redacted", key, line[key])
		}
	}
	if line["channel"] != "UC123" || line["msg"] != "connected" {
		t.Errorf("non-sensitive fields changed: %v", line)
	}
}

func TestNewRedactsSensitiveAttributesInsideGroups(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo)

	logger.Info("oauth", slog.Group("google", "access_token", "ya29"))

	group, _ := decodeLine(t, &buf)["google"].(map[string]any)
	if group["access_token"] != RedactedValue {
		t.Fatalf("grouped token not redacted: %v", group)
	}
}

func TestNewSuppressesRecordsBelowTheConfiguredLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelWarn)

	logger.Info("ignored")

	if buf.Len() != 0 {
		t.Fatalf("expected no output, got %q", buf.String())
	}
}

func TestFromContextReturnsStoredLogger(t *testing.T) {
	stored := slog.New(slog.DiscardHandler)
	ctx := WithLogger(context.Background(), stored)

	if got := FromContext(ctx, slog.Default()); got != stored {
		t.Fatal("expected the stored logger")
	}
}

func TestFromContextReturnsFallbackWhenNoLoggerIsStored(t *testing.T) {
	fallback := slog.New(slog.DiscardHandler)

	if got := FromContext(context.Background(), fallback); got != fallback {
		t.Fatal("expected the fallback logger")
	}
}
