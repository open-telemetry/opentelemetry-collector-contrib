// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers/signalfxdatareceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders/signalfxdatasender"
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

func TestStabilityMetricsSignalFx(t *testing.T) {
	scenarios.Scenario10kItemsPerSecond(
		t,
		signalfxdatasender.NewSFxMetricDataSender(testutil.GetAvailablePort(t)),
		signalfxdatareceiver.NewSFxMetricsDataReceiver(testutil.GetAvailablePort(t)),
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
