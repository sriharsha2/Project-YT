package httpapi

import (
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sriharsha2/Project-YT/backend/internal/logging"
)

// RequestIDHeader is echoed on every response and accepted from callers so traces span services.
const RequestIDHeader = "X-Request-ID"

// Caller-supplied IDs are reflected into logs and headers, so only a short, safe alphabet is trusted.
var validRequestID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.Writer.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		header.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		header.Set("Referrer-Policy", "no-referrer")
		c.Next()
	}
}

func requestID(base *slog.Logger, newID func() string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if !validRequestID.MatchString(id) {
			id = newID()
		}
		c.Writer.Header().Set(RequestIDHeader, id)
		logger := base.With("request_id", id)
		c.Request = c.Request.WithContext(logging.WithLogger(c.Request.Context(), logger))
		c.Next()
	}
}

func accessLog(base *slog.Logger, now func() time.Time) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := now()
		c.Next()
		logging.FromContext(c.Request.Context(), base).Info("http request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", now().Sub(start).Milliseconds(),
		)
	}
}

// recoverPanics turns a panic into a request error so handleErrors answers with a generic 500 and logs it.
func recoverPanics() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, recovered any) {
		_ = c.Error(fmt.Errorf("panic: %v", recovered)) // gin.Context.Error only returns its argument.
		c.Abort()
	})
}
