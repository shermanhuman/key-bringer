package server

import (
	"log/slog"
	"net/http"

	"github.com/Applesauce-Labs/key-bringer/internal/core"
	"github.com/Applesauce-Labs/key-bringer/internal/notifiers/telnyx"
)

// Config holds server configuration.
type Config struct {
	AgentSecret string
	AdminPhone  string
	ZFSKeyName  string
	PublicURL   string
}

// NewRouter creates a new stdlib router with all routes configured.
func NewRouter(
	cfg Config,
	verifier core.Verifier,
	notifier core.Notifier,
	webhookURLUpdater core.WebhookURLUpdater,
	secretStore core.SecretStore,
	webhookVerifier *telnyx.WebhookVerifier,
	logger *slog.Logger,
) http.Handler {
	mux := http.NewServeMux()

	handler := NewHandler(
		verifier,
		notifier,
		webhookURLUpdater,
		secretStore,
		webhookVerifier,
		cfg.AdminPhone,
		cfg.ZFSKeyName,
		cfg.PublicURL,
		logger,
	)

	// API routes (require agent auth)
	mux.Handle("POST /api/v1/unlock", AuthMiddleware(cfg.AgentSecret, http.HandlerFunc(handler.HandleUnlock)))
	mux.Handle("POST /api/v1/poll", AuthMiddleware(cfg.AgentSecret, http.HandlerFunc(handler.HandlePoll)))

	// Webhook routes (authenticated via signature)
	mux.HandleFunc("GET /webhooks/telnyx/{token}", handler.HandleWebhookProbe)
	mux.HandleFunc("POST /webhooks/telnyx/{token}", handler.HandleWebhookTokenized)
	// Legacy un-tokenized route (kept for initial bring-up; rejected once token rotation is active)
	mux.HandleFunc("POST /webhooks/telnyx", handler.HandleWebhookLegacy)

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	return mux
}
