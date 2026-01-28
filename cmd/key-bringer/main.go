package main

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if present (ignored in production)
	_ = godotenv.Load()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	logger.Info("key-bringer starting", "port", os.Getenv("PORT"))

	// TODO: Wire up dependencies and start server
	logger.Info("key-bringer server not yet implemented")
}
