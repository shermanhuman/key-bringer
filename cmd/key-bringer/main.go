package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Applesauce-Labs/key-bringer/internal/backends/gcp"
	"github.com/Applesauce-Labs/key-bringer/internal/config"
	"github.com/Applesauce-Labs/key-bringer/internal/core"
	"github.com/Applesauce-Labs/key-bringer/internal/notifiers/telnyx"
	"github.com/Applesauce-Labs/key-bringer/internal/server"
	"github.com/Applesauce-Labs/key-bringer/internal/totp"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if present (ignored in production)
	_ = godotenv.Load()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Get configuration from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	configPath := os.Getenv("KEYBRINGER_CONFIG")
	if configPath == "" {
		configPath = filepath.Join(".keybringer", "config.yaml")
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	secretStore, err := gcp.NewClient(ctx, cfg.GCP.ProjectID)
	if err != nil {
		logger.Error("failed to create secret store", "error", err)
		os.Exit(1)
	}
	defer secretStore.Close()

	// Fetch required secrets (pinned numeric versions).
	totpSeed, err := secretStore.GetSecret(ctx, cfg.Secrets.TOTPSeed)
	if err != nil {
		logger.Error("failed to fetch totp seed", "error", err)
		os.Exit(1)
	}
	adminPhone, err := secretStore.GetSecret(ctx, cfg.Secrets.AdminPhone)
	if err != nil {
		logger.Error("failed to fetch admin phone", "error", err)
		os.Exit(1)
	}
	telnyxAPIKey, err := secretStore.GetSecret(ctx, cfg.Secrets.TelnyxAPIKey)
	if err != nil {
		logger.Error("failed to fetch telnyx api key", "error", err)
		os.Exit(1)
	}
	telnyxFromNumber, err := secretStore.GetSecret(ctx, cfg.Secrets.TelnyxFromNumber)
	if err != nil {
		logger.Error("failed to fetch telnyx from number", "error", err)
		os.Exit(1)
	}
	telnyxPublicKey, err := secretStore.GetSecret(ctx, cfg.Secrets.TelnyxPublicKey)
	if err != nil {
		logger.Error("failed to fetch telnyx public key", "error", err)
		os.Exit(1)
	}

	verifier := totp.NewVerifier(totpSeed)
	telnyxClient := telnyx.NewClient(telnyxAPIKey, telnyxFromNumber, cfg.Telnyx.MessagingProfileID)

	webhookVerifier, err := telnyx.NewWebhookVerifier(telnyxPublicKey)
	if err != nil {
		logger.Error("failed to create webhook verifier", "error", err)
		os.Exit(1)
	}

	// Create router
	var agentSecretRef *core.SecretRef
	if cfg.Secrets.AgentSecret != nil {
		agentSecretRef = cfg.Secrets.AgentSecret
	}

	srvCfg := server.Config{
		RequireIAMAuth:    cfg.Runtime.RequireIAMAuth,
		IAMAudience:       cfg.Runtime.IAMAudience,
		AgentSecretRef:    agentSecretRef,
		AdminPhone:        adminPhone,
		ZFSMasterKey:      cfg.Secrets.ZFSMasterKey,
		PublicURL:         cfg.HTTP.PublicURL,
		MaxPendingMinutes: cfg.Runtime.MaxPendingMinutes,
		AllowedMachines:   cfg.Machines,
	}

	h := server.NewRouter(srvCfg, verifier, telnyxClient, telnyxClient, secretStore, webhookVerifier, logger)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	logger.Info("starting key-bringer", "port", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
