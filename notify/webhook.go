package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ardean/mdadm-notifier/config"
)

type Webhook struct {
	url    string
	client *http.Client
}

func NewWebhook(cfg config.WebhookConfig) (*Webhook, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("WEBHOOK_URL is missing")
	}

	return &Webhook{
		url: cfg.URL,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}, nil
}

func (w *Webhook) Name() string { return "webhook" }

func (w *Webhook) Send(ctx context.Context, message string) error {
	body, err := json.Marshal(map[string]string{"message": message})
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("post webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %s", resp.Status)
	}

	return nil
}

func (w *Webhook) Close() error { return nil }
