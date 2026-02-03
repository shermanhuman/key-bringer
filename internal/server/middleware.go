package server

import (
	"encoding/json"
	"net/http"
)

// WriteJSON writes a JSON response.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// AuthMiddleware validates the X-Agent-Secret header.
func AuthMiddleware(expectedSecret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := r.Header.Get("X-Agent-Secret")
		if secret == "" {
			WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing X-Agent-Secret header"})
			return
		}
		if expectedSecret == "" || secret != expectedSecret {
			WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid agent secret"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
