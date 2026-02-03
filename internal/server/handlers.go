package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Applesauce-Labs/key-bringer/internal/core"
	"github.com/Applesauce-Labs/key-bringer/internal/notifiers/telnyx"
	"github.com/gin-gonic/gin"
)

// Session represents a pending unlock request.
type Session struct {
	MachineID string
	Phone     string
	CreatedAt time.Time
	Approved  bool
	ApprovedAt time.Time
}

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	verifier        core.Verifier
	notifier        core.Notifier
	webhookUpdater  core.WebhookURLUpdater
	secretStore     core.SecretStore
	webhookVerifier *telnyx.WebhookVerifier
	adminPhone      string
	zfsKeyName      string
	publicURL       string

	webhookTokenCurrent          string
	webhookTokenPrevious         string
	webhookTokenPreviousValidTil time.Time

	// Deduplication for Telnyx webhook events (best-effort, in-memory).
	seenEventIDs map[string]time.Time

	// In-memory session store (use Redis in production)
	sessions map[string]*Session
	mu       sync.RWMutex
	logger   *slog.Logger
}

// NewHandler creates a new handler with dependencies.
func NewHandler(
	verifier core.Verifier,
	notifier core.Notifier,
	webhookUpdater core.WebhookURLUpdater,
	secretStore core.SecretStore,
	webhookVerifier *telnyx.WebhookVerifier,
	adminPhone string,
	zfsKeyName string,
	publicURL string,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		verifier:        verifier,
		notifier:        notifier,
		webhookUpdater:  webhookUpdater,
		secretStore:     secretStore,
		webhookVerifier: webhookVerifier,
		adminPhone:      adminPhone,
		zfsKeyName:      zfsKeyName,
		publicURL:       strings.TrimRight(publicURL, "/"),
		sessions:        make(map[string]*Session),
		seenEventIDs:    make(map[string]time.Time),
		logger:          logger,
	}
}

func generateWebhookToken() (string, error) {
	// 32 random bytes => 43-char base64url (no padding).
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (h *Handler) isValidWebhookToken(token string, now time.Time) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if token == "" {
		return false
	}
	if token == h.webhookTokenCurrent {
		return true
	}
	if token == h.webhookTokenPrevious && !h.webhookTokenPreviousValidTil.IsZero() && now.Before(h.webhookTokenPreviousValidTil) {
		return true
	}
	return false
}

func (h *Handler) hasActiveWebhookToken() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.webhookTokenCurrent != ""
}


func (h *Handler) rotateWebhookToken(ctx context.Context) (string, error) {
	if h.webhookUpdater == nil {
		return "", fmt.Errorf("webhook rotation is not configured")
	}
	if h.publicURL == "" {
		return "", fmt.Errorf("public URL is required for webhook rotation")
	}

	newToken, err := generateWebhookToken()
	if err != nil {
		return "", err
	}

	now := time.Now()
	oldCurrent := ""
	oldPrevious := ""
	oldPrevTil := time.Time{}

	// Stage the new token so the probe can succeed, but revert if update/probe fails.
	h.mu.Lock()
	oldCurrent = h.webhookTokenCurrent
	oldPrevious = h.webhookTokenPrevious
	oldPrevTil = h.webhookTokenPreviousValidTil
	h.webhookTokenPrevious = oldCurrent
	h.webhookTokenPreviousValidTil = now.Add(60 * time.Second)
	h.webhookTokenCurrent = newToken
	h.mu.Unlock()

	webhookURL := h.publicURL + "/webhooks/telnyx/" + newToken

	// Update Telnyx messaging profile webhook URL.
	if err := h.webhookUpdater.UpdateMessagingProfileWebhookURL(ctx, webhookURL); err != nil {
		// Revert staged token state.
		h.mu.Lock()
		h.webhookTokenCurrent = oldCurrent
		h.webhookTokenPrevious = oldPrevious
		h.webhookTokenPreviousValidTil = oldPrevTil
		h.mu.Unlock()
		return "", err
	}

	// Probe the new endpoint is reachable.
	probeOK := false
	deadline := time.Now().Add(20 * time.Second)
	httpClient := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, webhookURL, nil)
		if err != nil {
			break
		}
		resp, err := httpClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusNoContent {
				probeOK = true
				break
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !probeOK {
		// Revert staged token state.
		h.mu.Lock()
		h.webhookTokenCurrent = oldCurrent
		h.webhookTokenPrevious = oldPrevious
		h.webhookTokenPreviousValidTil = oldPrevTil
		h.mu.Unlock()
		return "", fmt.Errorf("webhook probe failed")
	}

	return newToken, nil
}

