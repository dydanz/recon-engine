package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"recon-engine/pkg/logger"
	"recon-engine/pkg/response"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.GetLogger().WithField("error", err).Error("Panic recovered")
				response.InternalError(c, "Internal server error", "An unexpected error occurred")
				c.Abort()
			}
		}()
		c.Next()
	}
}

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Handle any errors that were set during request processing
		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			logger.GetLogger().WithError(err.Err).Error("Request error")

			// Only send error response if not already sent
			if c.Writer.Status() == http.StatusOK {
				response.InternalError(c, "Request failed", err.Error())
			}
		}
	}
}
