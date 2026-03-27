package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Recovery returns a middleware that recovers from panics
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the error and stack trace
				logPanic(err)

				// Return a clean error response
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
					"code":  "INTERNAL_ERROR",
				})

				c.Abort()
			}
		}()

		c.Next()
	}
}

func logPanic(err interface{}) {
	// Create structured error log
	errorLog := map[string]interface{}{
		"error":      fmt.Sprintf("%v", err),
		"stack":      string(debug.Stack()),
		"panic_type": "recovered",
	}

	if logJSON, marshalErr := json.Marshal(errorLog); marshalErr == nil {
		_, _ = gin.DefaultErrorWriter.Write(logJSON)
		_, _ = gin.DefaultErrorWriter.Write([]byte("\n"))
	}
}
