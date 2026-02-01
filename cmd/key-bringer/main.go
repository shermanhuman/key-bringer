package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/Applesauce-Labs/key-bringer/internal/backends/gcp"
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

	agentSecret := os.Getenv("AGENT_SECRET")
	if agentSecret == "" {
		logger.Error("AGENT_SECRET is required")
		os.Exit(1)
	}

	totpSeed := os.Getenv("TOTP_SEED")
	if totpSeed == "" {
		logger.Error("TOTP_SEED is required")
		os.Exit(1)
	}

	telnyxAPIKey := os.Getenv("TELNYX_API_KEY")
	telnyxFromNumber := os.Getenv("TELNYX_FROM_NUMBER")
	telnyxPublicKey := os.Getenv("TELNYX_PUBLIC_KEY")
	adminPhone := os.Getenv("ADMIN_PHONE")
	gcpProject := os.Getenv("GCP_PROJECT")
	zfsKeyName := os.Getenv("ZFS_KEY_NAME")
	if zfsKeyName == "" {
		zfsKeyName = "zfs-master-key"
	}

	// Create dependencies
	verifier := totp.NewVerifier(totpSeed)
	notifier := telnyx.NewClient(telnyxAPIKey, telnyxFromNumber)

	webhookVerifier, err := telnyx.NewWebhookVerifier(telnyxPublicKey)
	if err != nil {
		logger.Error("failed to create webhook verifier", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	secretStore, err := gcp.NewClient(ctx, gcpProject)
	if err != nil {
		logger.Error("failed to create secret store", "error", err)
		os.Exit(1)
	}
	defer secretStore.Close()

	// Create router
	cfg := server.Config{
		AgentSecret: agentSecret,
		AdminPhone:  adminPhone,
		ZFSKeyName:  zfsKeyName,
	}

	router := server.NewRouter(cfg, verifier, notifier, secretStore, webhookVerifier, logger)

	// Start server
	logger.Info("starting key-bringer", "port", port)
	if err := router.Run(":" + port); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
