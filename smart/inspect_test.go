package smart

import "testing"

func TestBuildMonitoredCounters(t *testing.T) {
	prevVal := 10
	prev := &DeviceState{
		Counters: map[string]int{
			counterReallocated:  prevVal,
			counterDeviceErrors: 1,
		},
	}

	signals := CriticalSignals{
		Attributes: map[string]AttributeReading{
			counterReallocated: {Name: counterReallocated, Raw: 12},
		},
		DeviceErrors: 3,
	}

	opts := CheckOptions{
		ReallocatedThreshold:  1,
		ErrorLogThreshold:       1,
		UncorrectableThreshold:  0,
		PendingThreshold:        0,
		OfflineThreshold:        0,
	}

	monitored := buildMonitoredCounters(signals, prev, opts)
	if len(monitored) != 2 {
		t.Fatalf("expected 2 monitored counters, got %d", len(monitored))
	}

	var reallocated MonitoredCounter
	for _, item := range monitored {
		if item.Name == counterReallocated {
			reallocated = item
		}
	}

	if reallocated.Value != 12 || reallocated.Previous == nil || *reallocated.Previous != prevVal {
		t.Fatalf("unexpected reallocated counter: %+v", reallocated)
	}
	if !reallocated.Increased || !reallocated.Alert {
		t.Fatalf("expected increased alert counter: %+v", reallocated)
	}
}

func TestInspectDeviceReadFailure(t *testing.T) {
	status := InspectDevice("/dev/missing", CheckOptions{})
	if status.ReadOK {
		t.Fatal("expected read failure")
	}
	if status.Healthy {
		t.Fatal("expected unhealthy status")
	}
	if status.Summary == "" {
		t.Fatal("expected summary on read failure")
	}
}
