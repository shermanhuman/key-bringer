package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Applesauce-Labs/key-bringer/internal/core"
	"github.com/Applesauce-Labs/key-bringer/internal/unlockers/zfs"
	"github.com/joho/godotenv"
	"google.golang.org/api/idtoken"
)

func main() {
	// Load .env if present
	_ = godotenv.Load()

	// Parse flags
	var (
		totpCode     string
		monitor      bool
		serverURL    string
		machineID    string
		agentSecret  string
		target       string
		unlockerType string
	)

	flag.StringVar(&totpCode, "totp", "", "TOTP code for immediate unlock")
	flag.BoolVar(&monitor, "monitor", false, "Monitor mode: poll until approved")
	flag.StringVar(&serverURL, "server", os.Getenv("SERVER_URL"), "key-bringer server URL")
	flag.StringVar(&machineID, "machine", os.Getenv("MACHINE_ID"), "Machine identifier")
	flag.StringVar(&agentSecret, "secret", os.Getenv("AGENT_SECRET"), "Agent secret")
	flag.StringVar(&target, "target", os.Getenv("UNLOCK_TARGET"), "Target to unlock (e.g., ZFS dataset, BitLocker volume)")
	flag.StringVar(&unlockerType, "unlocker", os.Getenv("UNLOCKER_TYPE"), "Unlocker type: zfs, bitlocker, filevault (default: zfs)")
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

	ctx := context.Background()

	// Create HTTP client with Google Cloud authentication
	httpClient, err := idtoken.NewClient(ctx, serverURL)
	if err != nil {
		logger.Warn("failed to create authenticated client, using unauthenticated", "error", err)
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	client := &Client{
		serverURL:   serverURL,
		agentSecret: agentSecret,
		httpClient:  httpClient,
		logger:      logger,
	}

	// Create unlocker based on type
	unlocker, err := createUnlocker(unlockerType)
	if err != nil {
		logger.Error("failed to create unlocker", "error", err)
		os.Exit(1)
	}

	// Mode 1: Immediate unlock with TOTP
	if totpCode != "" {
		secret, err := client.UnlockWithTOTP(ctx, machineID, totpCode)
		if err != nil {
			logger.Error("unlock failed", "error", err)
			os.Exit(1)
		}
		if err := unlocker.ApplyKey(target, secret); err != nil {
			logger.Error("failed to apply key", "error", err)
			os.Exit(1)
		}
		logger.Info("unlocked successfully", "target", target, "unlocker", unlockerType)
		return
	}

	// Mode 2: Monitor/poll mode
	if monitor {
		secret, err := client.UnlockAndPoll(ctx, machineID)
		if err != nil {
			logger.Error("unlock failed", "error", err)
			os.Exit(1)
		}
		if err := unlocker.ApplyKey(target, secret); err != nil {
			logger.Error("failed to apply key", "error", err)
			os.Exit(1)
		}
		logger.Info("unlocked successfully", "target", target, "unlocker", unlockerType)
		return
	}

	// No mode specified
	fmt.Println("Usage: key-seeker [options]")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  --totp <code>      Unlock immediately with TOTP code")
	fmt.Println("  --monitor          Request unlock and poll until approved")
	fmt.Println("  --server <url>     key-bringer server URL")
	fmt.Println("  --machine <id>     Machine identifier")
	fmt.Println("  --secret <s>       Agent secret")
	fmt.Println("  --target <t>       Target to unlock (e.g., ZFS dataset, volume)")
	fmt.Println("  --unlocker <type>  Unlocker type: zfs (default), bitlocker, filevault")
	os.Exit(1)
}

// createUnlocker returns the appropriate unlocker based on type.
func createUnlocker(unlockerType string) (core.Unlocker, error) {
	switch strings.ToLower(unlockerType) {
	case "", "zfs":
		return zfs.NewUnlocker(), nil
	case "bitlocker":
		return nil, fmt.Errorf("bitlocker unlocker not yet implemented")
	case "filevault":
		return nil, fmt.Errorf("filevault unlocker not yet implemented")
	default:
		return nil, fmt.Errorf("unknown unlocker type: %s", unlockerType)
	}
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
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("invalid TOTP code (server: %s)", string(respBody))
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("authentication failed - check AGENT_SECRET (server: %s)", string(respBody))
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
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
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusAccepted {
		// Parse error message from server
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != "" {
			return "", fmt.Errorf("server error: %s", errResp.Error)
		}
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var initResp struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(respBody, &initResp); err != nil {
		return "", fmt.Errorf("failed to decode init response: %w", err)
	}

	c.logger.Info("SMS sent, waiting for TOTP response...", "session_id", initResp.SessionID)

	// Poll every 5 seconds for up to 10 minutes
	pollBody := map[string]string{
		"machine_id": machineID,
		"session_id": initResp.SessionID,
	}

	const pollTimeout = 10 * time.Minute
	startTime := time.Now()
	timeout := time.After(pollTimeout)
	pollTicker := time.NewTicker(5 * time.Second)
	defer pollTicker.Stop()

	// Setup spinner with its own animation ticker (100ms for smooth animation)
	spinner := newSpinner("Waiting for TOTP response via SMS", pollTimeout)
	spinnerDone := make(chan struct{})
	go func() {
		spinnerTicker := time.NewTicker(100 * time.Millisecond)
		defer spinnerTicker.Stop()
		spinner.Start()
		for {
			select {
			case <-spinnerDone:
				return
			case <-spinnerTicker.C:
				spinner.Tick()
			}
		}
	}()
	defer func() {
		close(spinnerDone)
		spinner.Stop()
	}()

	for {
		select {
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for approval")
		case <-pollTicker.C:
			// Update elapsed time on spinner
			spinner.SetElapsed(time.Since(startTime))

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

			// 202 Accepted - still pending, spinner continues independently
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

// spinner displays a Heroku-style Braille pattern animation with timer.
type spinner struct {
	message string
	frame   int
	frames  []rune
	isTTY   bool
	elapsed time.Duration
	timeout time.Duration
}

// Braille spinner frames (Heroku CLI style)
var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

// newSpinner creates a spinner for the given message.
func newSpinner(message string, timeout time.Duration) *spinner {
	isTTY := isTTYSupported()
	return &spinner{
		message: message,
		frame:   0,
		frames:  spinnerFrames,
		isTTY:   isTTY,
		elapsed: 0,
		timeout: timeout,
	}
}

// isTTYSupported checks if the terminal supports ANSI escape codes.
func isTTYSupported() bool {
	// Check if stdout is a terminal
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	if (fi.Mode() & os.ModeCharDevice) == 0 {
		return false // Not a TTY (e.g., piped)
	}
	// Check TERM environment variable (on Windows, TERM may be empty but TTY still works)
	term := os.Getenv("TERM")
	return term != "dumb"
}

// formatDuration formats duration as M:SS
func formatDuration(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

// Start displays the initial spinner frame.
func (s *spinner) Start() {
	if s.isTTY {
		s.render()
	}
}

// render draws the current spinner state
func (s *spinner) render() {
	timer := fmt.Sprintf(" (%s / %s)", formatDuration(s.elapsed), formatDuration(s.timeout))
	fmt.Printf("\r%c %s%s", s.frames[s.frame], s.message, timer)
}

// Tick advances the spinner to the next frame.
func (s *spinner) Tick() {
	if !s.isTTY {
		return
	}
	s.frame = (s.frame + 1) % len(s.frames)
	s.render()
}

// SetElapsed updates the elapsed time display.
func (s *spinner) SetElapsed(elapsed time.Duration) {
	s.elapsed = elapsed
}

// Stop clears the spinner line.
func (s *spinner) Stop() {
	if s.isTTY {
		// Clear the line (message + timer + padding)
		clearLen := len(s.message) + 20
		fmt.Printf("\r%s\r", strings.Repeat(" ", clearLen))
	}
}
