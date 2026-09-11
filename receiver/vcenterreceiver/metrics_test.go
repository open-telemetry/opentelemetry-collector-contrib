// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

func TestRecordVMStats_IncompleteDataDoesNotPanic(t *testing.T) {
	scraper := &vcenterMetricScraper{
		mb: metadata.NewMetricsBuilder(metadata.NewDefaultMetricsBuilderConfig(), receivertest.NewNopSettings(metadata.Type)),
	}
	ts := pcommon.NewTimestampFromTime(time.Now())

	validVM := &mo.VirtualMachine{
		Config: &types.VirtualMachineConfigInfo{},
		Summary: types.VirtualMachineSummary{
			Storage: &types.VirtualMachineStorageSummary{},
		},
	}
	validHost := &mo.HostSystem{
		Summary: types.HostListSummary{
			Hardware: &types.HostHardwareSummary{},
		},
	}

	testCases := []struct {
		name string
		vm   *mo.VirtualMachine
		hs   *mo.HostSystem
	}{
		{
			name: "nil vm config",
			vm: &mo.VirtualMachine{
				Summary: types.VirtualMachineSummary{
					Storage: &types.VirtualMachineStorageSummary{},
				},
			},
			hs: validHost,
		},
		{
			name: "nil vm storage summary",
			vm: &mo.VirtualMachine{
				Config: &types.VirtualMachineConfigInfo{},
			},
			hs: validHost,
		},
		{
			name: "nil host",
			vm:   validVM,
			hs:   nil,
		},
		{
			name: "nil host summary hardware",
			vm:   validVM,
			hs:   &mo.HostSystem{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.NotPanics(t, func() {
				scraper.recordVMStats(ts, tc.vm, tc.hs)
			})
		})
	}
}

func TestBuildVMMetrics_IncompleteVMSkipsWithoutError(t *testing.T) {
	scraper := &vcenterMetricScraper{
		scrapeData: &vcenterScrapeData{
			computesByRef: map[string]*mo.ComputeResource{
				"cr-1": {},
			},
		},
	}

	vm := &mo.VirtualMachine{
		Runtime: types.VirtualMachineRuntimeInfo{PowerState: types.VirtualMachinePowerStatePoweredOff},
	}
	vmRefToComputeRef := map[string]*types.ManagedObjectReference{
		"": {Type: "ComputeResource", Value: "cr-1"},
	}

	crRef, groupInfo, err := scraper.buildVMMetrics(
		pcommon.NewTimestampFromTime(time.Now()),
		&mo.Datacenter{},
		vm,
		vmRefToComputeRef,
	)

	require.NoError(t, err)
	require.NotNil(t, crRef)
	require.NotNil(t, groupInfo)
	require.Equal(t, int64(1), groupInfo.poweredOff)
	require.Equal(t, int64(0), groupInfo.poweredOn)
	require.Equal(t, int64(0), groupInfo.templates)
}

func TestRecordVMStats_CPUMetricsFollowPowerStateNotUsage(t *testing.T) {
	// An idle VM reports 0 MHz of CPU usage. That is a valid measurement, not a
	// sign that the VM is unavailable, so it must not suppress the CPU metrics.
	// Availability is decided by the power state instead.
	testCases := []struct {
		name        string
		powerState  types.VirtualMachinePowerState
		cpuUsage    int32
		wantMetrics map[string]float64
	}{
		{
			name:       "idle powered on vm still reports cpu metrics",
			powerState: types.VirtualMachinePowerStatePoweredOn,
			cpuUsage:   0,
			wantMetrics: map[string]float64{
				"vcenter.vm.cpu.usage":       0,
				"vcenter.vm.cpu.utilization": 0,
				"vcenter.vm.cpu.readiness":   3,
			},
		},
		{
			name:       "busy powered on vm reports cpu metrics",
			powerState: types.VirtualMachinePowerStatePoweredOn,
			cpuUsage:   500,
			wantMetrics: map[string]float64{
				"vcenter.vm.cpu.usage":       500,
				"vcenter.vm.cpu.utilization": 25,
				"vcenter.vm.cpu.readiness":   3,
			},
		},
		{
			name:        "powered off vm reports no cpu metrics",
			powerState:  types.VirtualMachinePowerStatePoweredOff,
			cpuUsage:    0,
			wantMetrics: map[string]float64{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scraper := &vcenterMetricScraper{
				mb: metadata.NewMetricsBuilder(metadata.NewDefaultMetricsBuilderConfig(), receivertest.NewNopSettings(metadata.Type)),
			}
			vm := &mo.VirtualMachine{
				Config: &types.VirtualMachineConfigInfo{
					Hardware: types.VirtualHardware{NumCPU: 2},
				},
				Runtime: types.VirtualMachineRuntimeInfo{PowerState: tc.powerState},
				Summary: types.VirtualMachineSummary{
					Storage: &types.VirtualMachineStorageSummary{},
					QuickStats: types.VirtualMachineQuickStats{
						OverallCpuUsage:     tc.cpuUsage,
						OverallCpuReadiness: 3,
					},
				},
			}
			host := &mo.HostSystem{
				Summary: types.HostListSummary{
					Hardware: &types.HostHardwareSummary{CpuMhz: 1000},
				},
			}

			scraper.recordVMStats(pcommon.NewTimestampFromTime(time.Now()), vm, host)

			got := cpuMetricValues(scraper.mb.Emit())
			require.Equal(t, tc.wantMetrics, got)
		})
	}
}

