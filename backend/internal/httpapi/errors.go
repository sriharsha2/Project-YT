package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sriharsha2/Project-YT/backend/internal/apperr"
	"github.com/sriharsha2/Project-YT/backend/internal/logging"
)

const internalErrorMessage = "something went wrong, please try again later"

type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}

// handleErrors is the single place that turns handler errors into HTTP responses.
// Unclassified errors become a generic 500 so internal details never reach clients.
func handleErrors(base *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}
		err := c.Errors.Last().Err
		status, detail := describeError(err)
		detail.RequestID = c.Writer.Header().Get(RequestIDHeader)

		logger := logging.FromContext(c.Request.Context(), base)
		if status >= http.StatusInternalServerError {
			logger.Error("request failed", "error", err.Error())
		} else {
			logger.Warn("request rejected", "error", err.Error(), "code", detail.Code)
		}
		c.AbortWithStatusJSON(status, errorResponse{Error: detail})
	}
}

func describeError(err error) (int, errorDetail) {
	appErr, ok := apperr.As(err)
	if !ok {
		return http.StatusInternalServerError, errorDetail{Code: "internal", Message: internalErrorMessage}
	}
	switch appErr.Kind {
	case apperr.KindValidation:
		return http.StatusBadRequest, errorDetail{Code: "validation_failed", Message: appErr.Message}
	case apperr.KindNotFound:
		return http.StatusNotFound, errorDetail{Code: "not_found", Message: appErr.Message}
	default:
		return http.StatusInternalServerError, errorDetail{Code: "internal", Message: internalErrorMessage}
	}
}
