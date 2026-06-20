package smart

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	overallHealthPattern = regexp.MustCompile(`(?i)SMART overall-health self-assessment test result:\s*(\S+)`)
	smartStatusPattern   = regexp.MustCompile(`(?i)SMART Health Status:\s*(\S+)`)
)

type Result struct {
	Device  string
	Healthy bool
	Summary string
}

func CheckDevice(device string, opts CheckOptions) Result {
	output, err := readDeviceOutput(device)
	text := string(output)

	if err != nil {
		return Result{
			Device:  device,
			Healthy: false,
			Summary: fmt.Sprintf("%s: smartctl failed: %v\n%s", device, err, text),
		}
	}

	healthy, reason := parseHealth(text)
	signals := parseCriticalSignals(text)
	prev := loadPreviousState(opts.StateDir, device, signals.Serial)
	issues := evaluateCriticalSignals(signals, opts, prev)

	summary := fmt.Sprintf("%s: %s", device, reason)
	if len(issues) > 0 {
		healthy = false
		summary += "\n" + strings.Join(issues, "\n")
		summary += "\n" + extractRelevantLines(text)
	} else if !healthy {
		summary += "\n" + extractRelevantLines(text)
	}

	persistDeviceState(opts.StateDir, device, signals)

	return Result{
		Device:  device,
		Healthy: healthy,
		Summary: summary,
	}
}

func parseHealth(output string) (bool, string) {
	if match := overallHealthPattern.FindStringSubmatch(output); len(match) == 2 {
		status := strings.ToUpper(match[1])
		if status == "PASSED" {
			return true, "SMART overall-health: PASSED"
		}
		return false, "SMART overall-health: " + match[1]
	}

	if match := smartStatusPattern.FindStringSubmatch(output); len(match) == 2 {
		status := strings.ToUpper(match[1])
		if status == "OK" {
			return true, "SMART health status: OK"
		}
		return false, "SMART health status: " + match[1]
	}

	return false, "could not determine SMART health from smartctl output"
}

func extractRelevantLines(output string) string {
	keep := []string{
		"SMART overall-health",
		"SMART Health Status",
		"marginal Attributes",
		"Reallocated_Sector",
		"Reported_Uncorrect",
		"Current_Pending_Sector",
		"Offline_Uncorrectable",
		"Airflow_Temperature",
		"Temperature_Celsius",
		"Device Error Count",
		"Media_Wearout_Indicator",
		"Percentage Used",
		"Critical Warning",
	}

	var lines []string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		for _, fragment := range keep {
			if strings.Contains(trimmed, fragment) {
				lines = append(lines, trimmed)
				break
			}
		}
	}

	return strings.Join(lines, "\n")
}