// UnlockRequest is the request body for POST /api/v1/unlock.
type UnlockRequest struct {
	MachineID string `json:"machine_id" binding:"required"`
	TOTPCode  string `json:"totp_code"`
}

// HandleUnlock handles POST /api/v1/unlock.
func (h *Handler) HandleUnlock(c *gin.Context) {
	var req UnlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// If TOTP provided, verify immediately
	if req.TOTPCode != "" {
		if !h.verifier.Validate(req.TOTPCode) {
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid TOTP code"})
			return
		}

		// Get and return the secret
		secret, err := h.secretStore.GetSecret(c.Request.Context(), h.zfsKeyName)
		if err != nil {
			h.logger.Error("failed to get secret", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve secret"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"secret": secret})
		return
	}

	// No TOTP - create session and send SMS
	sessionID := req.MachineID + "-" + time.Now().Format("20060102150405")

	h.mu.Lock()
	h.sessions[sessionID] = &Session{
		MachineID: req.MachineID,
		Phone:     h.adminPhone,
		CreatedAt: time.Now(),
		Approved:  false,
	}
	h.mu.Unlock()

	// Rotate webhook token BEFORE sending the SMS challenge (fail closed).
	if _, err := h.rotateWebhookToken(c.Request.Context()); err != nil {
		h.logger.Error("failed to rotate webhook token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare webhook"})
		return
	}

	// Send SMS challenge
	message := "Key-Bringer unlock request for " + req.MachineID + ". Reply with: APPROVE " + req.MachineID + " <6-digit-code>"
	if err := h.notifier.SendSMS(c.Request.Context(), h.adminPhone, message); err != nil {
		h.logger.Error("failed to send SMS", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send SMS"})
		return
	}

	h.logger.Info("unlock request created", "machine_id", req.MachineID, "session_id", sessionID)
	c.JSON(http.StatusAccepted, gin.H{"session_id": sessionID})
}

// PollRequest is the request body for POST /api/v1/poll.
type PollRequest struct {
	MachineID string `json:"machine_id" binding:"required"`
	SessionID string `json:"session_id" binding:"required"`
}

// HandlePoll handles POST /api/v1/poll.
func (h *Handler) HandlePoll(c *gin.Context) {
	var req PollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.mu.RLock()
	session, exists := h.sessions[req.SessionID]
	h.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusGone, gin.H{"error": "session expired or not found"})
		return
	}

	// Check if session expired (10 minute timeout)
	if time.Since(session.CreatedAt) > 10*time.Minute {
		h.mu.Lock()
		delete(h.sessions, req.SessionID)
		h.mu.Unlock()
		c.JSON(http.StatusGone, gin.H{"error": "session expired"})
		return
	}

	if !session.Approved {
		c.JSON(http.StatusAccepted, gin.H{"status": "pending"})
		return
	}

	// Session approved - fetch secret at deliver time and clean up
	secret, err := h.secretStore.GetSecret(c.Request.Context(), h.zfsKeyName)
	if err != nil {
		h.logger.Error("failed to get secret for approved session", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve secret"})
		return
	}

	h.mu.Lock()
	delete(h.sessions, req.SessionID)
	h.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{"secret": secret})
}

