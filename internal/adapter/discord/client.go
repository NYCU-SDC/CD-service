package discord

import (
	"NYCU-SDC/deployment-service/internal/domain"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Client implements domain.Notifier interface
type Client struct {
	webhookURL string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewClient creates a new Discord client
func NewClient(webhookURL string, logger *zap.Logger) *Client {
	return &Client{
		webhookURL: webhookURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
	}
}

// Embed represents a Discord embed
type Embed struct {
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Color       int       `json:"color"`
	Fields      []Field   `json:"fields,omitempty"`
	Timestamp   string    `json:"timestamp,omitempty"`
}

// Field represents a Discord embed field
type Field struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// WebhookPayload represents the Discord webhook payload
type WebhookPayload struct {
	Embeds []Embed `json:"embeds"`
}

// SendNotification sends a notification to Discord
func (c *Client) SendNotification(ctx context.Context, title, message string, success bool, metadata map[string]string) error {
	color := 0x00FF00 // Green for success
	if !success {
		color = 0xFF0000 // Red for failure
	}

	fields := make([]Field, 0, len(metadata))
	for key, value := range metadata {
		fields = append(fields, Field{
			Name:   key,
			Value:  value,
			Inline: true,
		})
	}

	embed := Embed{
		Title:       title,
		Description: message,
		Color:       color,
		Fields:      fields,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}

	payload := WebhookPayload{
		Embeds: []Embed{embed},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Discord API returned status %d", resp.StatusCode)
	}

	c.logger.Info("Discord notification sent",
		zap.String("title", title),
		zap.Bool("success", success),
	)

	return nil
}

// Ensure Client implements domain.Notifier
var _ domain.Notifier = (*Client)(nil)
