package smart

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var safeFilenamePattern = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

type DeviceState struct {
	Device    string         `json:"device"`
	Serial    string         `json:"serial,omitempty"`
	UpdatedAt time.Time      `json:"updated_at"`
	Counters  map[string]int `json:"counters"`
}

func loadDeviceState(dir, device, serial string) (*DeviceState, error) {
	if dir == "" {
		return nil, nil
	}

	path, err := stateFilePath(dir, device, serial)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var state DeviceState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode state %s: %w", path, err)
	}
	return &state, nil
}

func saveDeviceState(dir, device, serial string, counters map[string]int) (string, error) {
	if dir == "" {
		return "", nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	path, err := stateFilePath(dir, device, serial)
	if err != nil {
		return "", err
	}

	state := DeviceState{
		Device:    device,
		Serial:    serial,
		UpdatedAt: time.Now().UTC(),
		Counters:  counters,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return "", err
	}

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return "", err
	}
	return path, nil
}

func stateFilePath(dir, device, serial string) (string, error) {
	name := sanitizeStateFilename(serial)
	if name == "" {
		name = sanitizeStateFilename(device)
	}
	if name == "" {
		return "", fmt.Errorf("could not derive state filename for %s", device)
	}
	return filepath.Join(dir, name+".json"), nil
}

func sanitizeStateFilename(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.TrimPrefix(value, "/dev/")
	return safeFilenamePattern.ReplaceAllString(value, "_")
}

func persistDeviceState(dir, device string, signals CriticalSignals, prev *DeviceState) {
	if dir == "" {
		log.Printf("smart state: persistence disabled (empty state directory) for %s", device)
		return
	}

	counters := signals.counters()
	if prev != nil && countersEqual(prev.Counters, counters) {
		return
	}

	path, err := saveDeviceState(dir, device, signals.Serial, counters)
	if err != nil {
		log.Printf("smart state: failed to save state for %s: %v", device, err)
		return
	}
	log.Printf("smart state: saved %s", path)
}

func countersEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for name, value := range a {
		if b[name] != value {
			return false
		}
	}
	return true
}

func loadPreviousState(dir, device, serial string) *DeviceState {
	if dir == "" {
		return nil
	}

	state, err := loadDeviceState(dir, device, serial)
	if err != nil {
		log.Printf("smart state: failed to load state for %s: %v", device, err)
		return nil
	}
	return state
}
