// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

func TestRecordVMStats_IncompleteDataDoesNotPanic(t *testing.T) {
	scraper := &vcenterMetricScraper{
		mb: metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopSettings(metadata.Type)),
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
