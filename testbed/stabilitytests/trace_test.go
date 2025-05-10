// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package stabilitytests contains long-running test cases verifying that otel-collector can run
// sustainably for long time, 1 hour by default.
// Tests supposed to be run on CircleCI, each tests must be allocated to exactly one runner
// to make sure that the whole test suit will not take longer than one hour.
// Because of that, every time overall number of stability tests changed,
// make sure to update CircleCI parameter: run-stability-tests.runners-number

package tests

import (
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	scenarios "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/tests"
)

var (
	contribPerfResultsSummary = &testbed.PerformanceResults{}
	resourceCheckPeriod, _    = time.ParseDuration("1m")
	processorsConfig          = []scenarios.ProcessorNameAndConfigBody{
		{
			Name: "batch",
			Body: `
  batch:
`,
		},
	}
)

// TestMain is used to initiate setup, execution and tear down of testbed.
func TestMain(m *testing.M) {
	testbed.DoTestMain(m, contribPerfResultsSummary)
}

func TestStabilityTracesOpenCensus(t *testing.T) {
	scenarios.Scenario10kItemsPerSecond(
		t,
		datasenders.NewOCTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
		datareceivers.NewOCDataReceiver(testutil.GetAvailablePort(t)),
		testbed.ResourceSpec{
			ExpectedMaxCPU:      39,
			ExpectedMaxRAM:      90,
			ResourceCheckPeriod: resourceCheckPeriod,
		},
		contribPerfResultsSummary,
		processorsConfig,
		nil,
	)
}

func TestStabilityTracesSAPM(t *testing.T) {
	scenarios.Scenario10kItemsPerSecond(
		t,
		datasenders.NewSapmDataSender(testutil.GetAvailablePort(t), ""),
		datareceivers.NewSapmDataReceiver(testutil.GetAvailablePort(t), ""),
		testbed.ResourceSpec{
			ExpectedMaxCPU:      40,
			ExpectedMaxRAM:      100,
			ResourceCheckPeriod: resourceCheckPeriod,
		},
		contribPerfResultsSummary,
		processorsConfig,
		nil,
	)
}

func TestStabilityTracesOTLP(t *testing.T) {
	scenarios.Scenario10kItemsPerSecond(
		t,
		testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
		testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
		testbed.ResourceSpec{
			ExpectedMaxCPU:      20,
			ExpectedMaxRAM:      80,
			ResourceCheckPeriod: resourceCheckPeriod,
		},
		contribPerfResultsSummary,
		processorsConfig,
		nil,
	)
}

func TestStabilityTracesJaegerGRPC(t *testing.T) {
	scenarios.Scenario10kItemsPerSecond(
		t,
		datasenders.NewJaegerGRPCDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
		datareceivers.NewJaegerDataReceiver(testutil.GetAvailablePort(t)),
		testbed.ResourceSpec{
			ExpectedMaxCPU:      40,
			ExpectedMaxRAM:      90,
			ResourceCheckPeriod: resourceCheckPeriod,
		},
		contribPerfResultsSummary,
		processorsConfig,
		nil,
	)
}

func TestStabilityTracesZipkin(t *testing.T) {
	scenarios.Scenario10kItemsPerSecond(
		t,
		datasenders.NewZipkinDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
		datareceivers.NewZipkinDataReceiver(testutil.GetAvailablePort(t)),
		testbed.ResourceSpec{
			ExpectedMaxCPU:      80,
			ExpectedMaxRAM:      110,
			ResourceCheckPeriod: resourceCheckPeriod,
		},
		contribPerfResultsSummary,
		processorsConfig,
		nil,
	)
}

func TestStabilityTracesDatadog(t *testing.T) {
	scenarios.Scenario10kItemsPerSecond(
		t,
		datasenders.NewDatadogDataSender(),
		datareceivers.NewDataDogDataReceiver(),
		testbed.ResourceSpec{
			ExpectedMaxCPU:      80,
			ExpectedMaxRAM:      110,
			ResourceCheckPeriod: resourceCheckPeriod,
		},
		contribPerfResultsSummary,
		processorsConfig,
		nil,
	)
}
