package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Applesauce-Labs/key-bringer/internal/core"
)

type fakeStore struct{ val string }

func (f fakeStore) GetSecret(ctx context.Context, ref core.SecretRef) (string, error) { return f.val, nil }

func TestRequireAgentSecret(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	ref := core.SecretRef{SecretID: "agent-secret", Version: 1}
	h := RequireAgentSecret(fakeStore{val: "expected"}, ref, next)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("X-Agent-Secret", "expected")
	h.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w2.Code)
	}
}
