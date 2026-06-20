package smart

import (
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
	ReadOK  bool
	Summary string
}

func CheckDevice(device string, opts CheckOptions) Result {
	return InspectDevice(device, opts).Result()
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
