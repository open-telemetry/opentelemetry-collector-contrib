// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tests

// This file contains Test functions which initiate the tests. The tests can be either
// coded in this file or use scenarios from perf_scenarios.go.

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

func TestMetric10kDPS(t *testing.T) {
	tests := []struct {
		name         string
		sender       testbed.DataSender
		receiver     testbed.DataReceiver
		resourceSpec testbed.ResourceSpec
		skipMessage  string
	}{
		{
			name:     "Carbon",
			sender:   datasenders.NewCarbonDataSender(testbed.GetAvailablePort(t)),
			receiver: datareceivers.NewCarbonDataReceiver(testbed.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 237,
				ExpectedMaxRAM: 100,
			},
			skipMessage: "Flaky test, https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9729",
		},
		{
			name:     "OpenCensus",
			sender:   datasenders.NewOCMetricDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t)),
			receiver: datareceivers.NewOCDataReceiver(testbed.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 85,
				ExpectedMaxRAM: 100,
			},
		},
		{
			name:     "OTLP",
			sender:   testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t)),
			receiver: testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 60,
				ExpectedMaxRAM: 105,
			},
		},
		{
			name:     "OTLP-HTTP",
			sender:   testbed.NewOTLPHTTPMetricDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t)),
			receiver: testbed.NewOTLPHTTPDataReceiver(testbed.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 60,
				ExpectedMaxRAM: 100,
			},
		},
		{
			name:     "SignalFx",
			sender:   datasenders.NewSFxMetricDataSender(testbed.GetAvailablePort(t)),
			receiver: datareceivers.NewSFxMetricsDataReceiver(testbed.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 120,
				ExpectedMaxRAM: 98,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.skipMessage != "" {
				t.Skip(test.skipMessage)
			}
			Scenario10kItemsPerSecond(
				t,
				test.sender,
				test.receiver,
				test.resourceSpec,
				performanceResultsSummary,
				nil,
				nil,
			)
		})
	}

}

func TestMetricsFromFile(t *testing.T) {
	// This test demonstrates usage of NewFileDataProvider to generate load using
	// previously recorded data.

	resultDir, err := filepath.Abs(filepath.Join("results", t.Name()))
	require.NoError(t, err)

	dataProvider, err := testbed.NewFileDataProvider("testdata/k8s-metrics.yaml", component.DataTypeMetrics)
	assert.NoError(t, err)

	options := testbed.LoadOptions{
		DataItemsPerSecond: 1_000,
		Parallel:           1,
		// ItemsPerBatch is based on the data from the file.
		ItemsPerBatch: dataProvider.ItemsPerBatch,
	}
	agentProc := testbed.NewChildProcessCollector()

	sender := testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t))
	receiver := testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t))

	configStr := createConfigYaml(t, sender, receiver, resultDir, nil, nil)
	configCleanup, err := agentProc.PrepareConfig(configStr)
	require.NoError(t, err)
	defer configCleanup()

	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		agentProc,
		&testbed.PerfTestValidator{},
		performanceResultsSummary,
		testbed.WithResourceLimits(testbed.ResourceSpec{ExpectedMaxCPU: 120, ExpectedMaxRAM: 110}),
	)
	defer tc.Stop()

	tc.StartBackend()
	tc.StartAgent()

	tc.StartLoad(options)

	tc.Sleep(tc.Duration)

	tc.StopLoad()

	tc.WaitFor(func() bool { return tc.LoadGenerator.DataItemsSent() > 0 }, "load generator started")
	tc.WaitFor(func() bool { return tc.LoadGenerator.DataItemsSent() == tc.MockBackend.DataItemsReceived() },
		"all data items received")

	tc.StopAgent()

	tc.ValidateData()
}
