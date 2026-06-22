package smart

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	deviceModelPattern     = regexp.MustCompile(`(?m)^Device Model:\s+(.+)$`)
	modelNumberPattern     = regexp.MustCompile(`(?m)^Model Number:\s+(.+)$`)
	vendorPattern          = regexp.MustCompile(`(?m)^Vendor:\s+(.+)$`)
	productPattern         = regexp.MustCompile(`(?m)^Product:\s+(.+)$`)
	userCapacityPattern    = regexp.MustCompile(`(?m)^User Capacity:\s+.+?\[([^\]]+)\]`)
	nvmeTemperaturePattern = regexp.MustCompile(`(?m)^Temperature:\s+(\d+)\s+Celsius`)
)

var temperatureAttributeNames = []string{
	"Temperature_Celsius",
	"Airflow_Temperature_Cel",
	"Drive_Temperature",
	"Temperature_Case",
}

type DeviceIdentity struct {
	Manufacturer string
	Model        string
	Capacity     string
	TemperatureC *int
}

func parseDeviceIdentity(output string, attrs map[string]AttributeReading) DeviceIdentity {
	vendor := parseField(vendorPattern, output)
	model := parseField(deviceModelPattern, output)
	if model == "" {
		model = parseField(modelNumberPattern, output)
	}
	if model == "" {
		model = parseField(productPattern, output)
	}

	return DeviceIdentity{
		Manufacturer: deriveManufacturer(vendor, model),
		Model:        model,
		Capacity:     parseField(userCapacityPattern, output),
		TemperatureC: parseTemperatureCelsius(output, attrs),
	}
}

func parseField(pattern *regexp.Regexp, output string) string {
	if match := pattern.FindStringSubmatch(output); len(match) == 2 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func deriveManufacturer(vendor, model string) string {
	if vendor != "" {
		return vendor
	}
	fields := strings.Fields(model)
	if len(fields) >= 2 {
		return fields[0]
	}
	return ""
}

func parseTemperatureCelsius(output string, attrs map[string]AttributeReading) *int {
	if match := nvmeTemperaturePattern.FindStringSubmatch(output); len(match) == 2 {
		if temp, err := strconv.Atoi(match[1]); err == nil {
			return &temp
		}
	}

	for _, name := range temperatureAttributeNames {
		attr, ok := attrs[name]
		if !ok {
			continue
		}
		if temp := temperatureFromAttribute(attr); temp != nil {
			return temp
		}
	}

	return nil
}

func temperatureFromAttribute(attr AttributeReading) *int {
	if attr.RawText != "" {
		token := strings.Fields(strings.TrimSpace(attr.RawText))[0]
		token = strings.TrimSuffix(token, "(")
		if value, err := strconv.Atoi(token); err == nil && isReasonableTemperature(value) {
			return &value
		}
	}
	if isReasonableTemperature(attr.Raw) {
		value := attr.Raw
		return &value
	}
	return nil
}

func isReasonableTemperature(value int) bool {
	return value > 0 && value < 200
}
