package middleware

import (
	"net/http"

	"github.com/FluidifyAI/Regen/backend/internal/licence"
	"github.com/gin-gonic/gin"
)

// RequirePro aborts with 402 Payment Required if no valid Pro licence is active.
// Apply to route groups that contain Pro-only endpoints.
func RequirePro() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !licence.IsProEnabled() {
			c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{
				"error": "this feature requires a Fluidify Pro licence",
				"info":  "https://fluidify.ai/pro",
			})
			return
		}
		c.Next()
	}
}
