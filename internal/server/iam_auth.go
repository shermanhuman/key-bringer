package server


import (
	"net/http"
	"strings"

	"google.golang.org/api/idtoken"
)

var allowedIssuers = map[string]struct{}{
	"accounts.google.com":        {},
	"https://accounts.google.com": {},
	"https://sts.googleapis.com":  {},
}

// RequireIDToken validates a Google-signed ID token.
//
// It accepts tokens from either Authorization or X-Serverless-Authorization.
func RequireIDToken(audience string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := bearerToken(r.Header.Get("X-Serverless-Authorization"))
		if tok == "" {
			tok = bearerToken(r.Header.Get("Authorization"))
		}
		if tok == "" {
			WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
			return
		}

		payload, err := idtoken.Validate(r.Context(), tok, audience)
		if err != nil {
			WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			return
		}
		if payload == nil {
			WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			return
		}
		if iss, _ := payload.Claims["iss"].(string); iss != "" {
			if _, ok := allowedIssuers[iss]; !ok {
				WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token issuer"})
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func bearerToken(headerVal string) string {
	if headerVal == "" {
		return ""
	}
	parts := strings.Fields(headerVal)
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}
