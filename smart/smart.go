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
			Summary: fmt.Sprintf("%s: smartctl failed: %v", device, err),
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
