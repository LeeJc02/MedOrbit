package middleware

import (
	"net/http"
	"time"

	"ddi/internal/authctx"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type jwtClaims struct {
	UserID   string   `json:"user_id,omitempty"`
	TenantID string   `json:"tenant_id"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

func JWTMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := bearerToken(c.GetHeader("Authorization"))
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid token"})
			return
		}

		claims := &jwtClaims{}
		token, err := jwt.ParseWithClaims(
			tokenString,
			claims,
			func(token *jwt.Token) (any, error) {
				return []byte(secret), nil
			},
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithExpirationRequired(),
			jwt.WithLeeway(30*time.Second),
		)
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		userID := claims.UserID
		if userID == "" {
			userID = claims.Subject
		}
		if userID == "" || claims.TenantID == "" || len(claims.Roles) == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			return
		}

		authClaims := authctx.Claims{UserID: userID, TenantID: claims.TenantID, Roles: claims.Roles}
		c.Set("user_id", authClaims.UserID)
		c.Set("tenant_id", authClaims.TenantID)
		c.Set("roles", authClaims.Roles)
		c.Request = c.Request.WithContext(authctx.WithClaims(c.Request.Context(), authClaims))
		c.Next()
	}
}

func bearerToken(auth string) string {
	const prefix = "Bearer "
	if len(auth) <= len(prefix) || auth[:len(prefix)] != prefix {
		return ""
	}
	return auth[len(prefix):]
}

func RBACMiddleware(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roles, _ := c.Get("roles")
		if !hasRole(roles, role) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}

func hasRole(raw any, role string) bool {
	roles, ok := raw.([]string)
	if !ok {
		return false
	}
	for _, candidate := range roles {
		if candidate == role {
			return true
		}
	}
	return false
}
