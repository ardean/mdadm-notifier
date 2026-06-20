package notify

import (
	"strings"
	"testing"
)

func TestSplitMessageShort(t *testing.T) {
	msg := "hello"
	parts := SplitMessage(msg, DiscordMessageLimit)
	if len(parts) != 1 || parts[0] != msg {
		t.Fatalf("parts = %#v, want single message", parts)
	}
}

func TestSplitMessageLong(t *testing.T) {
	msg := strings.Repeat("a", 5000)
	parts := SplitMessage(msg, DiscordMessageLimit)

	if len(parts) < 2 {
		t.Fatalf("expected multiple parts, got %d", len(parts))
	}

	for i, part := range parts {
		if len(part) > DiscordMessageLimit {
			t.Fatalf("part %d length = %d, exceeds limit", i, len(part))
		}
	}
}

func TestSplitMessagePrefersNewlines(t *testing.T) {
	line := strings.Repeat("x", 100) + "\n"
	msg := strings.Repeat(line, 50)
	parts := SplitMessage(msg, 1200)

	for _, part := range parts {
		if len(part) > DiscordMessageLimit {
			t.Fatalf("part length = %d, exceeds limit", len(part))
		}
	}
}

func TestTruncateMessage(t *testing.T) {
	msg := strings.Repeat("a", 100)
	got := TruncateMessage(msg, 50)
	if len(got) > 50 {
		t.Fatalf("truncated length = %d, want <= 50", len(got))
	}
	if !strings.HasSuffix(got, "... (truncated)") {
		t.Fatalf("expected truncation suffix, got %q", got)
	}
}
