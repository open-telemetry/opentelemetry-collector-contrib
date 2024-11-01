// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	scenarios "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/tests"
)

func TestStabilityMetricsOTLP(t *testing.T) {
	scenarios.Scenario10kItemsPerSecond(
		t,
		testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
		testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
		testbed.ResourceSpec{
			ExpectedMaxCPU:      50,
			ExpectedMaxRAM:      80,
			ResourceCheckPeriod: resourceCheckPeriod,
		},
		contribPerfResultsSummary,
		nil,
		nil,
		nil,
	)
}

func TestStabilityMetricsOpenCensus(t *testing.T) {
	scenarios.Scenario10kItemsPerSecond(
		t,
		datasenders.NewOCMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
		datareceivers.NewOCDataReceiver(testutil.GetAvailablePort(t)),
		testbed.ResourceSpec{
			ExpectedMaxCPU:      85,
			ExpectedMaxRAM:      86,
			ResourceCheckPeriod: resourceCheckPeriod,
		},
		contribPerfResultsSummary,
		nil,
		nil,
		nil,
	)
}

func TestStabilityMetricsCarbon(t *testing.T) {
	scenarios.Scenario10kItemsPerSecond(
		t,
		datasenders.NewCarbonDataSender(testutil.GetAvailablePort(t)),
		datareceivers.NewCarbonDataReceiver(testutil.GetAvailablePort(t)),
		testbed.ResourceSpec{
			ExpectedMaxCPU:      237,
			ExpectedMaxRAM:      120,
			ResourceCheckPeriod: resourceCheckPeriod,
		},
		contribPerfResultsSummary,
		nil,
		nil,
		nil,
	)
}

func TestStabilityMetricsSignalFx(t *testing.T) {
	scenarios.Scenario10kItemsPerSecond(
		t,
		datasenders.NewSFxMetricDataSender(testutil.GetAvailablePort(t)),
		datareceivers.NewSFxMetricsDataReceiver(testutil.GetAvailablePort(t)),
		testbed.ResourceSpec{
			ExpectedMaxCPU:      120,
			ExpectedMaxRAM:      95,
			ResourceCheckPeriod: resourceCheckPeriod,
		},
		contribPerfResultsSummary,
		nil,
		nil,
		nil,
	)
}