// HandleWebhookProbe responds quickly for reachability checks.
// Returns 204 only if token is valid; otherwise 404.
func (h *Handler) HandleWebhookProbe(c *gin.Context) {
	token := c.Param("token")
	if !h.isValidWebhookToken(token, time.Now()) {
		c.Status(http.StatusNotFound)
		return
	}
	c.Status(http.StatusNoContent)
}

// HandleWebhookTokenized handles POST /webhooks/telnyx/:token.
func (h *Handler) HandleWebhookTokenized(c *gin.Context) {
	token := c.Param("token")
	if !h.isValidWebhookToken(token, time.Now()) {
		c.Status(http.StatusNotFound)
		return
	}
	h.handleWebhookCommon(c)
}

// HandleWebhookLegacy handles POST /webhooks/telnyx.
// Once token rotation has started, this endpoint is disabled to reduce attack surface.
func (h *Handler) HandleWebhookLegacy(c *gin.Context) {
	if h.hasActiveWebhookToken() {
		c.Status(http.StatusNotFound)
		return
	}
	h.handleWebhookCommon(c)
}

func (h *Handler) handleWebhookCommon(c *gin.Context) {
	// Read raw body for signature verification.
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	if err := h.webhookVerifier.Verify(c.Request, body); err != nil {
		h.logger.Warn("webhook signature verification failed", "error", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	// Telnyx webhook structure (simplified for v1):
	// {"data":{"id":"...","payload":{"from":"+1...","text":"APPROVE <machineId> <totp>"}}}
	var payload struct {
		Data struct {
			ID      string `json:"id"`
			Payload struct {
				From string `json:"from"`
				Text string `json:"text"`
			} `json:"payload"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	eventID := payload.Data.ID
	from := payload.Data.Payload.From
	text := payload.Data.Payload.Text

	if eventID != "" {
		now := time.Now()
		h.mu.Lock()
		// Best-effort pruning.
		for id, seenAt := range h.seenEventIDs {
			if now.Sub(seenAt) > 30*time.Minute {
				delete(h.seenEventIDs, id)
			}
		}
		if _, ok := h.seenEventIDs[eventID]; ok {
			h.mu.Unlock()
			c.JSON(http.StatusOK, gin.H{"status": "duplicate"})
			return
		}
		h.seenEventIDs[eventID] = now
		h.mu.Unlock()
	}

	if from != h.adminPhone {
		// Acknowledge quickly; do not leak details.
		h.logger.Warn("webhook from non-admin phone", "from", from)
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	fields := strings.Fields(text)
	if len(fields) < 3 || !strings.EqualFold(fields[0], "APPROVE") {
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	machineID := fields[1]
	code := fields[2]

	h.logger.Info("received SMS approval", "from", from, "machine_id", machineID, "code_length", len(code))

	if !h.verifier.Validate(code) {
		h.logger.Warn("invalid TOTP code received via SMS", "from", from, "machine_id", machineID)
		c.JSON(http.StatusOK, gin.H{"status": "invalid code"})
		return
	}

	// Approve the latest pending session for this machine.
	var latestSessionID string
	var latestCreated time.Time

	h.mu.Lock()
	for id, session := range h.sessions {
		if session.MachineID != machineID {
			continue
		}
		if session.Phone != from || session.Approved {
			continue
		}
		if time.Since(session.CreatedAt) > 10*time.Minute {
			delete(h.sessions, id)
			continue
		}
		if latestSessionID == "" || session.CreatedAt.After(latestCreated) {
			latestSessionID = id
			latestCreated = session.CreatedAt
		}
	}
	if latestSessionID != "" {
		h.sessions[latestSessionID].Approved = true
		h.sessions[latestSessionID].ApprovedAt = time.Now()
		h.logger.Info("session approved", "machine_id", machineID, "session_id", latestSessionID)
	}
	h.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
