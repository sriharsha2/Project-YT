package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sriharsha2/Project-YT/backend/internal/apperr"
	"github.com/sriharsha2/Project-YT/backend/internal/logging"
)

const (
	generatedRequestID = "generated-id"
	clockStep          = 15 * time.Millisecond
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

type harness struct {
	router *gin.Engine
	logs   *bytes.Buffer
}

// steppingClock advances by clockStep on every call so request durations are deterministic.
func steppingClock() func() time.Time {
	current := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	return func() time.Time {
		current = current.Add(clockStep)
		return current
	}
}

func newHarness(checks ...ReadyCheck) harness {
	logs := &bytes.Buffer{}
	router := NewRouter(Deps{
		Logger:       logging.New(logs, slog.LevelDebug),
		ReadyChecks:  checks,
		NewRequestID: func() string { return generatedRequestID },
		Now:          steppingClock(),
	})
	return harness{router: router, logs: logs}
}

func (h harness) get(path string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	h.router.ServeHTTP(recorder, request)
	return recorder
}

func decodeBody[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var body T
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v (%q)", err, recorder.Body.String())
	}
	return body
}

func passingCheck(name string) ReadyCheck {
	return ReadyCheck{Name: name, Probe: func(context.Context) error { return nil }}
}

func TestHealthReturnsOKWithSecurityHeaders(t *testing.T) {
	recorder := newHarness().get("/health", nil)

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"status":"ok"`) {
		t.Fatalf("got %d %s", recorder.Code, recorder.Body.String())
	}
	for _, header := range []string{"X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy",
		"Strict-Transport-Security", "Referrer-Policy"} {
		if recorder.Header().Get(header) == "" {
			t.Errorf("missing security header %s", header)
		}
	}
}

func TestRequestIDIsGeneratedWhenCallerSendsNone(t *testing.T) {
	recorder := newHarness().get("/health", nil)

	if got := recorder.Header().Get(RequestIDHeader); got != generatedRequestID {
		t.Fatalf("request id = %q, want %q", got, generatedRequestID)
	}
}

func TestRequestIDIsReusedWhenCallerSendsAValidOne(t *testing.T) {
	recorder := newHarness().get("/health", map[string]string{RequestIDHeader: "abc_123-XYZ"})

	if got := recorder.Header().Get(RequestIDHeader); got != "abc_123-XYZ" {
		t.Fatalf("request id = %q", got)
	}
}

func TestRequestIDIsReplacedWhenCallerSendsAnUnsafeOne(t *testing.T) {
	for _, unsafe := range []string{"has space", "<script>", strings.Repeat("a", 65)} {
		recorder := newHarness().get("/health", map[string]string{RequestIDHeader: unsafe})

		if got := recorder.Header().Get(RequestIDHeader); got != generatedRequestID {
			t.Errorf("unsafe id %q was kept as %q", unsafe, got)
		}
	}
}

func TestAccessLogIncludesRequestIDStatusAndDuration(t *testing.T) {
	h := newHarness()

	h.get("/health", nil)

	var line map[string]any
	if err := json.Unmarshal(h.logs.Bytes(), &line); err != nil {
		t.Fatalf("access log is not a single JSON line: %v (%q)", err, h.logs.String())
	}
	if line["msg"] != "http request" || line["request_id"] != generatedRequestID ||
		line["status"] != float64(http.StatusOK) || line["path"] != "/health" ||
		line["duration_ms"] != float64(clockStep.Milliseconds()) {
		t.Fatalf("unexpected access log: %v", line)
	}
}

func TestReadyReturnsOKWhenEveryCheckPasses(t *testing.T) {
	recorder := newHarness(passingCheck("postgres"), passingCheck("redis")).get("/ready", nil)

	body := decodeBody[readinessResponse](t, recorder)
	if recorder.Code != http.StatusOK || body.Status != statusOK ||
		body.Checks["postgres"] != statusOK || body.Checks["redis"] != statusOK {
		t.Fatalf("got %d %+v", recorder.Code, body)
	}
}

func TestReadyReturns503AndHidesDetailsWhenACheckFails(t *testing.T) {
	failing := ReadyCheck{Name: "redis", Probe: func(context.Context) error {
		return errors.New("dial tcp 10.0.0.5:6379: connection refused")
	}}
	h := newHarness(passingCheck("postgres"), failing)

	recorder := h.get("/ready", nil)

	body := decodeBody[readinessResponse](t, recorder)
	if recorder.Code != http.StatusServiceUnavailable || body.Status != statusUnavailable ||
		body.Checks["redis"] != statusUnavailable || body.Checks["postgres"] != statusOK {
		t.Fatalf("got %d %+v", recorder.Code, body)
	}
	if strings.Contains(recorder.Body.String(), "10.0.0.5") {
		t.Fatal("response leaked internal error detail")
	}
	if !strings.Contains(h.logs.String(), "connection refused") {
		t.Fatal("failure detail was not logged")
	}
}

func TestReadyGivesChecksABoundedDeadline(t *testing.T) {
	var deadline time.Time
	var hasDeadline bool
	check := ReadyCheck{Name: "postgres", Probe: func(ctx context.Context) error {
		deadline, hasDeadline = ctx.Deadline()
		return nil
	}}

	newHarness(check).get("/ready", nil)

	if !hasDeadline || time.Until(deadline) > readinessTimeout {
		t.Fatalf("probe context deadline = %v (set: %v), want within %v", deadline, hasDeadline, readinessTimeout)
	}
}

func TestUnknownRouteReturnsNotFoundError(t *testing.T) {
	recorder := newHarness().get("/nope", nil)

	body := decodeBody[errorResponse](t, recorder)
	want := errorDetail{Code: "not_found", Message: "route not found", RequestID: generatedRequestID}
	if recorder.Code != http.StatusNotFound || body.Error != want {
		t.Fatalf("got %d %+v", recorder.Code, body)
	}
}

func TestErrorsAreMappedToResponsesWithoutLeakingInternals(t *testing.T) {
	cases := []struct {
		name       string
		handler    gin.HandlerFunc
		wantStatus int
		wantDetail errorDetail
		wantLogged string
	}{
		{"validation error", failWith(apperr.Validation("title is required")),
			http.StatusBadRequest, errorDetail{Code: "validation_failed", Message: "title is required"}, "title is required"},
		{"unclassified error", failWith(errors.New("pq: password=hunter2 rejected")),
			http.StatusInternalServerError, errorDetail{Code: "internal", Message: internalErrorMessage}, "hunter2"},
		{"internal app error", failWith(&apperr.Error{Kind: apperr.KindInternal, Message: "hunter2 leaked"}),
			http.StatusInternalServerError, errorDetail{Code: "internal", Message: internalErrorMessage}, "hunter2"},
		{"panic", func(*gin.Context) { panic("hunter2 exploded") },
			http.StatusInternalServerError, errorDetail{Code: "internal", Message: internalErrorMessage}, "hunter2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness()
			h.router.GET("/boom", tc.handler)

			recorder := h.get("/boom", nil)

			tc.wantDetail.RequestID = generatedRequestID
			body := decodeBody[errorResponse](t, recorder)
			if recorder.Code != tc.wantStatus || body.Error != tc.wantDetail {
				t.Fatalf("got %d %+v", recorder.Code, body)
			}
			if !strings.Contains(h.logs.String(), tc.wantLogged) {
				t.Fatal("error detail was not logged for operators")
			}
		})
	}
}

func failWith(err error) gin.HandlerFunc {
	return func(c *gin.Context) {
		_ = c.Error(err)
	}
}
