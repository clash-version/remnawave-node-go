package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/clash-version/remnawave-node-go/pkg/logger"
)

// Logger creates a logging middleware
func Logger(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get status code
		statusCode := c.Writer.Status()

		// Build path with query
		if raw != "" {
			path = path + "?" + raw
		}

		// Get client IP
		clientIP := c.ClientIP()

		// Log request
		log.Infow("Request",
			"status", statusCode,
			"method", c.Request.Method,
			"path", path,
			"ip", clientIP,
			"latency", latency.String(),
			"user-agent", c.Request.UserAgent(),
		)
	}
}

// Recovery creates a recovery middleware
func Recovery(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Errorw("Panic recovered",
					"error", err,
					"path", c.Request.URL.Path,
				)
				c.AbortWithStatus(500)
			}
		}()
		c.Next()
	}
}
