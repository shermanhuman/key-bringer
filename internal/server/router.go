package server

import (
	"log/slog"

	"github.com/Applesauce-Labs/key-bringer/internal/core"
	"github.com/Applesauce-Labs/key-bringer/internal/telnyx"
	"github.com/gin-gonic/gin"
)

// Config holds server configuration.
type Config struct {
	AgentSecret string
	AdminPhone  string
	ZFSKeyName  string
}

// NewRouter creates a new Gin router with all routes configured.
func NewRouter(
	cfg Config,
	verifier core.Verifier,
	notifier core.Notifier,
	secretStore core.SecretStore,
	webhookVerifier *telnyx.WebhookVerifier,
	logger *slog.Logger,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	handler := NewHandler(
		verifier,
		notifier,
		secretStore,
		webhookVerifier,
		cfg.AdminPhone,
		cfg.ZFSKeyName,
		logger,
	)

	// API routes (require agent auth)
	api := r.Group("/api/v1")
	api.Use(AuthMiddleware(cfg.AgentSecret))
	{
		api.POST("/unlock", handler.HandleUnlock)
		api.POST("/poll", handler.HandlePoll)
	}

	// Webhook routes (authenticated via signature)
	r.POST("/webhooks/telnyx", handler.HandleWebhook)

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return r
}
