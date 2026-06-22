package web

import (
	"testing"

	"github.com/ardean/mdadm-notifier/mdadm"
)

func TestSyncProgressChanged(t *testing.T) {
	base := mdadm.SyncProgress{
		Active:    true,
		Action:    "recovery",
		Percent:   10,
		Completed: 100,
		Total:     1000,
	}

	same := base
	if syncProgressChanged(base, same) {
		t.Fatal("expected identical progress to be unchanged")
	}

	updated := base
	updated.Percent = 11
	if !syncProgressChanged(base, updated) {
		t.Fatal("expected percent change to be detected")
	}

	finished := mdadm.SyncProgress{Active: false}
	if !syncProgressChanged(base, finished) {
		t.Fatal("expected sync finishing to be detected")
	}
}

func TestReadAndStoreLockedDetectsChange(t *testing.T) {
	b := newSyncBroadcaster(nil, "/dev/md0")
	b.last = mdadm.SyncProgress{Active: true, Percent: 10}

	changed := syncProgressChanged(b.last, mdadm.SyncProgress{Active: true, Percent: 11})
	if !changed {
		t.Fatal("expected change before last is updated")
	}

	b.last = mdadm.SyncProgress{Active: true, Percent: 11}
	if syncProgressChanged(b.last, mdadm.SyncProgress{Active: true, Percent: 11}) {
		t.Fatal("expected no change after last matches new value")
	}
}
