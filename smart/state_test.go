package smart

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadDeviceState(t *testing.T) {
	dir := t.TempDir()

	counters := map[string]int{
		counterReallocated:   104,
		counterUncorrectable: 17,
		counterDeviceErrors:  17,
	}

	if _, err := saveDeviceState(dir, "/dev/sdd", "ZA1EV6NC", counters); err != nil {
		t.Fatalf("save state: %v", err)
	}

	state, err := loadDeviceState(dir, "/dev/sdd", "ZA1EV6NC")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state == nil {
		t.Fatal("expected state")
	}
	if state.Serial != "ZA1EV6NC" {
		t.Fatalf("expected serial ZA1EV6NC, got %q", state.Serial)
	}
	if state.Counters[counterUncorrectable] != 17 {
		t.Fatalf("expected uncorrectable count 17, got %d", state.Counters[counterUncorrectable])
	}
}

func TestStateFileUsesSerial(t *testing.T) {
	dir := t.TempDir()
	path, err := stateFilePath(dir, "/dev/sdd", "ZA1EV6NC")
	if err != nil {
		t.Fatalf("state file path: %v", err)
	}
	if filepath.Base(path) != "ZA1EV6NC.json" {
		t.Fatalf("expected serial-based filename, got %s", path)
	}
}

func TestStateFileFallsBackToDevice(t *testing.T) {
	dir := t.TempDir()
	path, err := stateFilePath(dir, "/dev/sdd", "")
	if err != nil {
		t.Fatalf("state file path: %v", err)
	}
	if filepath.Base(path) != "sdd.json" {
		t.Fatalf("expected device-based filename, got %s", path)
	}
}

func TestPersistDeviceStateCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "data")
	signals := parseCriticalSignals(seagateMarginalOutput)

	persistDeviceState(dir, "/dev/sdd", signals)

	path := filepath.Join(dir, "ZA1EV6NC.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected state file at %s: %v", path, err)
	}
}

func TestLoadPreviousStateMissingFile(t *testing.T) {
	dir := t.TempDir()
	state := loadPreviousState(dir, "/dev/sdd", "MISSING")
	if state != nil {
		t.Fatalf("expected nil state, got %#v", state)
	}
}

func TestLoadPreviousStateInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "BAD.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("write invalid json: %v", err)
	}

	state := loadPreviousState(dir, "/dev/sdd", "BAD")
	if state != nil {
		t.Fatalf("expected nil state on invalid json, got %#v", state)
	}
}
