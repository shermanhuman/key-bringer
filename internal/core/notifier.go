package core

import "context"

// Notifier handles sending SMS challenges to administrators.
type Notifier interface {
	// SendSMS sends an SMS message to the specified phone number.
	SendSMS(ctx context.Context, to, message string) error
}
