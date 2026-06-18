package mdadm

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
)

var devicePattern = regexp.MustCompile(`/dev/sd[a-z]+(?:\d+)?`)

func Detail(device string) (string, error) {
	cmd := exec.Command("mdadm", "-D", device)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return string(exitErr.Stderr), fmt.Errorf("mdadm -D %s: %w", device, err)
		}
		return "", fmt.Errorf("mdadm -D %s: %w", device, err)
	}

	return string(output), nil
}

func ParseDevices(mdadmOutput string) []string {
	seen := make(map[string]struct{})
	for _, match := range devicePattern.FindAllString(mdadmOutput, -1) {
		seen[match] = struct{}{}
	}

	devices := make([]string, 0, len(seen))
	for device := range seen {
		devices = append(devices, device)
	}
	sort.Strings(devices)

	return devices
}
