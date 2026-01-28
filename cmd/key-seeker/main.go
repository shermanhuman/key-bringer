package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env if present
	_ = godotenv.Load()

	// Parse flags
	var (
		totpCode    string
		monitor     bool
		serverURL   string
		machineID   string
		agentSecret string
		zfsDataset  string
	)

	flag.StringVar(&totpCode, "totp", "", "TOTP code for immediate unlock")
	flag.BoolVar(&monitor, "monitor", false, "Monitor mode: poll until approved")
	flag.StringVar(&serverURL, "server", os.Getenv("SERVER_URL"), "key-bringer server URL")
	flag.StringVar(&machineID, "machine", os.Getenv("MACHINE_ID"), "Machine identifier")
	flag.StringVar(&agentSecret, "secret", os.Getenv("AGENT_SECRET"), "Agent secret")
	flag.StringVar(&zfsDataset, "dataset", os.Getenv("ZFS_DATASET"), "ZFS dataset to unlock")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Validate required args
	if serverURL == "" {
		logger.Error("server URL required (--server or SERVER_URL)")
		os.Exit(1)
	}
	if machineID == "" {
		logger.Error("machine ID required (--machine or MACHINE_ID)")
		os.Exit(1)
	}
	if agentSecret == "" {
		logger.Error("agent secret required (--secret or AGENT_SECRET)")
		os.Exit(1)
	}

	client := &Client{
		serverURL:   serverURL,
		agentSecret: agentSecret,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		logger:      logger,
	}

	ctx := context.Background()

	// Mode 1: Immediate unlock with TOTP
	if totpCode != "" {
		secret, err := client.UnlockWithTOTP(ctx, machineID, totpCode)
		if err != nil {
			logger.Error("unlock failed", "error", err)
			os.Exit(1)
		}
		if err := applyKey(zfsDataset, secret); err != nil {
			logger.Error("failed to apply key", "error", err)
			os.Exit(1)
		}
		logger.Info("ZFS unlocked successfully", "dataset", zfsDataset)
		return
	}

	// Mode 2: Monitor/poll mode
	if monitor {
		secret, err := client.UnlockAndPoll(ctx, machineID)
		if err != nil {
			logger.Error("unlock failed", "error", err)
			os.Exit(1)
		}
		if err := applyKey(zfsDataset, secret); err != nil {
			logger.Error("failed to apply key", "error", err)
			os.Exit(1)
		}
		logger.Info("ZFS unlocked successfully", "dataset", zfsDataset)
		return
	}

	// No mode specified
	fmt.Println("Usage: key-seeker [options]")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  --totp <code>    Unlock immediately with TOTP code")
	fmt.Println("  --monitor        Request unlock and poll until approved")
	fmt.Println("  --server <url>   key-bringer server URL")
	fmt.Println("  --machine <id>   Machine identifier")
	fmt.Println("  --secret <s>     Agent secret")
	fmt.Println("  --dataset <ds>   ZFS dataset to unlock")
	os.Exit(1)
}

// Client communicates with the key-bringer server.
type Client struct {
	serverURL   string
	agentSecret string
	httpClient  *http.Client
	logger      *slog.Logger
}

// UnlockWithTOTP requests immediate unlock with a TOTP code.
func (c *Client) UnlockWithTOTP(ctx context.Context, machineID, totpCode string) (string, error) {
	body := map[string]string{
		"machine_id": machineID,
		"totp_code":  totpCode,
	}

	resp, err := c.post(ctx, "/api/v1/unlock", body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("invalid TOTP code")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var result struct {
		Secret string `json:"secret"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Secret, nil
}

// UnlockAndPoll initiates unlock and polls until approved.
func (c *Client) UnlockAndPoll(ctx context.Context, machineID string) (string, error) {
	// Initiate unlock
	body := map[string]string{"machine_id": machineID}
	resp, err := c.post(ctx, "/api/v1/unlock", body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var initResp struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&initResp); err != nil {
		return "", fmt.Errorf("failed to decode init response: %w", err)
	}

	c.logger.Info("SMS sent, waiting for approval...", "session_id", initResp.SessionID)

	// Poll every 5 seconds for up to 10 minutes
	pollBody := map[string]string{
		"machine_id": machineID,
		"session_id": initResp.SessionID,
	}

	timeout := time.After(10 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for approval")
		case <-ticker.C:
			resp, err := c.post(ctx, "/api/v1/poll", pollBody)
			if err != nil {
				c.logger.Warn("poll failed", "error", err)
				continue
			}

			if resp.StatusCode == http.StatusOK {
				var result struct {
					Secret string `json:"secret"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					return "", fmt.Errorf("failed to decode poll response: %w", err)
				}
				return result.Secret, nil
			}

			if resp.StatusCode == http.StatusGone {
				return "", fmt.Errorf("session expired")
			}

			// 202 Accepted - still pending
			c.logger.Info("still pending...")
		}
	}
}

func (c *Client) post(ctx context.Context, path string, body map[string]string) (*http.Response, error) {
	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", c.serverURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Secret", c.agentSecret)

	return c.httpClient.Do(req)
}

// applyKey loads the ZFS encryption key.
func applyKey(dataset, secret string) error {
	if dataset == "" {
		// No dataset specified - just log (for testing)
		fmt.Printf("Key received: %s\n", secret)
		return nil
	}

	// Run: echo "$secret" | zfs load-key $dataset
	cmd := exec.Command("zfs", "load-key", dataset)
	cmd.Stdin = bytes.NewBufferString(secret + "\n")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("zfs load-key failed: %w", err)
	}

	// Mount the dataset
	mountCmd := exec.Command("zfs", "mount", dataset)
	mountCmd.Stdout = os.Stdout
	mountCmd.Stderr = os.Stderr
	_ = mountCmd.Run() // Ignore mount errors (may already be mounted)

	return nil
}
