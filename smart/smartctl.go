package smart

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	smartctlCommandFailureMask = 0x7
	smartctlMaxAttempts        = 3
)

var smartctlRetryDelay = 500 * time.Millisecond

type commandRunner interface {
	CombinedOutput() ([]byte, error)
}

var execCommand = func(name string, args ...string) commandRunner {
	return exec.Command(name, args...)
}

type exitError interface {
	error
	ExitCode() int
}

func runSmartctl(device string, args ...string) ([]byte, error) {
	cmdArgs := append(append([]string{}, args...), device)
	command := smartctlCommandDesc(args, device)

	var output []byte
	var err error
	for attempt := 1; attempt <= smartctlMaxAttempts; attempt++ {
		output, err = execCommand("smartctl", cmdArgs...).CombinedOutput()
		if interpretErr := interpretSmartctlError(command, output, err); interpretErr == nil {
			return output, nil
		} else if attempt == smartctlMaxAttempts || !isRetryableSmartctlError(output, err) {
			return output, interpretErr
		}
		time.Sleep(smartctlRetryDelay)
	}

	return output, interpretSmartctlError(command, output, err)
}

func smartctlCommandDesc(args []string, device string) string {
	return fmt.Sprintf("smartctl %s %s", strings.Join(args, " "), device)
}

func interpretSmartctlError(command string, output []byte, err error) error {
	if err == nil {
		return nil
	}

	code, ok := exitStatusCode(err)
	if !ok {
		return fmt.Errorf("%s: %w", command, err)
	}

	if code&smartctlCommandFailureMask != 0 {
		return fmt.Errorf("%s: %s", command, smartctlFailureDetail(output, code))
	}

	if hasSmartOutput(string(output)) {
		return nil
	}

	return fmt.Errorf("%s: %s", command, smartctlFailureDetail(output, code))
}

func exitStatusCode(err error) (int, bool) {
	var exitErr exitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), true
	}
	return 0, false
}

func hasSmartOutput(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "smart overall-health") ||
		strings.Contains(lower, "smart health status")
}

func smartctlFailureDetail(output []byte, code int) string {
	if detail := parseSmartctlFailureDetail(string(output)); detail != "" {
		return detail
	}
	if reason := smartctlCommandFailureReason(code); reason != "" {
		return reason
	}
	if code != 0 {
		return fmt.Sprintf("exit status %d", code)
	}
	return "unknown error"
}

func parseSmartctlFailureDetail(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "failed:") {
			continue
		}

		if idx := strings.LastIndex(lower, "failed:"); idx >= 0 {
			detail := strings.TrimSpace(line[idx+len("failed:"):])
			if detail != "" {
				return detail
			}
		}
	}
	return ""
}

func smartctlCommandFailureReason(code int) string {
	var reasons []string
	if code&0x1 != 0 {
		reasons = append(reasons, "invalid command line")
	}
	if code&0x2 != 0 {
		reasons = append(reasons, "device open failed")
	}
	if code&0x4 != 0 {
		reasons = append(reasons, "SMART command failed")
	}
	return strings.Join(reasons, "; ")
}

func isRetryableSmartctlError(output []byte, err error) bool {
	if detail := parseSmartctlFailureDetail(string(output)); detail != "" {
		return strings.Contains(strings.ToLower(detail), "scsi error")
	}
	return false
}

func smartctlErrorMessage(output []byte, err error) string {
	if detail := parseSmartctlFailureDetail(string(output)); detail != "" {
		return detail
	}
	if code, ok := exitStatusCode(err); ok {
		if reason := smartctlCommandFailureReason(code); reason != "" {
			return reason
		}
	}
	msg := err.Error()
	if idx := strings.LastIndex(msg, ": "); idx >= 0 {
		if tail := strings.TrimSpace(msg[idx+2:]); tail != "" {
			return tail
		}
	}
	return msg
}

func readDeviceOutput(device string) ([]byte, error) {
	return runSmartctl(NormalizeDevice(device), "-x")
}

func readSelfTestOutput(device string) ([]byte, error) {
	return runSmartctl(NormalizeDevice(device), "-l", "xselftest,selftest")
}
