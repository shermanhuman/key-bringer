package server

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Applesauce-Labs/key-bringer/internal/core"
	"github.com/Applesauce-Labs/key-bringer/internal/notifiers/telnyx"
)

type denyAllVerifier struct{}

func (denyAllVerifier) Validate(string) bool { return false }

type nopNotifier struct{}

func (nopNotifier) SendSMS(context.Context, string, string) error { return nil }

type nopUpdater struct{}

func (nopUpdater) UpdateMessagingProfileWebhookURL(context.Context, string) error { return nil }

type constStore struct{ v string }

func (c constStore) GetSecret(context.Context, core.SecretRef) (string, error) { return c.v, nil }

func TestHandleUnlockRejectsUnknownMachine(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	h := NewHandler(
		denyAllVerifier{},
		nopNotifier{},
		nopUpdater{},
		constStore{v: "SECRET"},
		&telnyx.WebhookVerifier{},
		"+15555550123",
		core.SecretRef{SecretID: "zfs-master-key", Version: 1},
		"",
		10,
		[]string{"machine-1"},
		logger,
	)

	body := []byte(`{"machine_id":"machine-2"}`)
	req := httptest.NewRequest(http.MethodPost, "/unlock", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleUnlock(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}
