package smart

import "testing"

const seagateDeviceInfoOutput = `
=== START OF INFORMATION SECTION ===
Device Model:     ST2000DM008-2FR102
Serial Number:    ZA1EV6NC
User Capacity:    2,000,398,934,016 bytes [2.00 TB]
` + seagateMarginalOutput

const nvmeDeviceInfoOutput = `
=== START OF INFORMATION SECTION ===
Model Number:                       Samsung SSD 980 PRO 1TB
Serial Number:                      S5GXNX0N123456
User Capacity:                      1,000,204,886,016 bytes [1.00 TB]
=== START OF SMART DATA SECTION ===
SMART Health Status: OK
Temperature:                        42 Celsius
`

const scsiDeviceInfoOutput = `
=== START OF INFORMATION SECTION ===
Vendor:               SEAGATE
Product:              ST12000VN0007-2GS116
User Capacity:        12,000,138,625,024 bytes [12.0 TB]
`

func TestParseDeviceIdentitySeagate(t *testing.T) {
	attrs := parseATAAttributes(seagateMarginalOutput)
	identity := parseDeviceIdentity(seagateDeviceInfoOutput, attrs)

	if identity.Manufacturer != "" {
		t.Fatalf("expected empty manufacturer without vendor line, got %q", identity.Manufacturer)
	}
	if identity.Model != "ST2000DM008-2FR102" {
		t.Fatalf("model = %q", identity.Model)
	}
	if identity.Capacity != "2.00 TB" {
		t.Fatalf("capacity = %q", identity.Capacity)
	}
	if identity.TemperatureC == nil || *identity.TemperatureC != 46 {
		t.Fatalf("temperature = %v", identity.TemperatureC)
	}
}

func TestParseDeviceIdentityNVMe(t *testing.T) {
	identity := parseDeviceIdentity(nvmeDeviceInfoOutput, nil)

	if identity.Manufacturer != "Samsung" {
		t.Fatalf("manufacturer = %q", identity.Manufacturer)
	}
	if identity.Model != "Samsung SSD 980 PRO 1TB" {
		t.Fatalf("model = %q", identity.Model)
	}
	if identity.Capacity != "1.00 TB" {
		t.Fatalf("capacity = %q", identity.Capacity)
	}
	if identity.TemperatureC == nil || *identity.TemperatureC != 42 {
		t.Fatalf("temperature = %v", identity.TemperatureC)
	}
}

func TestParseDeviceIdentitySCSI(t *testing.T) {
	identity := parseDeviceIdentity(scsiDeviceInfoOutput, nil)

	if identity.Manufacturer != "SEAGATE" {
		t.Fatalf("manufacturer = %q", identity.Manufacturer)
	}
	if identity.Model != "ST12000VN0007-2GS116" {
		t.Fatalf("model = %q", identity.Model)
	}
	if identity.Capacity != "12.0 TB" {
		t.Fatalf("capacity = %q", identity.Capacity)
	}
}
