package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

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
	telnyxMessagingProfileID := os.Getenv("TELNYX_MESSAGING_PROFILE_ID")
	adminPhone := os.Getenv("ADMIN_PHONE")
	gcpProject := os.Getenv("GCP_PROJECT")
	publicURL := os.Getenv("PUBLIC_URL")
	zfsKeyName := os.Getenv("ZFS_KEY_NAME")
	if zfsKeyName == "" {
		zfsKeyName = "zfs-master-key"
	}

	if telnyxMessagingProfileID == "" {
		logger.Error("TELNYX_MESSAGING_PROFILE_ID is required")
		os.Exit(1)
	}
	if publicURL == "" {
		logger.Error("PUBLIC_URL is required")
		os.Exit(1)
	}

	// Create dependencies
	verifier := totp.NewVerifier(totpSeed)
	telnyxClient := telnyx.NewClient(telnyxAPIKey, telnyxFromNumber, telnyxMessagingProfileID)

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
		PublicURL:   publicURL,
	}

	h := server.NewRouter(cfg, verifier, telnyxClient, telnyxClient, secretStore, webhookVerifier, logger)

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