// cpuMetricValues returns the first data point of every vcenter.vm.cpu.* metric,
// keyed by metric name.
func cpuMetricValues(metrics pmetric.Metrics) map[string]float64 {
	values := map[string]float64{}
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				if !strings.HasPrefix(m.Name(), "vcenter.vm.cpu.") {
					continue
				}
				var dps pmetric.NumberDataPointSlice
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					dps = m.Gauge().DataPoints()
				case pmetric.MetricTypeSum:
					dps = m.Sum().DataPoints()
				default:
					continue
				}
				if dps.Len() == 0 {
					continue
				}
				dp := dps.At(0)
				if dp.ValueType() == pmetric.NumberDataPointValueTypeInt {
					values[m.Name()] = float64(dp.IntValue())
				} else {
					values[m.Name()] = dp.DoubleValue()
				}
			}
		}
	}
	return values
}

func TestRecordHostSystemStats_StateMetrics(t *testing.T) {
	testCases := []struct {
		name                string
		powerState          types.HostSystemPowerState
		connectionState     types.HostSystemConnectionState
		wantPowerState      map[string]int64
		wantConnectionState map[string]int64
	}{
		{
			name:                "powered on and connected host",
			powerState:          types.HostSystemPowerStatePoweredOn,
			connectionState:     types.HostSystemConnectionStateConnected,
			wantPowerState:      map[string]int64{"power_state=on": 1},
			wantConnectionState: map[string]int64{"connection_state=connected": 1},
		},
		{
			name:                "powered off and not responding host",
			powerState:          types.HostSystemPowerStatePoweredOff,
			connectionState:     types.HostSystemConnectionStateNotResponding,
			wantPowerState:      map[string]int64{"power_state=off": 1},
			wantConnectionState: map[string]int64{"connection_state=not_responding": 1},
		},
		{
			name:                "unrecognized power state is reported as unknown",
			powerState:          "",
			connectionState:     types.HostSystemConnectionStateDisconnected,
			wantPowerState:      map[string]int64{"power_state=unknown": 1},
			wantConnectionState: map[string]int64{"connection_state=disconnected": 1},
		},
		{
			name:                "unrecognized connection state is not reported",
			powerState:          types.HostSystemPowerStatePoweredOn,
			connectionState:     "",
			wantPowerState:      map[string]int64{"power_state=on": 1},
			wantConnectionState: map[string]int64{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scraper := newStateMetricsScraper()
			host := newTestHost()
			host.Runtime.PowerState = tc.powerState
			host.Runtime.ConnectionState = tc.connectionState

			scraper.recordHostSystemStats(pcommon.NewTimestampFromTime(time.Now()), host)

			metrics := scraper.mb.Emit()
			require.Equal(t, tc.wantPowerState, metricDataPoints(metrics, "vcenter.host.power_state"))
			require.Equal(t, tc.wantConnectionState, metricDataPoints(metrics, "vcenter.host.connection_state"))
		})
	}
}

