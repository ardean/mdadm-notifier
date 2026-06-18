package smart

import (
	"fmt"
	"os/exec"
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

func CheckDevice(device string) Result {
	cmd := exec.Command("smartctl", "-a", device)
	output, err := cmd.CombinedOutput()
	text := string(output)

	if err != nil {
		return Result{
			Device:  device,
			Healthy: false,
			Summary: fmt.Sprintf("%s: smartctl failed: %v\n%s", device, err, text),
		}
	}

	healthy, reason := parseHealth(text)
	summary := fmt.Sprintf("%s: %s", device, reason)
	if !healthy {
		summary += "\n" + extractRelevantLines(text)
	}

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
		"Reallocated_Sector",
		"Current_Pending_Sector",
		"Offline_Uncorrectable",
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
