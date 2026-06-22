package smart

import (
	"strings"
	"testing"
)

const healthyPASSEDOutput = `
=== START OF READ SMART DATA SECTION ===
SMART overall-health self-assessment test result: PASSED
SMART Attributes Data Structure revision number: 10
ID# ATTRIBUTE_NAME          FLAGS    VALUE WORST THRESH FAIL RAW_VALUE
  5 Reallocated_Sector_Ct   PO--CK   100   100   010    -    0
187 Reported_Uncorrect      -O--CK   100   100   000    -    0
197 Current_Pending_Sector  -O--C-   100   100   000    -    0
198 Offline_Uncorrectable   ----C-   100   100   000    -    0
`

const seagateMarginalOutput = `
=== START OF INFORMATION SECTION ===
Serial Number:    ZA1EV6NC
=== START OF READ SMART DATA SECTION ===
SMART overall-health self-assessment test result: PASSED
See vendor-specific Attribute list for marginal Attributes.
SMART Attributes Data Structure revision number: 10
Vendor Specific SMART Attributes with Thresholds:
ID# ATTRIBUTE_NAME          FLAGS    VALUE WORST THRESH FAIL RAW_VALUE
  5 Reallocated_Sector_Ct   PO--CK   100   100   010    -    104
187 Reported_Uncorrect      -O--CK   083   083   000    -    17
197 Current_Pending_Sector  -O--C-   100   100   000    -    0
198 Offline_Uncorrectable   ----C-   100   100   000    -    0
190 Airflow_Temperature_Cel -O---K   054   035   040    Past 46 (Min/Max 33/49 #2639)

SMART Extended Comprehensive Error Log Version: 1 (5 sectors)
Device Error Count: 17
`

const nvmeCriticalOutput = `
=== START OF INFORMATION SECTION ===
Serial Number:    NVME123
=== START OF SMART DATA SECTION ===
SMART Health Status: OK
Critical Warning:                   0x04
Percentage Used:                    12%
`

func defaultCheckOptions() CheckOptions {
	return CheckOptions{
		ReallocatedThreshold:   1,
		UncorrectableThreshold: 1,
		PendingThreshold:       1,
		OfflineThreshold:       1,
		ErrorLogThreshold:      1,
	}
}

func TestParseMarginalWarning(t *testing.T) {
	if !parseMarginalWarning(seagateMarginalOutput) {
		t.Fatal("expected marginal warning")
	}
	if parseMarginalWarning(healthyPASSEDOutput) {
		t.Fatal("did not expect marginal warning")
	}
}

func TestParseATAAttributesSeagate(t *testing.T) {
	attrs := parseATAAttributes(seagateMarginalOutput)

	if attrs["Reallocated_Sector_Ct"].Raw != 104 {
		t.Fatalf("expected 104 reallocated sectors, got %d", attrs["Reallocated_Sector_Ct"].Raw)
	}
	if attrs["Reported_Uncorrect"].Raw != 17 {
		t.Fatalf("expected 17 reported uncorrect, got %d", attrs["Reported_Uncorrect"].Raw)
	}
	if attrs["Airflow_Temperature_Cel"].FailFlag != "Past" {
		t.Fatalf("expected Past fail flag, got %q", attrs["Airflow_Temperature_Cel"].FailFlag)
	}
}

func TestParseDeviceErrorCount(t *testing.T) {
	if got := parseDeviceErrorCount(seagateMarginalOutput); got != 17 {
		t.Fatalf("expected 17 device errors, got %d", got)
	}
}

func TestParseNVMeCritical(t *testing.T) {
	warnings := parseNVMeCritical(nvmeCriticalOutput)
	if len(warnings) != 1 {
		t.Fatalf("expected one NVMe warning, got %v", warnings)
	}
	if !strings.Contains(warnings[0], "0x04") {
		t.Fatalf("unexpected warning: %s", warnings[0])
	}
}

func TestEvaluateCriticalSignalsHealthy(t *testing.T) {
	signals := parseCriticalSignals(healthyPASSEDOutput)
	issues := evaluateCriticalSignals(signals, defaultCheckOptions(), nil)
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
}

