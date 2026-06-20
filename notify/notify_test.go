package notify

import (
	"context"
	"strings"
	"testing"

	"github.com/ardean/mdadm-notifier/config"
)

type stubNotifier struct {
	name     string
	messages []string
}

func (s *stubNotifier) Name() string { return s.name }

func (s *stubNotifier) Send(_ context.Context, message string) error {
	s.messages = append(s.messages, message)
	return nil
}

func (s *stubNotifier) Close() error { return nil }

func TestManagerSend(t *testing.T) {
	first := &stubNotifier{name: "first"}
	second := &stubNotifier{name: "second"}

	m := &Manager{
		hostname:  "medianas",
		notifiers: []Notifier{first, second},
	}

	m.Send("disk unhealthy")

	want := "[medianas] disk unhealthy"
	if len(first.messages) != 1 || first.messages[0] != want {
		t.Fatalf("first notifier messages = %#v, want %q", first.messages, want)
	}
	if len(second.messages) != 1 || second.messages[0] != want {
		t.Fatalf("second notifier messages = %#v, want %q", second.messages, want)
	}
}

func TestManagerSendTruncatesLongMessage(t *testing.T) {
	notifier := &stubNotifier{name: "first"}
	m := &Manager{
		hostname:  "medianas",
		notifiers: []Notifier{notifier},
	}

	m.Send(strings.Repeat("a", 3000))

	if len(notifier.messages) != 1 {
		t.Fatalf("expected one message, got %d", len(notifier.messages))
	}
	if len(notifier.messages[0]) > DiscordMessageLimit {
		t.Fatalf("message length = %d, want <= %d", len(notifier.messages[0]), DiscordMessageLimit)
	}
	if !strings.HasSuffix(notifier.messages[0], "... (truncated)") {
		t.Fatalf("expected truncation suffix, got %q", notifier.messages[0][len(notifier.messages[0])-20:])
	}
}

func TestNewManagerLogOnly(t *testing.T) {
	m, err := NewManager(config.Config{
		NotifyMethods: []string{"log"},
		Hostname:      "medianas",
	})
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	defer m.Close()

	if len(m.Methods()) != 1 || m.Methods()[0] != "log" {
		t.Fatalf("methods = %#v, want [log]", m.Methods())
	}
}

func TestNewManagerWebhookRequiresURL(t *testing.T) {
	_, err := NewManager(config.Config{
		NotifyMethods: []string{"webhook"},
		Hostname:      "medianas",
	})
	if err == nil {
		t.Fatal("expected error when WEBHOOK_URL is missing")
	}
}

func TestFormatMethods(t *testing.T) {
	if got := FormatMethods([]string{"discord", "log"}); got != "discord, log" {
		t.Fatalf("FormatMethods() = %q", got)
	}
}
