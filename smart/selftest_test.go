package smart

import (
	"strings"
	"testing"
	"time"
)

const sampleSelfTestLog = `
SMART Self-test log structure revision number 1
Num  Test_Description    Status                  Remaining  LifeTime(hours)  LBA_of_first_error
# 1  Short offline       Completed without error       00%      5000         -
# 2  Extended offline    Completed: read failure       00%      5100         12345678
# 3  Short offline       Completed without error       00%      5200         -
`

const sampleExtendedSelfTestLog = `
SMART Extended Self-test Log Version: 1 (1 sectors)
Num  Test_Description    Status                  Remaining  LifeTime(hours)  LBA_of_first_error
# 1  Short offline       Completed without error       00%      9691         -
# 2  Extended offline    Completed: read failure       90%      9691         7948857160
# 3  Short offline       Completed: read failure       90%      9687         7948857160
`

const sampleCombinedSelfTestLog = sampleExtendedSelfTestLog + `
SMART Self-test log structure revision number 1
Num  Test_Description    Status                  Remaining  LifeTime(hours)  LBA_of_first_error
# 1  Short offline       Completed without error       00%      9691         -
# 2  Extended offline    Completed: read failure       90%      9691         3653889864
# 3  Short offline       Completed: read failure       90%      9687         3653889864
# 4  Short offline       Completed without error       00%         113         -
`

func TestParseSelfTestLog(t *testing.T) {
	log := parseSelfTestLog("/dev/sdb", sampleSelfTestLog)

	if len(log.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(log.Entries))
	}

	if log.LatestShort == nil || log.LatestShort.Num != 3 {
		t.Fatalf("expected latest short test #3, got %#v", log.LatestShort)
	}
	if !log.LatestShort.Passed {
		t.Fatal("expected latest short test to pass")
	}

	if log.LatestLong == nil || log.LatestLong.Num != 2 {
		t.Fatalf("expected latest long test #2, got %#v", log.LatestLong)
	}
	if log.LatestLong.Passed {
		t.Fatal("expected latest long test to fail")
	}
}

func TestParseExtendedSelfTestLog(t *testing.T) {
	log := parseSelfTestLog("/dev/sdd", sampleExtendedSelfTestLog)

	if len(log.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(log.Entries))
	}

	if log.LatestLong == nil || log.LatestLong.LifeTimeHours != 9691 {
		t.Fatalf("expected latest long test at lifetime 9691, got %#v", log.LatestLong)
	}
	if log.LatestLong.Passed {
		t.Fatal("expected failed extended self-test")
	}
}

func TestPreferExtendedSelfTestLog(t *testing.T) {
	log := parseSelfTestLog("/dev/sdd", sampleCombinedSelfTestLog)

	if len(log.Entries) != 3 {
		t.Fatalf("expected 3 entries from extended log only, got %d", len(log.Entries))
	}

	if log.LatestLong == nil || log.LatestLong.Num != 2 {
		t.Fatalf("expected latest long test #2, got %#v", log.LatestLong)
	}
	if log.LatestShort == nil || log.LatestShort.Num != 3 {
		t.Fatalf("expected latest short test #3, got %#v", log.LatestShort)
	}
}

func TestSelfTestLogText(t *testing.T) {
	got := selfTestLogText(sampleCombinedSelfTestLog)
	if strings.Contains(got, "3653889864") {
		t.Fatal("regular self-test log section should be excluded when extended log is present")
	}
	if !strings.Contains(got, "7948857160") {
		t.Fatal("expected extended self-test log section")
	}

	got = selfTestLogText(sampleSelfTestLog)
	if !strings.Contains(got, "SMART Self-test log") {
		t.Fatal("expected regular self-test log when extended log is absent")
	}
}

func TestSelfTestLogShortDue(t *testing.T) {
	log := parseSelfTestLog("/dev/sdb", sampleSelfTestLog)
	log.PowerOnHours = 5200 + 200

	if !log.ShortDue(168*time.Hour, 0) {
		t.Fatal("expected short test to be due")
	}

	log.PowerOnHours = 5200 + 24
	if log.ShortDue(168*time.Hour, 0) {
		t.Fatal("expected short test not to be due yet")
	}
}

func TestSelfTestLogLongDue(t *testing.T) {
	log := parseSelfTestLog("/dev/sdb", sampleSelfTestLog)
	log.PowerOnHours = 5100 + 800

	if !log.LongDue(720*time.Hour, 0) {
		t.Fatal("expected long test to be due")
	}
}

func TestSelfTestLogInProgress(t *testing.T) {
	output := `
Self-test execution status:  ( 249) Self-test routine in progress...
# 1  Short offline       Self-test routine in progress  90%      5300         -
`
	log := parseSelfTestLog("/dev/sdb", output)
	if !log.InProgress {
		t.Fatal("expected in-progress self-test")
	}
	if log.ShortDue(168*time.Hour, 0) {
		t.Fatal("should not schedule while test is running")
	}
}

func TestSelfTestMinGap(t *testing.T) {
	output := `
# 1  Short offline       Completed without error       00%      5000         -
# 2  Extended offline    Completed without error       00%      5290         -
`
	log := parseSelfTestLog("/dev/sdb", output)
	log.PowerOnHours = 5295

	if log.ShortDue(168*time.Hour, 24*time.Hour) {
		t.Fatal("short test should wait for min gap after long test")
	}
	if log.LongDue(720*time.Hour, 24*time.Hour) {
		t.Fatal("long test should wait for min gap after recent test")
	}

	log.PowerOnHours = 5290 + 24
	if !log.ShortDue(168*time.Hour, 24*time.Hour) {
		t.Fatal("short test should be allowed after min gap")
	}
}

func TestHealthSummaryFailedLongTest(t *testing.T) {
	log := parseSelfTestLog("/dev/sdb", sampleSelfTestLog)
	healthy, lines := log.HealthSummary()
	if healthy {
		t.Fatal("expected unhealthy summary due to failed long test")
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 summary lines, got %v", lines)
	}
}

func TestNormalizeDevice(t *testing.T) {
	if got := NormalizeDevice("/dev/sdb1"); got != "/dev/sdb" {
		t.Fatalf("expected /dev/sdb, got %s", got)
	}
	if got := NormalizeDevice("/dev/nvme0n1"); got != "/dev/nvme0n1" {
		t.Fatalf("expected unchanged nvme device, got %s", got)
	}
}

func TestOutputIndicatesInProgress(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{
			name:   "execution status",
			output: "Self-test execution status:  ( 249) Self-test routine in progress...",
			want:   true,
		},
		{
			name:   "start failure",
			output: "Can't start self-test without aborting current test (90% remaining)",
			want:   true,
		},
		{
			name:   "completed",
			output: "Self-test execution status:  (   0) The previous self-test completed without error",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := outputIndicatesInProgress(tt.output); got != tt.want {
				t.Fatalf("outputIndicatesInProgress() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParsePowerOnHours(t *testing.T) {
	output := `
  9 Power_On_Hours          0x0032   100   100   000    Old_age   Always       -       12345
`
	hours, ok := parsePowerOnHours(output)
	if !ok || hours != 12345 {
		t.Fatalf("expected 12345 power-on hours, got %d ok=%v", hours, ok)
	}

	nvme := `
Power On Hours:          12,345
`
	hours, ok = parsePowerOnHours(nvme)
	if !ok || hours != 12345 {
		t.Fatalf("expected 12345 nvme power-on hours, got %d ok=%v", hours, ok)
	}
}