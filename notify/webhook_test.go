package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ardean/mdadm-notifier/config"
)

func TestWebhookSend(t *testing.T) {
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("content-type = %q, want application/json", ct)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhook, err := NewWebhook(config.WebhookConfig{URL: server.URL})
	if err != nil {
		t.Fatalf("NewWebhook returned error: %v", err)
	}

	message := "[medianas] disk unhealthy"
	if err := webhook.Send(context.Background(), message); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	if gotBody["message"] != message {
		t.Fatalf("payload message = %q, want %q", gotBody["message"], message)
	}
}
