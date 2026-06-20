package smart

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	selfTestEntryPattern = regexp.MustCompile(`(?m)^\s*#\s*(\d+)\s+(\S+(?:\s+\S+)?)\s+(.+?)\s+(\d+)%\s+(\d+)`)
	inProgressPattern    = regexp.MustCompile(`(?i)self-test execution status:.*self-test routine in progress`)
	powerOnHoursAttrPattern  = regexp.MustCompile(`(?im)^\s*\d+\s+Power_On_Hours.+?(\d[\d,]*)\s*$`)
	powerOnHoursNVMePattern  = regexp.MustCompile(`(?i)Power On Hours:\s*(\d[\d,]*)`)
	devicePattern        = regexp.MustCompile(`^(/dev/sd[a-z]+)\d+$`)
)

var ErrSelfTestInProgress = errors.New("self-test already in progress")

type SelfTestEntry struct {
	Num             int
	Description     string
	Status          string
	Remaining       int
	LifeTimeHours   int
	Passed          bool
	InProgress      bool
	IsShort         bool
	IsLong          bool
}

type SelfTestLog struct {
	Device       string
	InProgress   bool
	Entries      []SelfTestEntry
	LatestShort  *SelfTestEntry
	LatestLong   *SelfTestEntry
	PowerOnHours int
}

type DeviceInfo struct {
	PowerOnHours int
	InProgress   bool
}

func NormalizeDevice(device string) string {
	if match := devicePattern.FindStringSubmatch(device); len(match) == 2 {
		return match[1]
	}
	return device
}

func StartSelfTest(device, testType string) error {
	device = NormalizeDevice(device)

	var flag string
	switch testType {
	case "short":
		flag = "short"
	case "long":
		flag = "long"
	default:
		return fmt.Errorf("unknown self-test type: %s", testType)
	}

	cmd := execCommand("smartctl", "-t", flag, device)
	output, err := cmd.CombinedOutput()
	text := string(output)
	if err != nil {
		if outputIndicatesInProgress(text) {
			return fmt.Errorf("%w on %s", ErrSelfTestInProgress, device)
		}
		return fmt.Errorf("smartctl -t %s %s: %w\n%s", flag, device, err, text)
	}

	return nil
}

func ReadSelfTestLog(device string) (SelfTestLog, error) {
	device = NormalizeDevice(device)

	output, err := readSelfTestOutput(device)
	text := string(output)
	if err != nil {
		return SelfTestLog{}, fmt.Errorf("smartctl -l xselftest,selftest %s: %w\n%s", device, err, text)
	}

	return parseSelfTestLog(device, text), nil
}

func ReadDeviceInfo(device string) (DeviceInfo, error) {
	device = NormalizeDevice(device)

	output, err := readDeviceOutput(device)
	text := string(output)
	if err != nil {
		return DeviceInfo{}, fmt.Errorf("smartctl -x %s: %w\n%s", device, err, text)
	}

	hours, ok := parsePowerOnHours(text)
	if !ok {
		return DeviceInfo{}, fmt.Errorf("could not parse power-on hours for %s", device)
	}

	return DeviceInfo{
		PowerOnHours: hours,
		InProgress:   outputIndicatesInProgress(text),
	}, nil
}

func ReadPowerOnHours(device string) (int, error) {
	info, err := ReadDeviceInfo(device)
	if err != nil {
		return 0, err
	}
	return info.PowerOnHours, nil
}

func IsSelfTestInProgress(err error) bool {
	return errors.Is(err, ErrSelfTestInProgress)
}

func outputIndicatesInProgress(output string) bool {
	if inProgressPattern.MatchString(output) {
		return true
	}

	lower := strings.ToLower(output)
	return strings.Contains(lower, "self test in progress") ||
		strings.Contains(lower, "aborting current test")
}

func parseSelfTestLog(device, output string) SelfTestLog {
	log := SelfTestLog{
		Device:     device,
		InProgress: outputIndicatesInProgress(output),
	}

	for _, match := range selfTestEntryPattern.FindAllStringSubmatch(selfTestLogText(output), -1) {
		num, _ := strconv.Atoi(match[1])
		description := strings.TrimSpace(match[2])
		status := strings.TrimSpace(match[3])
		remaining, _ := strconv.Atoi(match[4])
		lifeTime, _ := strconv.Atoi(match[5])

		entry := SelfTestEntry{
			Num:           num,
			Description:   description,
			Status:        status,
			Remaining:     remaining,
			LifeTimeHours: lifeTime,
			InProgress:    strings.Contains(strings.ToLower(status), "in progress"),
			IsShort:       isShortTest(description),
			IsLong:        isLongTest(description),
		}
		entry.Passed = entryPassed(entry)

		log.Entries = append(log.Entries, entry)

		if entry.InProgress {
			log.InProgress = true
		}

		if entry.IsShort && (log.LatestShort == nil || entry.Num > log.LatestShort.Num) {
			latest := entry
			log.LatestShort = &latest
		}
		if entry.IsLong && (log.LatestLong == nil || entry.Num > log.LatestLong.Num) {
			latest := entry
			log.LatestLong = &latest
		}
	}

	return log
}

