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

func TestParseHealthFromExtendedOutput(t *testing.T) {
	output := `
=== START OF READ SMART DATA SECTION ===
SMART overall-health self-assessment test result: PASSED
SMART Attributes Data Structure revision number: 16
  9 Power_On_Hours          0x0032   100   100   000    Old_age   Always       -       54321
=== START OF SMART EXTENDED SELF-TEST LOG ===
SMART Extended Self-test Log Version: 1 (1 sectors)
# 1  Extended offline    Completed without error       00%      5400         7948857160
`

	healthy, reason := parseHealth(output)
	if !healthy {
		t.Fatalf("expected healthy disk from extended output, got: %s", reason)
	}

	hours, ok := parsePowerOnHours(output)
	if !ok || hours != 54321 {
		t.Fatalf("expected power-on hours 54321, got %d ok=%v", hours, ok)
	}

	log := parseSelfTestLog("/dev/sdd", output)
	if log.LatestLong == nil || log.LatestLong.LifeTimeHours != 5400 {
		t.Fatalf("expected extended self-test entry, got %#v", log.LatestLong)
	}
}
