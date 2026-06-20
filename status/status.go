package status

import (
	"sync"
	"time"

	"github.com/ardean/mdadm-notifier/mdadm"
	"github.com/ardean/mdadm-notifier/smart"
)

type RAIDStatus struct {
	Device  string   `json:"device"`
	Healthy bool     `json:"healthy"`
	Issues  []string `json:"issues,omitempty"`
	Detail  string   `json:"detail,omitempty"`
	Devices []string `json:"devices,omitempty"`
	Error   string   `json:"error,omitempty"`
}

type ConfigView struct {
	CheckInterval         string `json:"check_interval"`
	SelfTestEnabled       bool   `json:"self_test_enabled"`
	SelfTestCheckInterval string `json:"self_test_check_interval,omitempty"`
	SelfTestShortInterval string `json:"self_test_short_interval,omitempty"`
	SelfTestLongInterval  string `json:"self_test_long_interval,omitempty"`
	SelfTestMinGap        string `json:"self_test_min_gap,omitempty"`
}

type Snapshot struct {
	Hostname  string            `json:"hostname"`
	CheckedAt time.Time         `json:"checked_at"`
	MDDevice  string            `json:"md_device"`
	Healthy   bool              `json:"healthy"`
	Config    ConfigView        `json:"config"`
	RAID      RAIDStatus        `json:"raid"`
	Disks     []smart.DiskStatus `json:"disks"`
}

type Store struct {
	mu       sync.RWMutex
	snapshot Snapshot
}

func NewStore() *Store {
	return &Store{}
}

func (s *Store) Update(snapshot Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = snapshot
}

func (s *Store) Get() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}

func RAIDFromHealth(device string, health mdadm.Health, err error) RAIDStatus {
	if err != nil {
		return RAIDStatus{
			Device: device,
			Error:  err.Error(),
		}
	}

	return RAIDStatus{
		Device:  device,
		Healthy: health.Healthy,
		Issues:  append([]string(nil), health.Issues...),
		Detail:  health.Detail,
		Devices: append([]string(nil), health.Devices...),
	}
}

func OverallHealthy(raid RAIDStatus, disks []smart.DiskStatus) bool {
	if raid.Error != "" || !raid.Healthy {
		return false
	}
	for _, disk := range disks {
		if !disk.Healthy {
			return false
		}
	}
	return true
}
