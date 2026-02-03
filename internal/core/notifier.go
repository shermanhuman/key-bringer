package core

import "context"

// Notifier handles sending SMS challenges to administrators.
type Notifier interface {
	// SendSMS sends an SMS message to the specified phone number.
	SendSMS(ctx context.Context, to, message string) error
}

// WebhookURLUpdater updates the inbound webhook URL for the SMS provider.
//
// This is used for per-unlock-session rotation of an unguessable webhook path token.
// Implementations must avoid logging or returning the full webhook URL if it includes a secret token.
type WebhookURLUpdater interface {
	UpdateMessagingProfileWebhookURL(ctx context.Context, webhookURL string) error
}
