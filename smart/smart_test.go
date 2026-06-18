package smart

import "testing"

func TestParseHealthPassed(t *testing.T) {
	output := `
SMART overall-health self-assessment test result: PASSED
`

	healthy, reason := parseHealth(output)
	if !healthy {
		t.Fatalf("expected healthy disk, got: %s", reason)
	}
}

func TestParseHealthFailed(t *testing.T) {
	output := `
SMART overall-health self-assessment test result: FAILED
`

	healthy, reason := parseHealth(output)
	if healthy {
		t.Fatal("expected unhealthy disk")
	}
	if reason != "SMART overall-health: FAILED" {
		t.Fatalf("unexpected reason: %s", reason)
	}
}

func TestParseHealthStatusOK(t *testing.T) {
	output := `
SMART Health Status: OK
`

	healthy, _ := parseHealth(output)
	if !healthy {
		t.Fatal("expected healthy disk from SMART Health Status")
	}
}
