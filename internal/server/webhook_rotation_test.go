package server

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Applesauce-Labs/key-bringer/internal/core"
	"github.com/Applesauce-Labs/key-bringer/internal/notifiers/telnyx"
)

type fakeVerifier struct{}

func (fakeVerifier) Validate(string) bool { return true }

type fakeNotifier struct{}

func (fakeNotifier) SendSMS(context.Context, string, string) error { return nil }

type fakeSecretStore struct{}

func (fakeSecretStore) GetSecret(context.Context, core.SecretRef) (string, error) { return "FAKE_SECRET", nil }

type fakeWebhookUpdater struct {
	calls   int
	lastURL string
	err     error
}

func (f *fakeWebhookUpdater) UpdateMessagingProfileWebhookURL(ctx context.Context, webhookURL string) error {
	f.calls++
	f.lastURL = webhookURL
	return f.err
}

func TestWebhookTokenRotationOverlap(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	updater := &fakeWebhookUpdater{}

	// Build handler with a placeholder public URL; we'll set it to the httptest server URL after creation.
	h := NewHandler(
		fakeVerifier{},
		fakeNotifier{},
		updater,
		fakeSecretStore{},
		&telnyx.WebhookVerifier{},
		"+15555550123",
		core.SecretRef{SecretID: "zfs-master-key", Version: 1},
		"http://invalid",
		10,
		[]string{"ny1"},
		logger,
	)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /webhooks/telnyx/{token}", h.HandleWebhookProbe)
	mux.HandleFunc("POST /webhooks/telnyx", h.HandleWebhookLegacy)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	h.publicURL = srv.URL

	// 1st rotation
	first, err := h.rotateWebhookToken(context.Background())
	if err != nil {
		t.Fatalf("rotate 1 failed: %v", err)
	}
	if first == "" {
		t.Fatalf("expected first token")
	}
	if updater.calls != 1 {
		t.Fatalf("expected 1 updater call, got %d", updater.calls)
	}

	// 2nd rotation => first becomes previous (valid within grace)
	second, err := h.rotateWebhookToken(context.Background())
	if err != nil {
		t.Fatalf("rotate 2 failed: %v", err)
	}
	if second == "" || second == first {
		t.Fatalf("expected distinct second token")
	}

	// previous token should still be valid initially
	wPrev := httptest.NewRecorder()
	reqPrev := httptest.NewRequest(http.MethodGet, "/webhooks/telnyx/"+first, nil)
	mux.ServeHTTP(wPrev, reqPrev)
	if wPrev.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for previous token in grace, got %d", wPrev.Code)
	}

	// expire previous token
	h.mu.Lock()
	h.webhookTokenPreviousValidTil = time.Now().Add(-1 * time.Second)
	h.mu.Unlock()

	wPrevExpired := httptest.NewRecorder()
	reqPrevExpired := httptest.NewRequest(http.MethodGet, "/webhooks/telnyx/"+first, nil)
	mux.ServeHTTP(wPrevExpired, reqPrevExpired)
	if wPrevExpired.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for expired previous token, got %d", wPrevExpired.Code)
	}

	// legacy endpoint should be disabled once token rotation is active
	wLegacy := httptest.NewRecorder()
	reqLegacy := httptest.NewRequest(http.MethodPost, "/webhooks/telnyx", nil)
	mux.ServeHTTP(wLegacy, reqLegacy)
	if wLegacy.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for legacy webhook once token active, got %d", wLegacy.Code)
	}
}

var _ core.WebhookURLUpdater = (*fakeWebhookUpdater)(nil)
