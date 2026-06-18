package mdadm

import "testing"

func TestParseDevices(t *testing.T) {
	output := `/dev/md0:
           Version : 1.2
     Number   Major   Minor   RaidDevice State
       0       8       16       0      active sync   /dev/sdb
       1       8       32       1      active sync   /dev/sdc
`

	devices := ParseDevices(output)
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d: %v", len(devices), devices)
	}
	if devices[0] != "/dev/sdb" || devices[1] != "/dev/sdc" {
		t.Fatalf("unexpected devices: %v", devices)
	}
}

func TestParseDevicesDeduplicates(t *testing.T) {
	output := `/dev/sdb
/dev/sdb
/dev/sdc1
`

	devices := ParseDevices(output)
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d: %v", len(devices), devices)
	}
}
