package mdadm

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const mdstatPath = "/proc/mdstat"

var (
	syncRunningPattern = regexp.MustCompile(`(?i)(resync|recovery|reshape|check)\s*=\s*(\d+(?:\.\d+)?)%\s*\((\d+)/(\d+)\)(?:\s+finish=([\d.]+)min)?(?:\s+speed=(\d+)K/sec)?`)
	syncPendingPattern = regexp.MustCompile(`(?i)(resync|recovery|reshape|check)=PENDING`)
	syncDelayedPattern = regexp.MustCompile(`(?i)(resync|recovery|reshape|check)=DELAYED`)
)

type SyncProgress struct {
	Active        bool    `json:"active"`
	Action        string  `json:"action,omitempty"`
	Percent       float64 `json:"percent,omitempty"`
	Completed     uint64  `json:"completed_blocks,omitempty"`
	Total         uint64  `json:"total_blocks,omitempty"`
	FinishMinutes float64 `json:"finish_minutes,omitempty"`
	SpeedKBps     uint64  `json:"speed_kbps,omitempty"`
	Pending       bool    `json:"pending,omitempty"`
	Delayed       bool    `json:"delayed,omitempty"`
}

func ReadMdstat() (string, error) {
	data, err := os.ReadFile(mdstatPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func ParseSyncProgress(device, mdstat string) (SyncProgress, bool) {
	return ParseSyncProgressWithDetail(device, mdstat, "")
}

func ParseSyncProgressWithDetail(device, mdstat, detail string) (SyncProgress, bool) {
	section := findArraySectionForDevice(device, mdstat, detail)
	if section == "" {
		return SyncProgress{}, false
	}

	return parseSyncFromSection(section)
}

func findArraySectionForDevice(device, mdstat, detail string) string {
	if name := mdDeviceName(device); name != "" {
		if section := findArraySection(mdstat, name); section != "" {
			return section
		}
	}

	if detail != "" {
		if section := findArraySectionByMembers(mdstat, ParseDevices(detail)); section != "" {
			return section
		}
	}

	return ""
}

func mdDeviceName(device string) string {
	base := filepath.Base(device)
	if !strings.HasPrefix(base, "md") {
		return ""
	}
	return base
}

var arrayHeaderPattern = regexp.MustCompile(`^md\d+\s*:`)

func findArraySection(mdstat, name string) string {
	lines := strings.Split(mdstat, "\n")
	var section []string
	inSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if arrayHeaderPattern.MatchString(trimmed) {
			if inSection {
				break
			}

			arrayName := strings.TrimSpace(strings.SplitN(trimmed, ":", 2)[0])
			if arrayName == name {
				inSection = true
				section = append(section, line)
			}
			continue
		}

		if strings.HasPrefix(trimmed, "unused devices:") {
			if inSection {
				break
			}
			continue
		}

		if inSection {
			section = append(section, line)
		}
	}

	if len(section) == 0 {
		return ""
	}
	return strings.Join(section, "\n")
}

func findArraySectionByMembers(mdstat string, members []string) string {
	memberBases := memberBaseNames(members)
	if len(memberBases) == 0 {
		return ""
	}

	lines := strings.Split(mdstat, "\n")
	var bestSection []string
	bestScore := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !arrayHeaderPattern.MatchString(trimmed) {
			continue
		}

		score := scoreMemberMatches(trimmed, memberBases)
		if score <= bestScore {
			continue
		}

		bestScore = score
		bestSection = collectArraySection(lines, i)
	}

	minScore := 2
	if len(memberBases) < minScore {
		minScore = len(memberBases)
	}
	if bestScore < minScore {
		return ""
	}

	if len(bestSection) == 0 {
		return ""
	}
	return strings.Join(bestSection, "\n")
}

func memberBaseNames(members []string) []string {
	bases := make([]string, 0, len(members))
	seen := make(map[string]struct{}, len(members))
	for _, member := range members {
		base := filepath.Base(member)
		if base == "" || base == "." {
			continue
		}
		if _, ok := seen[base]; ok {
			continue
		}
		seen[base] = struct{}{}
		bases = append(bases, base)
	}
	return bases
}

func scoreMemberMatches(header string, memberBases []string) int {
	score := 0
	for _, member := range memberBases {
		if strings.Contains(header, member+"[") {
			score++
		}
	}
	return score
}

func collectArraySection(lines []string, headerIndex int) []string {
	section := []string{lines[headerIndex]}
	for i := headerIndex + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if arrayHeaderPattern.MatchString(trimmed) {
			break
		}
		if strings.HasPrefix(trimmed, "unused devices:") {
			break
		}
		section = append(section, lines[i])
	}
	return section
}

func parseSyncFromSection(section string) (SyncProgress, bool) {
	for _, line := range strings.Split(section, "\n") {
		if match := syncRunningPattern.FindStringSubmatch(line); len(match) > 0 {
			percent, _ := strconv.ParseFloat(match[2], 64)
			completed, _ := strconv.ParseUint(match[3], 10, 64)
			total, _ := strconv.ParseUint(match[4], 10, 64)

			progress := SyncProgress{
				Active:    true,
				Action:    strings.ToLower(match[1]),
				Percent:   percent,
				Completed: completed,
				Total:     total,
			}
			if len(match) > 5 && match[5] != "" {
				progress.FinishMinutes, _ = strconv.ParseFloat(match[5], 64)
			}
			if len(match) > 6 && match[6] != "" {
				progress.SpeedKBps, _ = strconv.ParseUint(match[6], 10, 64)
			}
			return progress, true
		}

		if match := syncPendingPattern.FindStringSubmatch(line); len(match) > 0 {
			return SyncProgress{
				Active:  true,
				Action:  strings.ToLower(match[1]),
				Pending: true,
			}, true
		}

		if match := syncDelayedPattern.FindStringSubmatch(line); len(match) > 0 {
			return SyncProgress{
				Active:  true,
				Action:  strings.ToLower(match[1]),
				Delayed: true,
			}, true
		}
	}

	return SyncProgress{}, false
}
