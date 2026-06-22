package mdadm

import "testing"

const sampleResyncMdstat = `Personalities : [raid1] [raid6] [raid5] [raid4]
md127 : active raid1 sdb2[1] sda2[0]
      1953511936 blocks super 1.2 [2/2] [UU]
      [==>..................]  resync = 12.5% (244618752/1953511936) finish=149.0min speed=76524K/sec

unused devices: <none>
`

const sampleRecoveryMdstat = `Personalities : [raid5] [raid4] [raid1]
md0 : active raid5 sdc1[0] sdd1[1] sde1[2] sdf1[3] sdg1[4]
      1464725760 blocks level 5, 64k chunk, algorithm 2 [6/5] [UUUUU_]
      [==>..................]  recovery = 12.6% (37043392/292945152) finish=127.5min speed=33440K/sec

unused devices: <none>
`

func TestParseSyncProgressResync(t *testing.T) {
	progress, ok := ParseSyncProgress("/dev/md127", sampleResyncMdstat)
	if !ok {
		t.Fatal("expected sync progress")
	}
	if !progress.Active || progress.Action != "resync" {
		t.Fatalf("unexpected progress: %+v", progress)
	}
	if progress.Percent != 12.5 {
		t.Fatalf("expected 12.5%%, got %v", progress.Percent)
	}
	if progress.Completed != 244618752 || progress.Total != 1953511936 {
		t.Fatalf("unexpected block counts: %+v", progress)
	}
	if progress.FinishMinutes != 149.0 {
		t.Fatalf("expected finish time, got %v", progress.FinishMinutes)
	}
	if progress.SpeedKBps != 76524 {
		t.Fatalf("expected speed, got %v", progress.SpeedKBps)
	}
}

func TestParseSyncProgressRecovery(t *testing.T) {
	progress, ok := ParseSyncProgress("/dev/md0", sampleRecoveryMdstat)
	if !ok {
		t.Fatal("expected sync progress")
	}
	if progress.Action != "recovery" {
		t.Fatalf("expected recovery action, got %q", progress.Action)
	}
}

func TestParseSyncProgressPending(t *testing.T) {
	mdstat := `md0 : active raid1 sda1[0] sdb1[1]
      1046528 blocks super 1.2 [2/2] [UU]
      resync=PENDING
`
	progress, ok := ParseSyncProgress("/dev/md0", mdstat)
	if !ok || !progress.Pending || progress.Action != "resync" {
		t.Fatalf("unexpected pending progress: %+v ok=%v", progress, ok)
	}
}

func TestParseSyncProgressDelayed(t *testing.T) {
	mdstat := `md0 : active raid1 sda1[0] sdb1[1]
      1046528 blocks super 1.2 [2/2] [UU]
      recovery=DELAYED
`
	progress, ok := ParseSyncProgress("/dev/md0", mdstat)
	if !ok || !progress.Delayed || progress.Action != "recovery" {
		t.Fatalf("unexpected delayed progress: %+v ok=%v", progress, ok)
	}
}

func TestParseSyncProgressNoMatch(t *testing.T) {
	mdstat := `md0 : active raid1 sda1[0] sdb1[1]
      1046528 blocks super 1.2 [2/2] [UU]
`
	if _, ok := ParseSyncProgress("/dev/md0", mdstat); ok {
		t.Fatal("expected no sync progress")
	}
	if _, ok := ParseSyncProgress("/dev/md1", mdstat); ok {
		t.Fatal("expected no sync progress for other array")
	}
}

func TestFindArraySectionIgnoresOtherArrays(t *testing.T) {
	mdstat := `md0 : active raid1 sda1[0] sdb1[1]
      1046528 blocks super 1.2 [2/2] [UU]
md1 : active raid1 sdc1[0] sdd1[1]
      1046528 blocks super 1.2 [2/2] [UU]
      [====>................]  resync = 20.0% (100/500) finish=1.0min speed=1000K/sec
`
	progress, ok := ParseSyncProgress("/dev/md0", mdstat)
	if ok {
		t.Fatalf("expected md0 to have no sync progress, got %+v", progress)
	}

	progress, ok = ParseSyncProgress("/dev/md1", mdstat)
	if !ok || progress.Percent != 20.0 {
		t.Fatalf("expected md1 sync progress, got %+v ok=%v", progress, ok)
	}
}
