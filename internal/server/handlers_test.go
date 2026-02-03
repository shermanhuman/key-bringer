package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupTestHandler(agentSecret string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/unlock", AuthMiddleware(agentSecret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})))
	return mux
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	h := setupTestHandler("test-secret")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/unlock", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidSecret(t *testing.T) {
	h := setupTestHandler("test-secret")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/unlock", nil)
	req.Header.Set("X-Agent-Secret", "wrong-secret")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_ValidSecret(t *testing.T) {
	h := setupTestHandler("test-secret")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/unlock", nil)
	req.Header.Set("X-Agent-Secret", "test-secret")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
