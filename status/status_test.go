package status

import (
	"testing"

	"github.com/ardean/mdadm-notifier/mdadm"
	"github.com/ardean/mdadm-notifier/smart"
)

func TestOverallHealthy(t *testing.T) {
	healthyRAID := RAIDStatus{Healthy: true}
	unhealthyRAID := RAIDStatus{Healthy: false}
	failedRAID := RAIDStatus{Error: "boom"}

	disks := []smart.DiskStatus{
		{Healthy: true},
		{Healthy: false},
	}

	if !OverallHealthy(healthyRAID, []smart.DiskStatus{{Healthy: true}}) {
		t.Fatal("expected healthy snapshot")
	}
	if OverallHealthy(unhealthyRAID, nil) {
		t.Fatal("expected unhealthy raid to fail overall health")
	}
	if OverallHealthy(failedRAID, nil) {
		t.Fatal("expected raid error to fail overall health")
	}
	if OverallHealthy(healthyRAID, disks) {
		t.Fatal("expected unhealthy disk to fail overall health")
	}
}

func TestRAIDFromHealth(t *testing.T) {
	health := mdadm.Health{
		Healthy: false,
		Detail:  "detail text",
		Devices: []string{"/dev/sdb"},
		Issues:  []string{"array state: degraded"},
	}

	status := RAIDFromHealth("/dev/md0", health, nil)
	if status.Device != "/dev/md0" || status.Detail != "detail text" {
		t.Fatalf("unexpected raid status: %+v", status)
	}
	if len(status.Devices) != 1 || status.Devices[0] != "/dev/sdb" {
		t.Fatalf("unexpected devices: %+v", status.Devices)
	}

	errStatus := RAIDFromHealth("/dev/md0", mdadm.Health{}, errTest())
	if errStatus.Error == "" {
		t.Fatal("expected raid error to be captured")
	}

	sync := &mdadm.SyncProgress{Active: true, Action: "recovery", Percent: 42.5}
	syncStatus := RAIDFromHealth("/dev/md0", mdadm.Health{Sync: sync}, nil)
	if syncStatus.Sync == nil || syncStatus.Sync.Percent != 42.5 {
		t.Fatalf("expected sync progress to be copied: %+v", syncStatus.Sync)
	}
}

func TestUpdateSync(t *testing.T) {
	store := NewStore()
	store.Update(Snapshot{
		RAID: RAIDStatus{Device: "/dev/md0", Healthy: true},
	})

	store.UpdateSync(&mdadm.SyncProgress{Active: true, Action: "recovery", Percent: 10})
	got := store.Get()
	if got.RAID.Sync == nil || got.RAID.Sync.Percent != 10 {
		t.Fatalf("expected sync to be updated: %+v", got.RAID.Sync)
	}

	store.UpdateSync(&mdadm.SyncProgress{Active: false})
	got = store.Get()
	if got.RAID.Sync != nil {
		t.Fatal("expected sync to be cleared when inactive")
	}
}

func errTest() error {
	return &testError{msg: "mdadm failed"}
}

type testError struct {
	msg string
}

func (e *testError) Error() string { return e.msg }
