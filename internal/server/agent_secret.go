package server

import (
	"net/http"

	"github.com/Applesauce-Labs/key-bringer/internal/core"
)

// RequireAgentSecret enforces X-Agent-Secret if configured.
//
// The expected secret value is fetched from Secret Manager at check time.
func RequireAgentSecret(secretStore core.SecretStore, ref core.SecretRef, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := r.Header.Get("X-Agent-Secret")
		if provided == "" {
			WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing X-Agent-Secret header"})
			return
		}
		expected, err := secretStore.GetSecret(r.Context(), ref)
		if err != nil {
			WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid agent secret"})
			return
		}
		if provided != expected {
			WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid agent secret"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
