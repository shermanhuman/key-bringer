package telnyx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const apiURL = "https://api.telnyx.com/v2/messages"

// Client implements core.Notifier using Telnyx SMS API.
type Client struct {
	apiKey     string
	fromNumber string
	httpClient *http.Client
}

// NewClient creates a new Telnyx SMS client.
func NewClient(apiKey, fromNumber string) *Client {
	return &Client{
		apiKey:     apiKey,
		fromNumber: fromNumber,
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

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(jsonBody))
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
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telnyx: API error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
