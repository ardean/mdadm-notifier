package alert

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ardean/mdadm-notifier/mdadm"
	"github.com/ardean/mdadm-notifier/smart"
)

type DiskIssue struct {
	Device string
	Issues []string
}

type State struct {
	RAIDError  string
	RAIDIssues []string
	Disks      []DiskIssue
}

type Tracker struct {
	last       State
	lastNotify time.Time
	hadIssues  bool
}

type Result struct {
	Notify  bool
	Message string
}

func BuildState(raidErr error, raid mdadm.Health, unhealthy []smart.DiskStatus) State {
	state := State{
		RAIDIssues: append([]string(nil), raid.Issues...),
	}

	if raidErr != nil {
		state.RAIDError = raidErr.Error()
		return state
	}

	for _, disk := range unhealthy {
		state.Disks = append(state.Disks, DiskIssue{
			Device: disk.Device,
			Issues: append([]string(nil), disk.Issues...),
		})
	}

	sortDiskIssues(state.Disks)
	sort.Strings(state.RAIDIssues)
	return state
}

func (s State) Unhealthy() bool {
	if s.RAIDError != "" {
		return true
	}
	if len(s.RAIDIssues) > 0 {
		return true
	}
	return len(s.Disks) > 0
}

func (t *Tracker) Evaluate(now time.Time, reminderInterval time.Duration, mdDevice string, raidErr error, raid mdadm.Health, unhealthy []smart.DiskStatus) Result {
	current := BuildState(raidErr, raid, unhealthy)

	if !current.Unhealthy() {
		if t.hadIssues {
			t.hadIssues = false
			t.last = current
			t.lastNotify = now
			return Result{Notify: true, Message: "Health check: all issues cleared"}
		}
		t.last = current
		return Result{}
	}

	changed := !statesEqual(t.last, current)
	first := !t.hadIssues
	reminder := reminderInterval > 0 && !t.lastNotify.IsZero() && now.Sub(t.lastNotify) >= reminderInterval

	if first || changed || reminder {
		t.hadIssues = true
		t.last = current
		t.lastNotify = now

		message := FormatHealthAlert(mdDevice, raidErr, raid, unhealthy)
		if reminder && !first && !changed {
			message = "Reminder — ongoing issues:\n" + message
		}
		return Result{Notify: true, Message: message}
	}

	t.last = current
	return Result{}
}

func FormatHealthAlert(mdDevice string, raidErr error, raid mdadm.Health, unhealthy []smart.DiskStatus) string {
	if raidErr != nil {
		return fmt.Sprintf("RAID health check failed for %s: %v", mdDevice, raidErr)
	}

	var message strings.Builder
	message.WriteString("Health check found issues")

	if !raid.Healthy {
		message.WriteString("\nRAID: ")
		message.WriteString(strings.Join(raid.Issues, ", "))
	}

	for _, disk := range unhealthy {
		message.WriteString("\n")
		message.WriteString(disk.Summary)
	}

	return message.String()
}

func statesEqual(a, b State) bool {
	if a.RAIDError != b.RAIDError {
		return false
	}
	if !stringSlicesEqual(a.RAIDIssues, b.RAIDIssues) {
		return false
	}
	if len(a.Disks) != len(b.Disks) {
		return false
	}
	for i := range a.Disks {
		if a.Disks[i].Device != b.Disks[i].Device {
			return false
		}
		if !stringSlicesEqual(a.Disks[i].Issues, b.Disks[i].Issues) {
			return false
		}
	}
	return true
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sortDiskIssues(disks []DiskIssue) {
	sort.Slice(disks, func(i, j int) bool {
		return disks[i].Device < disks[j].Device
	})
	for i := range disks {
		sort.Strings(disks[i].Issues)
	}
}
