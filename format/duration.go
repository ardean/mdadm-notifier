package format

import (
	"fmt"
	"time"
)

func Duration(d time.Duration) string {
	if d < 24*time.Hour {
		return formatUnits(d)
	}

	days := d / (24 * time.Hour)
	remainder := d % (24 * time.Hour)
	if remainder == 0 {
		return fmt.Sprintf("%dd", days)
	}

	return fmt.Sprintf("%dd%s", days, formatUnits(remainder))
}

func formatUnits(d time.Duration) string {
	if d == 0 {
		return ""
	}

	hours := d / time.Hour
	d %= time.Hour
	minutes := d / time.Minute

	switch {
	case hours > 0 && minutes > 0:
		return fmt.Sprintf("%dh%dm", hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%dh", hours)
	case minutes > 0:
		return fmt.Sprintf("%dm", minutes)
	default:
		return d.String()
	}
}
