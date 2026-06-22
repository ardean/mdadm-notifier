package mdadm

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	rebuildStatusPattern   = regexp.MustCompile(`(?im)^\s*Rebuild Status\s*:\s*(\d+(?:\.\d+)?)%\s*complete`)
	recoveryStatusPattern  = regexp.MustCompile(`(?im)^\s*Recovery Status\s*:\s*(\d+(?:\.\d+)?)%\s*complete`)
	resyncStatusPattern    = regexp.MustCompile(`(?im)^\s*Resync Status\s*:\s*(\d+(?:\.\d+)?)%\s*complete`)
	recoveringStatePattern = regexp.MustCompile(`(?im)^\s*State\s*:\s*.+\b(recovering|resyncing|reshaping)\b`)
)

func resolveSyncProgress(device, detail, mdstat string) *SyncProgress {
	if mdstat != "" {
		if progress, ok := ParseSyncProgressWithDetail(device, mdstat, detail); ok {
			return &progress
		}
	}

	if progress, ok := ParseDetailSyncProgress(detail); ok {
		return &progress
	}

	return nil
}

func ParseDetailSyncProgress(detail string) (SyncProgress, bool) {
	if match := rebuildStatusPattern.FindStringSubmatch(detail); len(match) == 2 {
		return detailSyncProgress("recovery", match[1])
	}
	if match := recoveryStatusPattern.FindStringSubmatch(detail); len(match) == 2 {
		return detailSyncProgress("recovery", match[1])
	}
	if match := resyncStatusPattern.FindStringSubmatch(detail); len(match) == 2 {
		return detailSyncProgress("resync", match[1])
	}
	if match := recoveringStatePattern.FindStringSubmatch(detail); len(match) == 2 {
		return SyncProgress{
			Active: true,
			Action: normalizeSyncAction(match[1]),
		}, true
	}

	return SyncProgress{}, false
}

func detailSyncProgress(action, percentRaw string) (SyncProgress, bool) {
	percent, err := strconv.ParseFloat(percentRaw, 64)
	if err != nil {
		return SyncProgress{}, false
	}

	return SyncProgress{
		Active:  true,
		Action:  action,
		Percent: percent,
	}, true
}

func normalizeSyncAction(state string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "resyncing":
		return "resync"
	case "reshaping":
		return "reshape"
	default:
		return "recovery"
	}
}