func TestRecordHostSystemStats_Uptime(t *testing.T) {
	now := time.Now()
	bootTime := now.Add(-40 * 24 * time.Hour)

	testCases := []struct {
		name       string
		powerState types.HostSystemPowerState
		bootTime   *time.Time
		wantUptime map[string]int64
	}{
		{
			name:       "powered on host reports seconds since boot",
			powerState: types.HostSystemPowerStatePoweredOn,
			bootTime:   &bootTime,
			wantUptime: map[string]int64{"": 40 * 24 * 60 * 60},
		},
		{
			name:       "powered off host reports no uptime",
			powerState: types.HostSystemPowerStatePoweredOff,
			bootTime:   &bootTime,
			wantUptime: map[string]int64{},
		},
		{
			name:       "host without boot time reports no uptime",
			powerState: types.HostSystemPowerStatePoweredOn,
			bootTime:   nil,
			wantUptime: map[string]int64{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scraper := newStateMetricsScraper()
			host := newTestHost()
			host.Runtime.PowerState = tc.powerState
			host.Runtime.BootTime = tc.bootTime

			scraper.recordHostSystemStats(pcommon.NewTimestampFromTime(now), host)

			require.Equal(t, tc.wantUptime, metricDataPoints(scraper.mb.Emit(), "vcenter.host.uptime"))
		})
	}
}

func TestRecordHostSystemStats_AlarmCount(t *testing.T) {
	scraper := newStateMetricsScraper()
	host := newTestHost()
	host.TriggeredAlarmState = []types.AlarmState{
		{OverallStatus: types.ManagedEntityStatusRed},
		{OverallStatus: types.ManagedEntityStatusRed},
		{OverallStatus: types.ManagedEntityStatusYellow},
	}

	scraper.recordHostSystemStats(pcommon.NewTimestampFromTime(time.Now()), host)

	require.Equal(t,
		map[string]int64{"status=red": 2, "status=yellow": 1},
		metricDataPoints(scraper.mb.Emit(), "vcenter.host.alarm.count"),
	)
}

func TestCountTriggeredAlarmsByStatus(t *testing.T) {
	testCases := []struct {
		name       string
		alarms     []types.AlarmState
		wantRed    int64
		wantYellow int64
	}{
		{
			name:       "no triggered alarms",
			alarms:     nil,
			wantRed:    0,
			wantYellow: 0,
		},
		{
			name: "only red and yellow alarms are counted",
			alarms: []types.AlarmState{
				{OverallStatus: types.ManagedEntityStatusRed},
				{OverallStatus: types.ManagedEntityStatusYellow},
				{OverallStatus: types.ManagedEntityStatusRed},
				{OverallStatus: types.ManagedEntityStatusGreen},
				{OverallStatus: types.ManagedEntityStatusGray},
			},
			wantRed:    2,
			wantYellow: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			red, yellow := countTriggeredAlarmsByStatus(tc.alarms)
			require.Equal(t, tc.wantRed, red)
			require.Equal(t, tc.wantYellow, yellow)
		})
	}
}

func TestRecordDatastoreStats_StateMetrics(t *testing.T) {
	testCases := []struct {
		name                string
		maintenanceMode     string
		alarms              []types.AlarmState
		wantMaintenanceMode map[string]int64
		wantAlarmCount      map[string]int64
	}{
		{
			name:                "datastore in normal mode with a red alarm",
			maintenanceMode:     string(types.DatastoreSummaryMaintenanceModeStateNormal),
			alarms:              []types.AlarmState{{OverallStatus: types.ManagedEntityStatusRed}},
			wantMaintenanceMode: map[string]int64{"maintenance_mode=normal": 1},
			wantAlarmCount:      map[string]int64{"status=red": 1, "status=yellow": 0},
		},
		{
			name:                "datastore entering maintenance",
			maintenanceMode:     string(types.DatastoreSummaryMaintenanceModeStateEnteringMaintenance),
			wantMaintenanceMode: map[string]int64{"maintenance_mode=entering_maintenance": 1},
			wantAlarmCount:      map[string]int64{"status=red": 0, "status=yellow": 0},
		},
		{
			name:                "datastore in maintenance",
			maintenanceMode:     string(types.DatastoreSummaryMaintenanceModeStateInMaintenance),
			wantMaintenanceMode: map[string]int64{"maintenance_mode=in_maintenance": 1},
			wantAlarmCount:      map[string]int64{"status=red": 0, "status=yellow": 0},
		},
		{
			name:                "datastore without maintenance mode reports no maintenance mode",
			maintenanceMode:     "",
			wantMaintenanceMode: map[string]int64{},
			wantAlarmCount:      map[string]int64{"status=red": 0, "status=yellow": 0},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scraper := newStateMetricsScraper()
			ds := &mo.Datastore{
				Summary: types.DatastoreSummary{
					Capacity:        100,
					FreeSpace:       40,
					MaintenanceMode: tc.maintenanceMode,
				},
			}
			ds.TriggeredAlarmState = tc.alarms

			scraper.recordDatastoreStats(pcommon.NewTimestampFromTime(time.Now()), ds)

			metrics := scraper.mb.Emit()
			require.Equal(t, tc.wantMaintenanceMode, metricDataPoints(metrics, "vcenter.datastore.maintenance_mode"))
			require.Equal(t, tc.wantAlarmCount, metricDataPoints(metrics, "vcenter.datastore.alarm.count"))
		})
	}
}

