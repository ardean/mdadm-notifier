package smart

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	CounterReallocated   = counterReallocated
	CounterUncorrectable = counterUncorrectable
	CounterPending       = counterPending
	CounterOffline       = counterOffline
	CounterDeviceErrors  = counterDeviceErrors
)

var monitoredCounterNames = []string{
	counterReallocated,
	counterUncorrectable,
	counterPending,
	counterOffline,
	counterDeviceErrors,
}

type MonitoredCounter struct {
	Name      string `json:"name"`
	Value     int    `json:"value"`
	Previous  *int   `json:"previous,omitempty"`
	Threshold int    `json:"threshold"`
	Alert     bool   `json:"alert"`
	Increased bool   `json:"increased"`
}

type AttributeView struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Raw      int    `json:"raw"`
	RawText  string `json:"raw_text"`
	FailFlag string `json:"fail_flag,omitempty"`
}

type SelfTestEntryView struct {
	Num           int    `json:"num"`
	Description   string `json:"description"`
	Status        string `json:"status"`
	Remaining     int    `json:"remaining_percent,omitempty"`
	LifeTimeHours int    `json:"lifetime_hours,omitempty"`
	Passed        bool   `json:"passed"`
	InProgress    bool   `json:"in_progress"`
	IsShort       bool   `json:"is_short"`
	IsLong        bool   `json:"is_long"`
}

type SelfTestView struct {
	InProgress    bool                `json:"in_progress"`
	PowerOnHours  int                 `json:"power_on_hours,omitempty"`
	LatestShort   *SelfTestEntryView  `json:"latest_short,omitempty"`
	LatestLong    *SelfTestEntryView  `json:"latest_long,omitempty"`
	RecentEntries []SelfTestEntryView `json:"recent_entries,omitempty"`
	ShortDue      bool                `json:"short_due"`
	LongDue       bool                `json:"long_due"`
	Lines         []string            `json:"lines,omitempty"`
	Healthy       bool                `json:"healthy"`
}

type DiskStatus struct {
	Device          string             `json:"device"`
	Healthy         bool               `json:"healthy"`
	ReadOK          bool               `json:"read_ok"`
	HealthStatus    string             `json:"health_status"`
	Issues          []string           `json:"issues,omitempty"`
	Serial          string             `json:"serial,omitempty"`
	Manufacturer    string             `json:"manufacturer,omitempty"`
	Model           string             `json:"model,omitempty"`
	Capacity        string             `json:"capacity,omitempty"`
	TemperatureC    *int               `json:"temperature_c,omitempty"`
	MarginalWarning bool               `json:"marginal_warning"`
	DeviceErrors    int                `json:"device_errors"`
	NVMeWarnings    []string           `json:"nvme_warnings,omitempty"`
	Monitored       []MonitoredCounter `json:"monitored"`
	Attributes      []AttributeView    `json:"attributes,omitempty"`
	SelfTest        *SelfTestView      `json:"self_test,omitempty"`
	Summary         string             `json:"summary"`
}

type SelfTestSchedule struct {
	ShortInterval time.Duration
	LongInterval  time.Duration
	MinGap        time.Duration
}

func InspectDevice(device string, opts CheckOptions) DiskStatus {
	output, err := readDeviceOutput(device)
	text := string(output)

	if err != nil {
		summary := fmt.Sprintf("%s: SMART read failed — %s", device, smartctlErrorMessage(output, err))
		return DiskStatus{
			Device:  device,
			Healthy: false,
			ReadOK:  false,
			Issues:  []string{summary},
			Summary: summary,
		}
	}

	healthy, healthStatus := parseHealth(text)
	signals := parseCriticalSignals(text)
	identity := parseDeviceIdentity(text, signals.Attributes)
	prev := loadPreviousState(opts.StateDir, device, signals.Serial)
	issues := evaluateCriticalSignals(signals, opts, prev)

	if len(issues) > 0 {
		healthy = false
	}

	status := buildDiskStatus(device, healthy, healthStatus, issues, signals, identity, prev, opts)
	persistDeviceState(opts.StateDir, device, signals)
	return status
}

func (s DiskStatus) Result() Result {
	return Result{
		Device:  s.Device,
		Healthy: s.Healthy,
		ReadOK:  s.ReadOK,
		Summary: s.Summary,
	}
}

func EnrichDiskStatus(status DiskStatus, schedule SelfTestSchedule) DiskStatus {
	if !status.ReadOK {
		return status
	}

	device := NormalizeDevice(status.Device)

	log, err := ReadSelfTestLog(device)
	if err != nil {
		status.Healthy = false
		status.Issues = append(status.Issues, err.Error())
		status.Summary += "\n" + err.Error()
		return status
	}

	if info, err := ReadDeviceInfo(device); err == nil {
		log.PowerOnHours = info.PowerOnHours
		if info.InProgress {
			log.InProgress = true
		}
	} else {
		status.Summary += "\n" + err.Error()
	}

	view := buildSelfTestView(log, schedule)
	selfTestHealthy := view.Healthy

	status.SelfTest = &view
	if !selfTestHealthy {
		status.Healthy = false
		status.Issues = append(status.Issues, view.Lines...)
		if len(view.Lines) > 0 {
			status.Summary += "\n" + strings.Join(view.Lines, "\n")
		}
	}

	return status
}

