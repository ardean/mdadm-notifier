package smart

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	counterReallocated   = "Reallocated_Sector_Ct"
	counterUncorrectable = "Reported_Uncorrect"
	counterPending       = "Current_Pending_Sector"
	counterOffline       = "Offline_Uncorrectable"
	counterDeviceErrors  = "DeviceErrorCount"
)

var (
	marginalWarningPattern = regexp.MustCompile(`(?i)marginal Attributes`)
	attributeRowPattern    = regexp.MustCompile(`^\s*(\d+)\s+(\S+)\s+\S+\s+\d+\s+\d+\s+\d+\s+(\S+)\s+(.+)$`)
	deviceErrorCountPattern = regexp.MustCompile(`(?m)^Device Error Count:\s*(\d+)`)
	serialNumberPattern    = regexp.MustCompile(`(?m)^Serial Number:\s+(\S+)`)
	criticalWarningPattern = regexp.MustCompile(`(?i)Critical Warning:\s*(.+)$`)
)

type AttributeReading struct {
	ID       int
	Name     string
	Raw      int
	FailFlag string
	RawText  string
}

type CriticalSignals struct {
	MarginalWarning bool
	DeviceErrors    int
	Attributes      map[string]AttributeReading
	NVMeWarnings    []string
	Serial          string
}

type CheckOptions struct {
	ReallocatedThreshold   int
	UncorrectableThreshold int
	PendingThreshold       int
	OfflineThreshold       int
	ErrorLogThreshold      int
	StateDir               string
}

func parseCriticalSignals(output string) CriticalSignals {
	signals := CriticalSignals{
		Attributes: parseATAAttributes(output),
	}
	signals.MarginalWarning = marginalWarningPattern.MatchString(output)
	signals.DeviceErrors = parseDeviceErrorCount(output)
	signals.NVMeWarnings = parseNVMeCritical(output)
	signals.Serial = parseSerialNumber(output)
	return signals
}

func parseMarginalWarning(output string) bool {
	return marginalWarningPattern.MatchString(output)
}

func parseSerialNumber(output string) string {
	if match := serialNumberPattern.FindStringSubmatch(output); len(match) == 2 {
		return match[1]
	}
	return ""
}

func parseATAAttributes(output string) map[string]AttributeReading {
	attrs := make(map[string]AttributeReading)
	for _, line := range strings.Split(output, "\n") {
		match := attributeRowPattern.FindStringSubmatch(line)
		if len(match) != 5 {
			continue
		}

		id, _ := strconv.Atoi(match[1])
		raw, ok := parseRawValue(match[4])
		if !ok {
			continue
		}

		name := match[2]
		attrs[name] = AttributeReading{
			ID:       id,
			Name:     name,
			Raw:      raw,
			FailFlag: match[3],
			RawText:  strings.TrimSpace(match[4]),
		}
	}
	return attrs
}

func parseRawValue(rawColumn string) (int, bool) {
	rawColumn = strings.TrimSpace(rawColumn)
	if rawColumn == "" || rawColumn == "-" {
		return 0, true
	}

	token := strings.Fields(rawColumn)[0]
	token = strings.ReplaceAll(token, ",", "")
	value, err := strconv.Atoi(token)
	if err != nil {
		return 0, false
	}
	return value, true
}

func parseDeviceErrorCount(output string) int {
	if match := deviceErrorCountPattern.FindStringSubmatch(output); len(match) == 2 {
		count, err := strconv.Atoi(match[1])
		if err == nil {
			return count
		}
	}
	return 0
}

func parseNVMeCritical(output string) []string {
	var warnings []string
	for _, line := range strings.Split(output, "\n") {
		match := criticalWarningPattern.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) != 2 {
			continue
		}

		value := strings.TrimSpace(match[1])
		if value == "" || value == "0x00" || strings.EqualFold(value, "0x0") || value == "0" {
			continue
		}
		warnings = append(warnings, "Critical Warning: "+value)
	}
	return warnings
}

func parseThresholdFailures(attrs map[string]AttributeReading) []string {
	var issues []string
	for _, attr := range attrs {
		flag := strings.ToUpper(attr.FailFlag)
		if flag != "PAST" && flag != "FAIL" {
			continue
		}
		issues = append(issues, fmt.Sprintf("%s: threshold %s (%s)", attr.Name, attr.FailFlag, attr.RawText))
	}
	return issues
}

func (signals CriticalSignals) counters() map[string]int {
	counters := map[string]int{
		counterDeviceErrors: signals.DeviceErrors,
	}

	for name, reading := range signals.Attributes {
		counters[name] = reading.Raw
	}
	return counters
}

func evaluateCriticalSignals(signals CriticalSignals, opts CheckOptions, prev *DeviceState) (issues []string) {
	if signals.MarginalWarning {
		issues = append(issues, "SMART marginal attributes reported by drive")
	}

	issues = append(issues, evaluateAttributeThreshold(signals, counterReallocated, opts.ReallocatedThreshold)...)
	issues = append(issues, evaluateAttributeThreshold(signals, counterUncorrectable, opts.UncorrectableThreshold)...)
	issues = append(issues, evaluateAttributeThreshold(signals, counterPending, opts.PendingThreshold)...)
	issues = append(issues, evaluateAttributeThreshold(signals, counterOffline, opts.OfflineThreshold)...)
	issues = append(issues, parseThresholdFailures(signals.Attributes)...)

	if signals.DeviceErrors >= opts.ErrorLogThreshold {
		issues = append(issues, fmt.Sprintf("Device error log count: %d", signals.DeviceErrors))
	}

	issues = append(issues, signals.NVMeWarnings...)

	if prev != nil {
		issues = append(issues, compareCounterDeltas(signals.counters(), prev.Counters)...)
	}

	return issues
}

func evaluateAttributeThreshold(signals CriticalSignals, name string, threshold int) []string {
	if threshold <= 0 {
		return nil
	}

	reading, ok := signals.Attributes[name]
	if !ok {
		return nil
	}
	if reading.Raw < threshold {
		return nil
	}

	return []string{fmt.Sprintf("%s: %d", name, reading.Raw)}
}

func compareCounterDeltas(current, previous map[string]int) []string {
	var issues []string
	monitored := []string{
		counterReallocated,
		counterUncorrectable,
		counterPending,
		counterOffline,
		counterDeviceErrors,
	}

	for _, name := range monitored {
		newVal, ok := current[name]
		if !ok {
			continue
		}
		oldVal, ok := previous[name]
		if !ok {
			continue
		}
		if newVal > oldVal {
			issues = append(issues, fmt.Sprintf("%s increased from %d to %d", name, oldVal, newVal))
		}
	}
	return issues
}
