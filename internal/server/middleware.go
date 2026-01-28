package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates the X-Agent-Secret header.
func AuthMiddleware(expectedSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := c.GetHeader("X-Agent-Secret")
		if secret == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing X-Agent-Secret header",
			})
			return
		}

		if secret != expectedSecret {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid agent secret",
			})
			return
		}

		c.Next()
	}
}
