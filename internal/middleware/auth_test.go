package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func TestJWTMiddlewareRejectsMissingToken(t *testing.T) {
	status := authStatus(t, "", false)
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
}

func TestJWTMiddlewareRejectsBadFormat(t *testing.T) {
	status := authStatus(t, "Token abc", false)
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
}

func TestJWTMiddlewareRejectsBadSignature(t *testing.T) {
	status := authStatus(t, "Bearer "+signedToken(t, "wrong-secret", jwt.SigningMethodHS256, claims(jwt.NewNumericDate(time.Now().Add(time.Hour)), "tenant-1", []string{"doctor"})), false)
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
}

func TestJWTMiddlewareRejectsBadAlgorithm(t *testing.T) {
	status := authStatus(t, "Bearer "+signedToken(t, "secret", jwt.SigningMethodHS384, claims(jwt.NewNumericDate(time.Now().Add(time.Hour)), "tenant-1", []string{"doctor"})), false)
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
}

func TestJWTMiddlewareRejectsExpiredToken(t *testing.T) {
	status := authStatus(t, "Bearer "+signedToken(t, "secret", jwt.SigningMethodHS256, claims(jwt.NewNumericDate(time.Now().Add(-time.Hour)), "tenant-1", []string{"doctor"})), false)
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
}

func TestJWTMiddlewareRejectsMissingTenant(t *testing.T) {
	status := authStatus(t, "Bearer "+signedToken(t, "secret", jwt.SigningMethodHS256, claims(jwt.NewNumericDate(time.Now().Add(time.Hour)), "", []string{"doctor"})), false)
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
}

func TestJWTMiddlewareAllowsValidToken(t *testing.T) {
	status := authStatus(t, "Bearer "+validDoctorToken(t, "tenant-1"), false)
	if status != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", status, http.StatusNoContent)
	}
}

func TestRBACMiddlewareRejectsMissingDoctorRole(t *testing.T) {
	status := authStatus(t, "Bearer "+signedToken(t, "secret", jwt.SigningMethodHS256, claims(jwt.NewNumericDate(time.Now().Add(time.Hour)), "tenant-1", []string{"nurse"})), true)
	if status != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", status, http.StatusForbidden)
	}
}

func TestRBACMiddlewareAllowsDoctorRole(t *testing.T) {
	status := authStatus(t, "Bearer "+validDoctorToken(t, "tenant-1"), true)
	if status != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", status, http.StatusNoContent)
	}
}

func authStatus(t *testing.T, authHeader string, withRBAC bool) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(JWTMiddleware("secret"))
	if withRBAC {
		router.Use(RBACMiddleware("doctor"))
	}
	router.GET("/ok", func(c *gin.Context) {
		if userID, exists := c.Get("user_id"); !exists || userID != "u1" {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		if tenantID, exists := c.Get("tenant_id"); !exists || tenantID != "tenant-1" {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w.Code
}

func validDoctorToken(t *testing.T, tenantID string) string {
	t.Helper()
	return signedToken(t, "secret", jwt.SigningMethodHS256, claims(jwt.NewNumericDate(time.Now().Add(time.Hour)), tenantID, []string{"doctor"}))
}

func claims(expiresAt *jwt.NumericDate, tenantID string, roles []string) jwt.MapClaims {
	return jwt.MapClaims{
		"sub":       "u1",
		"tenant_id": tenantID,
		"roles":     roles,
		"exp":       expiresAt.Unix(),
	}
}

func signedToken(t *testing.T, secret string, method jwt.SigningMethod, claims jwt.Claims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(method, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}
