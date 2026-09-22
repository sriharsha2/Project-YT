package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sriharsha2/Project-YT/backend/internal/logging"
)

const (
	statusOK          = "ok"
	statusUnavailable = "unavailable"
)

// ReadyCheck probes one dependency (database, queue, ...) for the readiness endpoint.
type ReadyCheck struct {
	Name  string
	Probe func(ctx context.Context) error
}

type readinessResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// liveness only proves the process can serve HTTP; dependency health belongs to /ready
// so a database outage does not make the orchestrator restart healthy API pods.
func liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": statusOK})
}

func readiness(base *slog.Logger, checks []ReadyCheck, timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		logger := logging.FromContext(c.Request.Context(), base)
		response := readinessResponse{Status: statusOK, Checks: make(map[string]string, len(checks))}
		for _, check := range checks {
			response.Checks[check.Name] = statusOK
			if err := check.Probe(ctx); err != nil {
				logger.Warn("readiness check failed", "check", check.Name, "error", err.Error())
				response.Checks[check.Name] = statusUnavailable
				response.Status = statusUnavailable
			}
		}

		code := http.StatusOK
		if response.Status != statusOK {
			code = http.StatusServiceUnavailable
		}
		c.JSON(code, response)
	}
}
