// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	scenarios "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/tests"
)

func TestStabilityMetricsOTLP(t *testing.T) {
	scenarios.Scenario10kItemsPerSecond(
		t,
		testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t)),
		testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t)),
		testbed.ResourceSpec{
			ExpectedMaxCPU:      50,
			ExpectedMaxRAM:      80,
			ResourceCheckPeriod: resourceCheckPeriod,
		},
		contribPerfResultsSummary,
		nil,
		nil,
	)
}

func TestStabilityMetricsOpenCensus(t *testing.T) {
	scenarios.Scenario10kItemsPerSecond(
		t,
		datasenders.NewOCMetricDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t)),
		datareceivers.NewOCDataReceiver(testbed.GetAvailablePort(t)),
		testbed.ResourceSpec{
			ExpectedMaxCPU:      85,
			ExpectedMaxRAM:      86,
			ResourceCheckPeriod: resourceCheckPeriod,
		},
		contribPerfResultsSummary,
		nil,
		nil,
	)
}

func TestStabilityMetricsCarbon(t *testing.T) {
	scenarios.Scenario10kItemsPerSecond(
		t,
		datasenders.NewCarbonDataSender(testbed.GetAvailablePort(t)),
		datareceivers.NewCarbonDataReceiver(testbed.GetAvailablePort(t)),
		testbed.ResourceSpec{
			ExpectedMaxCPU:      237,
			ExpectedMaxRAM:      120,
			ResourceCheckPeriod: resourceCheckPeriod,
		},
		contribPerfResultsSummary,
		nil,
		nil,
	)
}

func TestStabilityMetricsSignalFx(t *testing.T) {
	scenarios.Scenario10kItemsPerSecond(
		t,
		datasenders.NewSFxMetricDataSender(testbed.GetAvailablePort(t)),
		datareceivers.NewSFxMetricsDataReceiver(testbed.GetAvailablePort(t)),
		testbed.ResourceSpec{
			ExpectedMaxCPU:      120,
			ExpectedMaxRAM:      95,
			ResourceCheckPeriod: resourceCheckPeriod,
		},
		contribPerfResultsSummary,
		nil,
		nil,
	)
}