func selfTestLogText(output string) string {
	lower := strings.ToLower(output)
	extendedMarker := "smart extended self-test"
	regularMarker := "smart self-test log"

	extIdx := strings.Index(lower, extendedMarker)
	regIdx := strings.Index(lower, regularMarker)

	if extIdx >= 0 {
		end := len(output)
		if regIdx > extIdx {
			end = regIdx
		}
		return output[extIdx:end]
	}

	if regIdx >= 0 {
		return output[regIdx:]
	}

	return output
}

func isShortTest(description string) bool {
	return strings.Contains(strings.ToLower(description), "short")
}

func isLongTest(description string) bool {
	lower := strings.ToLower(description)
	return strings.Contains(lower, "extended") || strings.Contains(lower, "long")
}

func entryPassed(entry SelfTestEntry) bool {
	if entry.InProgress {
		return true
	}

	lower := strings.ToLower(entry.Status)
	if strings.Contains(lower, "completed without error") {
		return true
	}
	if strings.Contains(lower, "completed") {
		return false
	}
	if strings.Contains(lower, "aborted") || strings.Contains(lower, "interrupted") {
		return false
	}

	return true
}

func parsePowerOnHours(output string) (int, bool) {
	if match := powerOnHoursNVMePattern.FindStringSubmatch(output); len(match) == 2 {
		return parseHourValue(match[1])
	}

	if match := powerOnHoursAttrPattern.FindStringSubmatch(output); len(match) == 2 {
		return parseHourValue(match[1])
	}

	return 0, false
}

func parseHourValue(raw string) (int, bool) {
	raw = strings.ReplaceAll(raw, ",", "")
	hours, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}

	return hours, true
}

func (log SelfTestLog) HoursSince(entry *SelfTestEntry) (int, bool) {
	if entry == nil {
		return 0, false
	}
	if log.PowerOnHours == 0 {
		return 0, false
	}

	return log.PowerOnHours - entry.LifeTimeHours, true
}

func (log SelfTestLog) HoursSinceLastCompletedTest() (int, bool) {
	var latest *SelfTestEntry
	for i := range log.Entries {
		entry := &log.Entries[i]
		if entry.InProgress {
			continue
		}
		if latest == nil ||
			entry.LifeTimeHours > latest.LifeTimeHours ||
			(entry.LifeTimeHours == latest.LifeTimeHours && entry.Num > latest.Num) {
			latest = entry
		}
	}
	if latest == nil {
		return 0, false
	}

	return log.HoursSince(latest)
}

func (log SelfTestLog) allowsNewTest(minGap time.Duration) bool {
	if log.InProgress {
		return false
	}
	if minGap <= 0 {
		return true
	}

	hoursSince, ok := log.HoursSinceLastCompletedTest()
	if !ok {
		return true
	}

	return hoursSince >= int(minGap.Hours())
}

func (log SelfTestLog) ShortDue(interval, minGap time.Duration) bool {
	if interval <= 0 {
		return false
	}
	if !log.allowsNewTest(minGap) {
		return false
	}
	if log.LatestShort == nil {
		return true
	}
	if log.LatestShort.InProgress {
		return false
	}

	hoursSince, ok := log.HoursSince(log.LatestShort)
	if !ok {
		return true
	}

	return hoursSince >= int(interval.Hours())
}

func (log SelfTestLog) LongDue(interval, minGap time.Duration) bool {
	if interval <= 0 {
		return false
	}
	if !log.allowsNewTest(minGap) {
		return false
	}
	if log.LatestLong == nil {
		return true
	}
	if log.LatestLong.InProgress {
		return false
	}

	hoursSince, ok := log.HoursSince(log.LatestLong)
	if !ok {
		return true
	}

	return hoursSince >= int(interval.Hours())
}

func (log SelfTestLog) HealthSummary() (healthy bool, lines []string) {
	healthy = true

	if log.InProgress {
		for _, entry := range log.Entries {
			if entry.InProgress {
				lines = append(lines, fmt.Sprintf("self-test in progress: %s (%s)", entry.Description, entry.Status))
				return true, lines
			}
		}
	}

	if log.LatestShort != nil && !log.LatestShort.InProgress {
		line := fmt.Sprintf("latest short self-test: %s", log.LatestShort.Status)
		lines = append(lines, line)
		if !log.LatestShort.Passed {
			healthy = false
		}
	} else if log.LatestShort == nil {
		lines = append(lines, "latest short self-test: none recorded")
	}

	if log.LatestLong != nil && !log.LatestLong.InProgress {
		line := fmt.Sprintf("latest long self-test: %s", log.LatestLong.Status)
		lines = append(lines, line)
		if !log.LatestLong.Passed {
			healthy = false
		}
	} else if log.LatestLong == nil {
		lines = append(lines, "latest long self-test: none recorded")
	}

	return healthy, lines
}

func EnrichWithSelfTest(result Result) Result {
	device := NormalizeDevice(result.Device)

	log, err := ReadSelfTestLog(device)
	if err != nil {
		result.Healthy = false
		result.Summary += "\n" + err.Error()
		return result
	}

	powerOnHours, err := ReadPowerOnHours(device)
	if err != nil {
		result.Summary += "\n" + err.Error()
	} else {
		log.PowerOnHours = powerOnHours
	}

	selfTestHealthy, lines := log.HealthSummary()
	if !selfTestHealthy {
		result.Healthy = false
	}

	if len(lines) > 0 {
		result.Summary += "\n" + strings.Join(lines, "\n")
	}

	return result
}