func TestRecordVMStats_PowerState(t *testing.T) {
	validHost := &mo.HostSystem{
		Summary: types.HostListSummary{
			Hardware: &types.HostHardwareSummary{CpuMhz: 1000},
		},
	}

	testCases := []struct {
		name           string
		powerState     types.VirtualMachinePowerState
		template       bool
		host           *mo.HostSystem
		wantPowerState map[string]int64
	}{
		{
			name:           "powered on vm",
			powerState:     types.VirtualMachinePowerStatePoweredOn,
			host:           validHost,
			wantPowerState: map[string]int64{"power_state=on": 1},
		},
		{
			name:           "powered off vm",
			powerState:     types.VirtualMachinePowerStatePoweredOff,
			host:           validHost,
			wantPowerState: map[string]int64{"power_state=off": 1},
		},
		{
			name:           "suspended vm",
			powerState:     types.VirtualMachinePowerStateSuspended,
			host:           validHost,
			wantPowerState: map[string]int64{"power_state=suspended": 1},
		},
		{
			name:           "template reports no power state",
			powerState:     types.VirtualMachinePowerStatePoweredOff,
			template:       true,
			host:           validHost,
			wantPowerState: map[string]int64{},
		},
		{
			name:           "power state is reported even without host hardware",
			powerState:     types.VirtualMachinePowerStatePoweredOn,
			host:           &mo.HostSystem{},
			wantPowerState: map[string]int64{"power_state=on": 1},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scraper := newStateMetricsScraper()
			vm := &mo.VirtualMachine{
				Config:  &types.VirtualMachineConfigInfo{Template: tc.template},
				Runtime: types.VirtualMachineRuntimeInfo{PowerState: tc.powerState},
				Summary: types.VirtualMachineSummary{
					Storage: &types.VirtualMachineStorageSummary{},
				},
			}

			scraper.recordVMStats(pcommon.NewTimestampFromTime(time.Now()), vm, tc.host)

			require.Equal(t, tc.wantPowerState, metricDataPoints(scraper.mb.Emit(), "vcenter.vm.power_state"))
		})
	}
}

// newStateMetricsScraper returns a scraper with all host, datastore and VM state metrics enabled.
func newStateMetricsScraper() *vcenterMetricScraper {
	cfg := metadata.NewDefaultMetricsBuilderConfig()
	cfg.Metrics.VcenterHostPowerState.Enabled = true
	cfg.Metrics.VcenterHostConnectionState.Enabled = true
	cfg.Metrics.VcenterHostUptime.Enabled = true
	cfg.Metrics.VcenterHostAlarmCount.Enabled = true
	cfg.Metrics.VcenterDatastoreMaintenanceMode.Enabled = true
	cfg.Metrics.VcenterDatastoreAlarmCount.Enabled = true
	cfg.Metrics.VcenterVMPowerState.Enabled = true
	return &vcenterMetricScraper{
		mb: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
}

// newTestHost returns a host with the hardware summary that recordHostSystemStats requires.
func newTestHost() *mo.HostSystem {
	return &mo.HostSystem{
		Summary: types.HostListSummary{
			Hardware: &types.HostHardwareSummary{
				MemorySize:  1 << 30,
				NumCpuCores: 2,
				CpuMhz:      1000,
			},
		},
	}
}

// metricDataPoints returns the int values of every data point of the named metric,
// keyed by the data point's attributes formatted as "key=value".
func metricDataPoints(metrics pmetric.Metrics, name string) map[string]int64 {
	values := map[string]int64{}
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				if m.Name() != name {
					continue
				}
				var dps pmetric.NumberDataPointSlice
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					dps = m.Gauge().DataPoints()
				case pmetric.MetricTypeSum:
					dps = m.Sum().DataPoints()
				default:
					continue
				}
				for l := 0; l < dps.Len(); l++ {
					dp := dps.At(l)
					var attrs []string
					dp.Attributes().Range(func(key string, value pcommon.Value) bool {
						attrs = append(attrs, key+"="+value.AsString())
						return true
					})
					values[strings.Join(attrs, ",")] = dp.IntValue()
				}
			}
		}
	}
	return values
}
