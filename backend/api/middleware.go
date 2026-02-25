package api

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func InternalAuthMiddleware(expectedToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if expectedToken == "" {
			c.Next()
			return
		}

		auth := c.GetHeader("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		if token != expectedToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

func OAuthMiddleware(ac *AccessChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.GetHeader("X-Forwarded-User")
		if username == "" {
			log.Printf("Rejected request: missing X-Forwarded-User header from %s", c.ClientIP())
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "no authenticated user"})
			return
		}
		email := c.GetHeader("X-Forwarded-Email")

		var groups []string
		if gh := c.GetHeader("X-Forwarded-Groups"); gh != "" {
			for _, g := range strings.Split(gh, ",") {
				if t := strings.TrimSpace(g); t != "" {
					groups = append(groups, t)
				}
			}
		}

		if !ac.IsAllowed(username, email, groups) {
			log.Printf("Forbidden: user=%s email=%s groups=%v from %s", username, email, groups, c.ClientIP())
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		c.Set("username", username)
		c.Next()
	}
}
