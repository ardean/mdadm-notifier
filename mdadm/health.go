package mdadm

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	statePattern         = regexp.MustCompile(`(?m)^\s*State\s*:\s*(.+)$`)
	failedDevicesPattern = regexp.MustCompile(`(?m)^\s*Failed Devices\s*:\s*(\d+)$`)
)

type Health struct {
	Healthy bool
	Detail  string
	Devices []string
	Issues  []string
}

func CheckHealth(device string) (Health, error) {
	detail, err := Detail(device)
	if err != nil {
		return Health{}, err
	}

	devices := ParseDevices(detail)
	issues := collectIssues(detail)

	return Health{
		Healthy: len(issues) == 0,
		Detail:  detail,
		Devices: devices,
		Issues:  issues,
	}, nil
}

func collectIssues(detail string) []string {
	var issues []string

	if match := failedDevicesPattern.FindStringSubmatch(detail); len(match) == 2 {
		if count, err := strconv.Atoi(match[1]); err == nil && count > 0 {
			issues = append(issues, fmt.Sprintf("%d failed device(s)", count))
		}
	}

	if match := statePattern.FindStringSubmatch(detail); len(match) == 2 {
		state := strings.ToLower(strings.TrimSpace(match[1]))
		if state != "" && state != "clean" && state != "active" {
			issues = append(issues, "array state: "+strings.TrimSpace(match[1]))
		}
	}

	return issues
}
