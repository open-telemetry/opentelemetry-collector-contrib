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
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
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
			sender:   datasenders.NewCarbonDataSender(testutil.GetAvailablePort(t)),
			receiver: datareceivers.NewCarbonDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 237,
				ExpectedMaxRAM: 105,
			},
		},
		{
			name:     "OpenCensus",
			sender:   datasenders.NewOCMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver: datareceivers.NewOCDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 85,
				ExpectedMaxRAM: 100,
			},
		},
		{
			name:     "OTLP",
			sender:   testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 60,
				ExpectedMaxRAM: 105,
			},
		},
		{
			name:     "OTLP-HTTP",
			sender:   testbed.NewOTLPHTTPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver: testbed.NewOTLPHTTPDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 60,
				ExpectedMaxRAM: 100,
			},
		},
		{
			name:     "SignalFx",
			sender:   datasenders.NewSFxMetricDataSender(testutil.GetAvailablePort(t)),
			receiver: datareceivers.NewSFxMetricsDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 120,
				ExpectedMaxRAM: 98,
			},
		},
		{
			name:     "STEF",
			sender:   datasenders.NewStefDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver: datareceivers.NewStefDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 60,
				ExpectedMaxRAM: 100,
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

	dataProvider, err := testbed.NewFileDataProvider("testdata/k8s-metrics.yaml", pipeline.SignalMetrics)
	assert.NoError(t, err)

	options := testbed.LoadOptions{
		DataItemsPerSecond: 1_000,
		Parallel:           1,
		// ItemsPerBatch is based on the data from the file.
		ItemsPerBatch: dataProvider.ItemsPerBatch,
	}
	agentProc := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))

	sender := testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
	receiver := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))

	configStr := createConfigYaml(t, sender, receiver, resultDir, nil, nil)
	configCleanup, err := agentProc.PrepareConfig(t, configStr)
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
