package middleware

import (
	"errors"
	"mindblog/internal/firebaseauth"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		token := strings.TrimPrefix(header, "Bearer ")
		email, err := firebaseauth.VerifyIDToken(token)
		if err != nil {
			// Token válido pero email no autorizado -> 403.
			if errors.Is(err, firebaseauth.ErrForbidden) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
			// Token ausente/inválido -> 401.
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("email", email)
		c.Next()
	}
}
