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
	zfsMasterKey    core.SecretRef
	publicURL       string
	maxPending      time.Duration
	allowedMachines map[string]struct{}

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
	zfsMasterKey core.SecretRef,
	publicURL string,
	maxPendingMinutes int,
	allowedMachines []string,
	logger *slog.Logger,
) *Handler {
	allow := make(map[string]struct{}, len(allowedMachines))
	for _, id := range allowedMachines {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		allow[id] = struct{}{}
	}
	maxPending := time.Duration(maxPendingMinutes) * time.Minute
	if maxPending <= 0 {
		maxPending = 10 * time.Minute
	}

	return &Handler{
		verifier:        verifier,
		notifier:        notifier,
		webhookUpdater:  webhookUpdater,
		secretStore:     secretStore,
		webhookVerifier: webhookVerifier,
		adminPhone:      adminPhone,
		zfsMasterKey:    zfsMasterKey,
		publicURL:       strings.TrimRight(publicURL, "/"),
		maxPending:      maxPending,
		allowedMachines: allow,
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

	// Update provider messaging profile webhook URL.
	if err := h.webhookUpdater.UpdateMessagingProfileWebhookURL(ctx, webhookURL); err != nil {
		// Revert staged token state.
		h.mu.Lock()
		h.webhookTokenCurrent = oldCurrent
		h.webhookTokenPrevious = oldPrevious
		h.webhookTokenPreviousValidTil = oldPrevTil
		h.mu.Unlock()
		// Do not return the underlying error; it may include the URL/token.
		return "", fmt.Errorf("provider webhook update failed")
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

// UnlockRequest is the request body for POST /unlock.
type UnlockRequest struct {
	MachineID string `json:"machine_id"`
	TOTPCode  string `json:"totp_code"`
}

// HandleUnlock handles POST /unlock.

func (h *Handler) HandleUnlock(w http.ResponseWriter, r *http.Request) {
	var req UnlockRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if strings.TrimSpace(req.MachineID) == "" {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "machine_id is required"})
		return
	}
	if _, ok := h.allowedMachines[req.MachineID]; !ok {
		// Do not reveal allow-list membership.
		WriteJSON(w, http.StatusForbidden, map[string]string{"error": "unauthorized"})
		return
	}

	// If TOTP provided, verify immediately
	if req.TOTPCode != "" {
		if !h.verifier.Validate(req.TOTPCode) {
			WriteJSON(w, http.StatusForbidden, map[string]string{"error": "invalid TOTP code"})
			return
		}

		// Get and return the secret
		secret, err := h.secretStore.GetSecret(r.Context(), h.zfsMasterKey)
		if err != nil {
			h.logger.Error("failed to get secret", "error", err)
			WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to retrieve secret"})
			return
		}

		WriteJSON(w, http.StatusOK, map[string]string{"secret": secret})
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
	if _, err := h.rotateWebhookToken(r.Context()); err != nil {
		h.logger.Error("failed to rotate webhook token", "error", err)
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to prepare webhook"})
		return
	}

	// Send SMS challenge
	message := "Key-Bringer unlock request for " + req.MachineID + ". Reply with: APPROVE " + req.MachineID + " <6-digit-code>"
	if err := h.notifier.SendSMS(r.Context(), h.adminPhone, message); err != nil {
		h.logger.Error("failed to send SMS", "error", err)
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to send SMS"})
		return
	}

	h.logger.Info("unlock request created", "machine_id", req.MachineID, "session_id", sessionID)
	WriteJSON(w, http.StatusAccepted, map[string]string{"session_id": sessionID})
}

// PollRequest is the request body for POST /api/v1/poll.
type PollRequest struct {
	MachineID string
	SessionID string
}

// HandlePoll handles POST /api/v1/poll.

func (h *Handler) HandlePoll(w http.ResponseWriter, r *http.Request) {
	req := PollRequest{
		MachineID: r.URL.Query().Get("machine_id"),
		SessionID: r.URL.Query().Get("session_id"),
	}
	if strings.TrimSpace(req.SessionID) == "" {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "session_id is required"})
		return
	}
	if strings.TrimSpace(req.MachineID) == "" {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "machine_id is required"})
		return
	}

	h.mu.RLock()
	session, exists := h.sessions[req.SessionID]
	h.mu.RUnlock()

	if !exists {
		WriteJSON(w, http.StatusGone, map[string]string{"error": "session expired or not found"})
		return
	}

	// Check if session expired (10 minute timeout)
	if time.Since(session.CreatedAt) > h.maxPending {
		h.mu.Lock()
		delete(h.sessions, req.SessionID)
		h.mu.Unlock()
		WriteJSON(w, http.StatusGone, map[string]string{"error": "session expired"})
		return
	}

	if !session.Approved {
		WriteJSON(w, http.StatusAccepted, map[string]string{"status": "pending"})
		return
	}

	// Session approved - fetch secret at deliver time and clean up
	secret, err := h.secretStore.GetSecret(r.Context(), h.zfsMasterKey)
	if err != nil {
		h.logger.Error("failed to get secret for approved session", "error", err)
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to retrieve secret"})
		return
	}

	h.mu.Lock()
	delete(h.sessions, req.SessionID)
	h.mu.Unlock()

	WriteJSON(w, http.StatusOK, map[string]string{"secret": secret})
}

// HandleWebhookProbe responds quickly for reachability checks.
// Returns 204 only if token is valid; otherwise 404.

func (h *Handler) HandleWebhookProbe(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if !h.isValidWebhookToken(token, time.Now()) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleWebhookTokenized handles POST /webhooks/telnyx/:token.

func (h *Handler) HandleWebhookTokenized(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if !h.isValidWebhookToken(token, time.Now()) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	h.handleWebhookCommon(w, r)
}

// HandleWebhookLegacy handles POST /webhooks/telnyx.
// Once token rotation has started, this endpoint is disabled to reduce attack surface.

func (h *Handler) HandleWebhookLegacy(w http.ResponseWriter, r *http.Request) {
	if h.hasActiveWebhookToken() {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	h.handleWebhookCommon(w, r)
}


func (h *Handler) handleWebhookCommon(w http.ResponseWriter, r *http.Request) {
	// Read raw body for signature verification.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}

	if err := h.webhookVerifier.Verify(r, body); err != nil {
		h.logger.Warn("webhook signature verification failed", "error", err)
		WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
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
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
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
			WriteJSON(w, http.StatusOK, map[string]string{"status": "duplicate"})
			return
		}
		h.seenEventIDs[eventID] = now
		h.mu.Unlock()
	}

	if from != h.adminPhone {
		// Acknowledge quickly; do not leak details.
		h.logger.Warn("webhook from non-admin phone", "from", from)
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}

	fields := strings.Fields(text)
	if len(fields) < 3 || !strings.EqualFold(fields[0], "APPROVE") {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}

	machineID := fields[1]
	code := fields[2]

	h.logger.Info("received SMS approval", "from", from, "machine_id", machineID, "code_length", len(code))

	if !h.verifier.Validate(code) {
		h.logger.Warn("invalid TOTP code received via SMS", "from", from, "machine_id", machineID)
		WriteJSON(w, http.StatusOK, map[string]string{"status": "invalid code"})
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

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
