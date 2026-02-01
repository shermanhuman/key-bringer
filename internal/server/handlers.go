package server

import (
	"io"
	"log/slog"
	"net/http"
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
	Secret    string
}

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	verifier        core.Verifier
	notifier        core.Notifier
	secretStore     core.SecretStore
	webhookVerifier *telnyx.WebhookVerifier
	adminPhone      string
	zfsKeyName      string

	// In-memory session store (use Redis in production)
	sessions map[string]*Session
	mu       sync.RWMutex
	logger   *slog.Logger
}

// NewHandler creates a new handler with dependencies.
func NewHandler(
	verifier core.Verifier,
	notifier core.Notifier,
	secretStore core.SecretStore,
	webhookVerifier *telnyx.WebhookVerifier,
	adminPhone string,
	zfsKeyName string,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		verifier:        verifier,
		notifier:        notifier,
		secretStore:     secretStore,
		webhookVerifier: webhookVerifier,
		adminPhone:      adminPhone,
		zfsKeyName:      zfsKeyName,
		sessions:        make(map[string]*Session),
		logger:          logger,
	}
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

	// Send SMS challenge
	message := "Key-Bringer unlock request for " + req.MachineID + ". Reply with your 6-digit code."
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

	// Session approved - return secret and clean up
	h.mu.Lock()
	delete(h.sessions, req.SessionID)
	h.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{"secret": session.Secret})
}

// HandleWebhook handles POST /webhooks/telnyx.
func (h *Handler) HandleWebhook(c *gin.Context) {
	// Read body for signature verification
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	// Verify signature
	if err := h.webhookVerifier.Verify(c.Request, body); err != nil {
		h.logger.Warn("webhook signature verification failed", "error", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	// Parse the webhook payload to extract the SMS text
	// Telnyx webhook structure: {"data":{"payload":{"from":"+1...", "text":"123456"}}}
	var payload struct {
		Data struct {
			Payload struct {
				From string `json:"from"`
				Text string `json:"text"`
			} `json:"payload"`
		} `json:"data"`
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		// Re-parse from body since we already consumed it
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	code := payload.Data.Payload.Text
	from := payload.Data.Payload.From

	h.logger.Info("received SMS reply", "from", from, "code_length", len(code))

	// Validate TOTP
	if !h.verifier.Validate(code) {
		h.logger.Warn("invalid TOTP code received via SMS", "from", from)
		c.JSON(http.StatusOK, gin.H{"status": "invalid code"})
		return
	}

	// Find and approve pending sessions for this phone
	secret, err := h.secretStore.GetSecret(c.Request.Context(), h.zfsKeyName)
	if err != nil {
		h.logger.Error("failed to get secret for webhook approval", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve secret"})
		return
	}

	h.mu.Lock()
	for _, session := range h.sessions {
		if session.Phone == from && !session.Approved {
			session.Approved = true
			session.Secret = secret
			h.logger.Info("session approved", "machine_id", session.MachineID)
		}
	}
	h.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{"status": "approved"})
}
