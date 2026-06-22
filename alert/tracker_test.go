package alert

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ardean/mdadm-notifier/mdadm"
	"github.com/ardean/mdadm-notifier/smart"
)

func TestTrackerNotifiesOnFirstIssue(t *testing.T) {
	t.Parallel()

	tracker := &Tracker{}
	now := time.Now()
	unhealthy := []smart.DiskStatus{{
		Device:  "/dev/sda",
		Issues:  []string{"Reallocated_Sector_Ct: 5"},
		Summary: "/dev/sda: SMART overall-health: PASSED\nReallocated_Sector_Ct: 5",
	}}

	result := tracker.Evaluate(now, 0, "/dev/md0", nil, mdadm.Health{Healthy: true}, unhealthy)
	if !result.Notify {
		t.Fatal("expected notification on first issue")
	}
}

func TestTrackerSkipsUnchangedIssuesDespiteTemperatureFluctuation(t *testing.T) {
	t.Parallel()

	tracker := &Tracker{}
	now := time.Now()
	raid := mdadm.Health{Healthy: true}

	makeDisk := func(temp int) []smart.DiskStatus {
		return []smart.DiskStatus{{
			Device: "/dev/sda",
			Issues: []string{
				"SMART marginal attributes reported by drive",
				"Reallocated_Sector_Ct: 104",
				"Reported_Uncorrect: 17",
				"Device error log count: 17",
				"Airflow_Temperature_Cel: threshold Past",
			},
			Summary: fmt.Sprintf("/dev/sda: SMART overall-health: PASSED\nAirflow_Temperature_Cel: threshold Past (%d C)", temp),
		}}
	}

	first := tracker.Evaluate(now, 0, "/dev/md0", nil, raid, makeDisk(42))
	if !first.Notify {
		t.Fatal("expected first notification")
	}

	second := tracker.Evaluate(now.Add(time.Hour), 0, "/dev/md0", nil, raid, makeDisk(43))
	if second.Notify {
		t.Fatal("expected no notification when only volatile SMART readings change")
	}
}

func TestTrackerSkipsUnchangedIssues(t *testing.T) {
	t.Parallel()

	tracker := &Tracker{}
	now := time.Now()
	unhealthy := []smart.DiskStatus{{
		Device:  "/dev/sda",
		Issues:  []string{"Reallocated_Sector_Ct: 5"},
		Summary: "/dev/sda: SMART overall-health: PASSED\nReallocated_Sector_Ct: 5",
	}}
	raid := mdadm.Health{Healthy: true}

	first := tracker.Evaluate(now, 0, "/dev/md0", nil, raid, unhealthy)
	if !first.Notify {
		t.Fatal("expected first notification")
	}

	second := tracker.Evaluate(now.Add(time.Hour), 0, "/dev/md0", nil, raid, unhealthy)
	if second.Notify {
		t.Fatal("expected no notification for unchanged issues")
	}
}

func TestTrackerNotifiesOnIssueChange(t *testing.T) {
	t.Parallel()

	tracker := &Tracker{}
	now := time.Now()
	raid := mdadm.Health{Healthy: true}

	tracker.Evaluate(now, 0, "/dev/md0", nil, raid, []smart.DiskStatus{{
		Device: "/dev/sda",
		Issues: []string{"Reallocated_Sector_Ct: 5"},
	}})

	changed := tracker.Evaluate(now.Add(time.Hour), 0, "/dev/md0", nil, raid, []smart.DiskStatus{{
		Device: "/dev/sda",
		Issues: []string{"Reallocated_Sector_Ct: 5", "Reallocated_Sector_Ct increased from 5 to 6"},
	}})
	if !changed.Notify {
		t.Fatal("expected notification when issues change")
	}
}

func TestTrackerNotifiesOnRecovery(t *testing.T) {
	t.Parallel()

	tracker := &Tracker{}
	now := time.Now()
	raid := mdadm.Health{Healthy: true}

	tracker.Evaluate(now, 0, "/dev/md0", nil, raid, []smart.DiskStatus{{
		Device: "/dev/sda",
		Issues: []string{"Reallocated_Sector_Ct: 5"},
	}})

	recovered := tracker.Evaluate(now.Add(time.Hour), 0, "/dev/md0", nil, raid, nil)
	if !recovered.Notify {
		t.Fatal("expected recovery notification")
	}
	if recovered.Message != "Health check: all issues cleared" {
		t.Fatalf("unexpected recovery message: %q", recovered.Message)
	}
}

func TestTrackerReminderForOngoingIssues(t *testing.T) {
	t.Parallel()

	tracker := &Tracker{}
	start := time.Now()
	unhealthy := []smart.DiskStatus{{
		Device: "/dev/sda",
		Issues: []string{"Reallocated_Sector_Ct: 5"},
	}}
	raid := mdadm.Health{Healthy: true}
	reminder := 24 * time.Hour

	tracker.Evaluate(start, reminder, "/dev/md0", nil, raid, unhealthy)

	tooSoon := tracker.Evaluate(start.Add(2*time.Hour), reminder, "/dev/md0", nil, raid, unhealthy)
	if tooSoon.Notify {
		t.Fatal("expected no reminder before interval elapsed")
	}

	reminded := tracker.Evaluate(start.Add(25*time.Hour), reminder, "/dev/md0", nil, raid, unhealthy)
	if !reminded.Notify {
		t.Fatal("expected reminder notification")
	}
	if reminded.Message[:len("Reminder — ongoing issues:")] != "Reminder — ongoing issues:" {
		t.Fatalf("expected reminder prefix, got: %q", reminded.Message)
	}
}

func TestTrackerSkipsRepeatedRAIDErrors(t *testing.T) {
	t.Parallel()

	tracker := &Tracker{}
	now := time.Now()
	raidErr := errors.New("mdadm unavailable")

	first := tracker.Evaluate(now, 0, "/dev/md0", raidErr, mdadm.Health{}, nil)
	if !first.Notify {
		t.Fatal("expected notification on first raid error")
	}

	second := tracker.Evaluate(now.Add(time.Hour), 0, "/dev/md0", raidErr, mdadm.Health{}, nil)
	if second.Notify {
		t.Fatal("expected no notification for unchanged raid error")
	}
}

func TestBuildStateSortsDiskIssues(t *testing.T) {
	t.Parallel()

	state := BuildState(nil, mdadm.Health{}, []smart.DiskStatus{
		{Device: "/dev/sdb", Issues: []string{"b", "a"}},
		{Device: "/dev/sda", Issues: []string{"z"}},
	})

	if len(state.Disks) != 2 || state.Disks[0].Device != "/dev/sda" {
		t.Fatalf("unexpected disk order: %+v", state.Disks)
	}
	if state.Disks[0].Issues[0] != "z" || state.Disks[1].Issues[0] != "a" {
		t.Fatalf("unexpected issue order: %+v", state.Disks)
	}
}
