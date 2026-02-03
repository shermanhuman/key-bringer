package telnyx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	apiBaseURL      = "https://api.telnyx.com/v2"
	messagesAPIPath = "/messages"
)

// Client implements core.Notifier using Telnyx SMS API.
type Client struct {
	apiKey     string
	fromNumber string
	profileID  string
	httpClient *http.Client
}

// NewClient creates a new Telnyx SMS client.
func NewClient(apiKey, fromNumber, messagingProfileID string) *Client {
	return &Client{
		apiKey:     apiKey,
		fromNumber: fromNumber,
		profileID:  messagingProfileID,
		httpClient: &http.Client{},
	}
}

// messageRequest is the Telnyx API request body.
type messageRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
	Text string `json:"text"`
}

// SendSMS sends an SMS message to the specified phone number.
func (c *Client) SendSMS(ctx context.Context, to, message string) error {
	reqBody := messageRequest{
		From: c.fromNumber,
		To:   to,
		Text: message,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("telnyx: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiBaseURL+messagesAPIPath, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("telnyx: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telnyx: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		_ = resp.Body.Close()
		return fmt.Errorf("telnyx: API error status %d", resp.StatusCode)
	}

	return nil
}

// UpdateMessagingProfileWebhookURL updates the Telnyx messaging profile webhook_url.
//
// IMPORTANT: webhookURL may contain a secret token. Do not log or return it.
func (c *Client) UpdateMessagingProfileWebhookURL(ctx context.Context, webhookURL string) error {
	if c.profileID == "" {
		return fmt.Errorf("telnyx: messaging profile id is required")
	}

	// Telnyx API: PATCH /v2/messaging_profiles/{id}
	// Body: {"webhook_url":"https://..."}
	body := struct {
		WebhookURL string `json:"webhook_url"`
	}{WebhookURL: webhookURL}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("telnyx: failed to marshal webhook update: %w", err)
	}

	url := fmt.Sprintf("%s/messaging_profiles/%s", apiBaseURL, c.profileID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("telnyx: failed to create webhook update request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telnyx: webhook update request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// Avoid returning response body; it may include the webhook URL.
		_, _ = io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("telnyx: webhook update failed with status %d", resp.StatusCode)
	}

	return nil
}