func TestEvaluateCriticalSignalsSeagateMarginal(t *testing.T) {
	signals := parseCriticalSignals(seagateMarginalOutput)
	issues := evaluateCriticalSignals(signals, defaultCheckOptions(), nil)

	required := []string{
		"SMART marginal attributes reported by drive",
		"Reallocated_Sector_Ct: 104",
		"Reported_Uncorrect: 17",
		"Device error log count: 17",
		"Airflow_Temperature_Cel: threshold Past",
	}

	for _, want := range required {
		found := false
		for _, issue := range issues {
			if strings.Contains(issue, want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing issue %q in %v", want, issues)
		}
	}
}

func TestCompareCounterDeltas(t *testing.T) {
	prev := &DeviceState{
		Counters: map[string]int{
			counterUncorrectable: 15,
			counterReallocated:   104,
		},
	}

	signals := parseCriticalSignals(seagateMarginalOutput)
	issues := evaluateCriticalSignals(signals, defaultCheckOptions(), prev)

	found := false
	for _, issue := range issues {
		if issue == "Reported_Uncorrect increased from 15 to 17" {
			found = true
		}
		if strings.Contains(issue, "Reallocated_Sector_Ct increased") {
			t.Fatalf("did not expect reallocated delta, got %q", issue)
		}
	}
	if !found {
		t.Fatalf("expected uncorrectable delta issue, got %v", issues)
	}
}

func TestCheckDeviceSeagateMarginal(t *testing.T) {
	orig := execCommand
	t.Cleanup(func() { execCommand = orig })

	execCommand = func(name string, args ...string) commandRunner {
		return fakeCommandRunner{output: seagateMarginalOutput}
	}

	result := CheckDevice("/dev/sdd", CheckOptions{
		ReallocatedThreshold:   1,
		UncorrectableThreshold: 1,
		PendingThreshold:       1,
		OfflineThreshold:       1,
		ErrorLogThreshold:      1,
	})

	if result.Healthy {
		t.Fatalf("expected unhealthy disk, got summary:\n%s", result.Summary)
	}
	if !strings.Contains(result.Summary, "SMART overall-health: PASSED") {
		t.Fatalf("expected passed overall health in summary:\n%s", result.Summary)
	}
	if !strings.Contains(result.Summary, "Reallocated_Sector_Ct: 104") {
		t.Fatalf("expected reallocated sectors in summary:\n%s", result.Summary)
	}
	if strings.Contains(result.Summary, "PO--CK") {
		t.Fatalf("expected concise summary without raw attribute rows:\n%s", result.Summary)
	}
}

func TestInspectDeviceIdentityFields(t *testing.T) {
	orig := execCommand
	t.Cleanup(func() { execCommand = orig })

	output := `
=== START OF INFORMATION SECTION ===
Device Model:     WDC WD40EFRX-68WT0N0
Serial Number:    WD-WCC4E1234567
User Capacity:    4,000,787,030,016 bytes [4.00 TB]
=== START OF READ SMART DATA SECTION ===
SMART overall-health self-assessment test result: PASSED
ID# ATTRIBUTE_NAME          FLAGS    VALUE WORST THRESH FAIL RAW_VALUE
194 Temperature_Celsius     -O---K   062   055   000    -    38
`
	execCommand = func(name string, args ...string) commandRunner {
		return fakeCommandRunner{output: output}
	}

	status := InspectDevice("/dev/sda", defaultCheckOptions())
	if status.Manufacturer != "WDC" {
		t.Fatalf("manufacturer = %q", status.Manufacturer)
	}
	if status.Model != "WDC WD40EFRX-68WT0N0" {
		t.Fatalf("model = %q", status.Model)
	}
	if status.Capacity != "4.00 TB" {
		t.Fatalf("capacity = %q", status.Capacity)
	}
	if status.TemperatureC == nil || *status.TemperatureC != 38 {
		t.Fatalf("temperature = %v", status.TemperatureC)
	}
}

func TestCheckDeviceHealthy(t *testing.T) {
	orig := execCommand
	t.Cleanup(func() { execCommand = orig })

	execCommand = func(name string, args ...string) commandRunner {
		return fakeCommandRunner{output: healthyPASSEDOutput}
	}

	result := CheckDevice("/dev/sdb", defaultCheckOptions())
	if !result.Healthy {
		t.Fatalf("expected healthy disk, got summary:\n%s", result.Summary)
	}
}
