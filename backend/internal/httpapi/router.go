// Package httpapi wires HTTP routes and cross-cutting middleware for the API process.
package httpapi

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sriharsha2/Project-YT/backend/internal/apperr"
)

// readinessTimeout bounds the total time /ready spends probing dependencies so
// orchestrator probes never hang behind a stuck database connection.
const readinessTimeout = 2 * time.Second

// Deps holds the collaborators the router needs. Every I/O-facing value is injected for testability.
type Deps struct {
	Logger       *slog.Logger
	ReadyChecks  []ReadyCheck
	NewRequestID func() string
	Now          func() time.Time
}

// NewRouter builds the API's HTTP handler. Callers set gin's mode before calling it.
func NewRouter(deps Deps) *gin.Engine {
	router := gin.New()
	router.Use(
		securityHeaders(),
		requestID(deps.Logger, deps.NewRequestID),
		accessLog(deps.Logger, deps.Now),
		handleErrors(deps.Logger),
		recoverPanics(),
	)
	router.NoRoute(func(c *gin.Context) {
		_ = c.Error(apperr.NotFound("route not found")) // gin.Context.Error only returns its argument.
	})
	router.GET("/health", liveness)
	router.GET("/ready", readiness(deps.Logger, deps.ReadyChecks, readinessTimeout))
	return router
}