func buildDiskStatus(
	device string,
	healthy bool,
	healthStatus string,
	issues []string,
	signals CriticalSignals,
	identity DeviceIdentity,
	prev *DeviceState,
	opts CheckOptions,
) DiskStatus {
	summary := fmt.Sprintf("%s: %s", device, healthStatus)
	if len(issues) > 0 {
		summary += "\n" + strings.Join(issues, "\n")
	}

	return DiskStatus{
		Device:          device,
		Healthy:         healthy,
		ReadOK:          true,
		HealthStatus:    healthStatus,
		Issues:          append([]string(nil), issues...),
		Serial:          signals.Serial,
		Manufacturer:    identity.Manufacturer,
		Model:           identity.Model,
		Capacity:        identity.Capacity,
		TemperatureC:    identity.TemperatureC,
		MarginalWarning: signals.MarginalWarning,
		DeviceErrors:    signals.DeviceErrors,
		NVMeWarnings:    append([]string(nil), signals.NVMeWarnings...),
		Monitored:       buildMonitoredCounters(signals, prev, opts),
		Attributes:      buildAttributeViews(signals.Attributes),
		Summary:         summary,
	}
}

func buildMonitoredCounters(signals CriticalSignals, prev *DeviceState, opts CheckOptions) []MonitoredCounter {
	counters := signals.counters()
	thresholds := map[string]int{
		counterReallocated:   opts.ReallocatedThreshold,
		counterUncorrectable: opts.UncorrectableThreshold,
		counterPending:       opts.PendingThreshold,
		counterOffline:       opts.OfflineThreshold,
		counterDeviceErrors:  opts.ErrorLogThreshold,
	}

	var monitored []MonitoredCounter
	for _, name := range monitoredCounterNames {
		value, ok := counters[name]
		if !ok {
			continue
		}

		entry := MonitoredCounter{
			Name:      name,
			Value:     value,
			Threshold: thresholds[name],
		}

		if prev != nil {
			if oldVal, ok := prev.Counters[name]; ok {
				oldCopy := oldVal
				entry.Previous = &oldCopy
				entry.Increased = value > oldVal
			}
		}

		if entry.Threshold > 0 && value >= entry.Threshold {
			entry.Alert = true
		}

		monitored = append(monitored, entry)
	}

	return monitored
}

func buildAttributeViews(attrs map[string]AttributeReading) []AttributeView {
	views := make([]AttributeView, 0, len(attrs))
	for _, attr := range attrs {
		views = append(views, AttributeView{
			ID:       attr.ID,
			Name:     attr.Name,
			Raw:      attr.Raw,
			RawText:  attr.RawText,
			FailFlag: attr.FailFlag,
		})
	}

	sort.Slice(views, func(i, j int) bool {
		if views[i].ID != views[j].ID {
			return views[i].ID < views[j].ID
		}
		return views[i].Name < views[j].Name
	})

	return views
}

func buildSelfTestView(log SelfTestLog, schedule SelfTestSchedule) SelfTestView {
	selfTestHealthy, lines := log.HealthSummary()

	view := SelfTestView{
		InProgress:   log.InProgress,
		PowerOnHours: log.PowerOnHours,
		Lines:        lines,
		Healthy:      selfTestHealthy,
		ShortDue:     log.ShortDue(schedule.ShortInterval, schedule.MinGap),
		LongDue:      log.LongDue(schedule.LongInterval, schedule.MinGap),
	}

	if log.LatestShort != nil {
		entry := selfTestEntryView(*log.LatestShort)
		view.LatestShort = &entry
	}
	if log.LatestLong != nil {
		entry := selfTestEntryView(*log.LatestLong)
		view.LatestLong = &entry
	}

	limit := 8
	if len(log.Entries) < limit {
		limit = len(log.Entries)
	}
	for i := 0; i < limit; i++ {
		view.RecentEntries = append(view.RecentEntries, selfTestEntryView(log.Entries[i]))
	}

	return view
}

func selfTestEntryView(entry SelfTestEntry) SelfTestEntryView {
	return SelfTestEntryView{
		Num:           entry.Num,
		Description:   entry.Description,
		Status:        entry.Status,
		Remaining:     entry.Remaining,
		LifeTimeHours: entry.LifeTimeHours,
		Passed:        entry.Passed,
		InProgress:    entry.InProgress,
		IsShort:       entry.IsShort,
		IsLong:        entry.IsLong,
	}
}
