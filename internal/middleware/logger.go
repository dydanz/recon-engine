package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"recon-engine/pkg/logger"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// Process request
		c.Next()

		endTime := time.Now()
		latency := endTime.Sub(startTime)

		logger.GetLogger().WithFields(map[string]interface{}{
			"status":     c.Writer.Status(),
			"method":     c.Request.Method,
			"path":       c.Request.URL.Path,
			"query":      c.Request.URL.RawQuery,
			"ip":         c.ClientIP(),
			"user_agent": c.Request.UserAgent(),
			"latency":    latency.Milliseconds(),
			"errors":     c.Errors.String(),
		}).Info("Request processed")
	}
}
