package config

import "testing"

func TestContainsMethod(t *testing.T) {
	methods := []string{"discord", "log"}
	if !containsMethod(methods, "discord") {
		t.Fatal("expected discord to be present")
	}
	if containsMethod(methods, "webhook") {
		t.Fatal("did not expect webhook to be present")
	}
}
