package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestRouter(agentSecret string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	api := r.Group("/api/v1")
	api.Use(AuthMiddleware(agentSecret))
	{
		api.POST("/unlock", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})
	}

	return r
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	router := setupTestRouter("test-secret")

	req := httptest.NewRequest("POST", "/api/v1/unlock", strings.NewReader(`{"machine_id":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidSecret(t *testing.T) {
	router := setupTestRouter("test-secret")

	req := httptest.NewRequest("POST", "/api/v1/unlock", strings.NewReader(`{"machine_id":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Secret", "wrong-secret")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_ValidSecret(t *testing.T) {
	router := setupTestRouter("test-secret")

	req := httptest.NewRequest("POST", "/api/v1/unlock", strings.NewReader(`{"machine_id":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Secret", "test-secret")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
