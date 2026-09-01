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
