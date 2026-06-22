package mdadm

import "testing"

const sampleRecoveringDetail = `/dev/md0:
           Version : 1.2
             State : clean, degraded, recovering
    Rebuild Status : 1% complete
     Number   Major   Minor   RaidDevice State
       6       8        0        0      spare rebuilding   /dev/sda
       5       8       48        1      active sync   /dev/sdd
`

func TestParseDetailSyncProgressRebuildStatus(t *testing.T) {
	progress, ok := ParseDetailSyncProgress(sampleRecoveringDetail)
	if !ok {
		t.Fatal("expected rebuild progress from mdadm detail")
	}
	if !progress.Active || progress.Action != "recovery" {
		t.Fatalf("unexpected progress: %+v", progress)
	}
	if progress.Percent != 1 {
		t.Fatalf("expected 1%% complete, got %v", progress.Percent)
	}
}

func TestParseDetailSyncProgressRecoveringStateOnly(t *testing.T) {
	detail := `
             State : clean, degraded, recovering
    Failed Devices : 0
`
	progress, ok := ParseDetailSyncProgress(detail)
	if !ok {
		t.Fatal("expected recovering state to report active sync")
	}
	if !progress.Active || progress.Action != "recovery" {
		t.Fatalf("unexpected progress: %+v", progress)
	}
	if progress.Percent != 0 {
		t.Fatalf("expected no percent without rebuild status line, got %v", progress.Percent)
	}
}

func TestResolveSyncProgressPrefersMdstat(t *testing.T) {
	mdstat := `md0 : active raid5 sdc1[0] sdd1[1]
      1464725760 blocks level 5, 64k chunk, algorithm 2 [6/5] [UUUUU_]
      [==>..................]  recovery = 12.6% (37043392/292945152) finish=127.5min speed=33440K/sec
`
	progress := resolveSyncProgress("/dev/md0", sampleRecoveringDetail, mdstat)
	if progress == nil {
		t.Fatal("expected sync progress")
	}
	if progress.Percent != 12.6 {
		t.Fatalf("expected mdstat percent, got %v", progress.Percent)
	}
	if progress.SpeedKBps != 33440 {
		t.Fatalf("expected mdstat speed, got %v", progress.SpeedKBps)
	}
}

func TestResolveSyncProgressFallsBackToDetail(t *testing.T) {
	progress := resolveSyncProgress("/dev/md0", sampleRecoveringDetail, "")
	if progress == nil {
		t.Fatal("expected detail fallback progress")
	}
	if progress.Percent != 1 {
		t.Fatalf("expected rebuild percent from detail, got %v", progress.Percent)
	}
}
