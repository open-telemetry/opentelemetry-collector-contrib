// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package tests contains test cases. To run the tests go to tests directory and run:
// RUN_TESTBED=1 go test -v

package tests

import (
	"context"
	"path"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

func TestLog10kDPS(t *testing.T) {
	tests := []struct {
		name         string
		sender       testbed.DataSender
		receiver     testbed.DataReceiver
		resourceSpec testbed.ResourceSpec
		extensions   map[string]string
	}{
		{
			name:     "OTLP",
			sender:   testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 30,
				ExpectedMaxRAM: 120,
			},
		},
		{
			name:     "OTLP-HTTP",
			sender:   testbed.NewOTLPHTTPLogsDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver: testbed.NewOTLPHTTPDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 30,
				ExpectedMaxRAM: 120,
			},
		},
		{
			name:     "filelog",
			sender:   datasenders.NewFileLogWriter(t),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 50,
				ExpectedMaxRAM: 120,
			},
		},
		{
			name:     "filelog checkpoints",
			sender:   datasenders.NewFileLogWriter(t),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 50,
				ExpectedMaxRAM: 120,
			},
			extensions: datasenders.NewLocalFileStorageExtension(t),
		},
		{
			name:     "kubernetes containers",
			sender:   datasenders.NewKubernetesContainerWriter(),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 110,
				ExpectedMaxRAM: 150,
			},
		},
		{
			name:     "kubernetes containers parser",
			sender:   datasenders.NewKubernetesContainerParserWriter(),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 110,
				ExpectedMaxRAM: 150,
			},
		},
		{
			name:     "k8s CRI-Containerd",
			sender:   datasenders.NewKubernetesCRIContainerdWriter(),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 100,
				ExpectedMaxRAM: 150,
			},
		},
		{
			name:     "k8s CRI-Containerd no attr ops",
			sender:   datasenders.NewKubernetesCRIContainerdNoAttributesOpsWriter(),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 100,
				ExpectedMaxRAM: 150,
			},
		},
		{
			name:     "CRI-Containerd",
			sender:   datasenders.NewCRIContainerdWriter(),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 100,
				ExpectedMaxRAM: 150,
			},
		},
		{
			name:     "syslog-tcp-batch-1",
			sender:   datasenders.NewTCPUDPWriter("tcp", testbed.DefaultHost, testutil.GetAvailablePort(t), 1),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 80,
				ExpectedMaxRAM: 150,
			},
		},
		{
			name:     "syslog-tcp-batch-100",
			sender:   datasenders.NewTCPUDPWriter("tcp", testbed.DefaultHost, testutil.GetAvailablePort(t), 100),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 80,
				ExpectedMaxRAM: 150,
			},
		},
		{
			name:     "FluentForward-SplunkHEC",
			sender:   datasenders.NewFluentLogsForwarder(t, testutil.GetAvailablePort(t)),
			receiver: datareceivers.NewSplunkHECDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 60,
				ExpectedMaxRAM: 150,
			},
		},
		{
			name:     "tcp-batch-1",
			sender:   datasenders.NewTCPUDPWriter("tcp", testbed.DefaultHost, testutil.GetAvailablePort(t), 1),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 80,
				ExpectedMaxRAM: 150,
			},
		},
		{
			name:     "tcp-batch-100",
			sender:   datasenders.NewTCPUDPWriter("tcp", testbed.DefaultHost, testutil.GetAvailablePort(t), 100),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 80,
				ExpectedMaxRAM: 150,
			},
		},
	}

	processors := []ProcessorNameAndConfigBody{
		{
			Name: "batch",
			Body: `
  batch:
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			Scenario10kItemsPerSecond(
				t,
				test.sender,
				test.receiver,
				test.resourceSpec,
				performanceResultsSummary,
				processors,
				test.extensions,
				nil,
			)
		})
	}
}

func TestLogOtlpSendingQueue(t *testing.T) {
	otlpreceiver10 := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))
	otlpreceiver10.WithRetry(`
    retry_on_failure:
      enabled: true
`)
	otlpreceiver10.WithQueue(`
    sending_queue:
      enabled: true
      queue_size: 10
`)
	t.Run("OTLP-sending-queue-full", func(t *testing.T) {
		ScenarioSendingQueuesFull(
			t,
			testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			otlpreceiver10,
			testbed.LoadOptions{
				DataItemsPerSecond: 100,
				ItemsPerBatch:      10,
				Parallel:           1,
			},
			testbed.ResourceSpec{
				ExpectedMaxCPU: 80,
				ExpectedMaxRAM: 120,
			}, 10,
			performanceResultsSummary,
			nil,
			nil)
	})

	otlpreceiver100 := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))
	otlpreceiver100.WithRetry(`
    retry_on_failure:
      enabled: true
`)
	otlpreceiver10.WithQueue(`
    sending_queue:
      enabled: true
      queue_size: 100
`)
	t.Run("OTLP-sending-queue-not-full", func(t *testing.T) {
		ScenarioSendingQueuesNotFull(
			t,
			testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			otlpreceiver100,
			testbed.LoadOptions{
				DataItemsPerSecond: 100,
				ItemsPerBatch:      10,
				Parallel:           1,
			},
			testbed.ResourceSpec{
				ExpectedMaxCPU: 80,
				ExpectedMaxRAM: 120,
			}, 10,
			performanceResultsSummary,
			nil,
			nil)
	})
}

func TestLogLargeFiles(t *testing.T) {
	tests := []struct {
		name         string
		sender       testbed.DataSender
		receiver     testbed.DataReceiver
		loadOptions  testbed.LoadOptions
		resourceSpec testbed.ResourceSpec
		sleepSeconds int
	}{
		{
			/*
			 * The FileLogWriter generates strings almost 100 bytes each.
			 * With a rate of 200,000 lines per second over a duration of 100 seconds,
			 * this results in a file size of approximately 2GB over its lifetime.
			 */
			name:     "filelog-largefiles-2Gb-lifetime",
			sender:   datasenders.NewFileLogWriter(t),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			loadOptions: testbed.LoadOptions{
				DataItemsPerSecond: 200000,
				ItemsPerBatch:      1,
				Parallel:           100,
			},
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 80,
				ExpectedMaxRAM: 150,
			},
			sleepSeconds: 100,
		},
		{
			/*
			 * The FileLogWriter generates strings almost 100 bytes each.
			 * With a rate of 330,000 lines per second over a duration of 200 seconds,
			 * this results in a file size of approximately 6GB over its lifetime.
			 */
			name:     "filelog-largefiles-6GB-lifetime",
			sender:   datasenders.NewFileLogWriter(t),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			loadOptions: testbed.LoadOptions{
				DataItemsPerSecond: 330000,
				ItemsPerBatch:      10,
				Parallel:           10,
			},
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 100,
				ExpectedMaxRAM: 150,
			},
			sleepSeconds: 200,
		},
	}
	processors := []ProcessorNameAndConfigBody{
		{
			Name: "batch",
			Body: `
  batch:
`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ScenarioLong(
				t,
				test.sender,
				test.receiver,
				test.loadOptions,
				performanceResultsSummary,
				test.sleepSeconds,
				processors,
			)
		})
	}
}

func TestLargeFileOnce(t *testing.T) {
	processors := []ProcessorNameAndConfigBody{
		{
			Name: "batch",
			Body: `
  batch:
`,
		},
	}
	resultDir, err := filepath.Abs(path.Join("results", t.Name()))
	require.NoError(t, err)
	sender := datasenders.NewFileLogWriter(t)
	receiver := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))
	loadOptions := testbed.LoadOptions{
		DataItemsPerSecond: 1,
		ItemsPerBatch:      10000000,
		Parallel:           1,
	}

	// Write data at once, before starting up the collector
	dataProvider := testbed.NewPerfTestDataProvider(loadOptions)
	dataItemsGenerated := atomic.Uint64{}
	dataProvider.SetLoadGeneratorCounters(&dataItemsGenerated)
	ld, _ := dataProvider.GenerateLogs()

	require.NoError(t, sender.ConsumeLogs(context.Background(), ld))
	agentProc := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))

	configStr := createConfigYaml(t, sender, receiver, resultDir, processors, nil)
	configCleanup, err := agentProc.PrepareConfig(t, configStr)
	require.NoError(t, err)
	defer configCleanup()

	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		agentProc,
		&testbed.CorrectnessLogTestValidator{},
		performanceResultsSummary,
	)
	t.Cleanup(tc.Stop)

	tc.StartBackend()
	tc.StartAgent()

	tc.WaitForN(func() bool { return dataItemsGenerated.Load() == tc.MockBackend.DataItemsReceived() }, 200*time.Second, "all logs received")

	tc.StopAgent()
	tc.ValidateData()
}

func TestMemoryLimiterHit(t *testing.T) {
	tests := []struct {
		name   string
		sender testbed.DataSender
	}{
		{
			name:   "otlp",
			sender: testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
		},
		{
			name: "filelog",
			sender: datasenders.NewFileLogWriter(t).WithRetry(`
    retry_on_failure:
      enabled: true
`),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			otlpreceiver := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))
			otlpreceiver.WithRetry(`
    retry_on_failure:
      enabled: true
      max_interval: 5s
`)
			otlpreceiver.WithQueue(`
    sending_queue:
       enabled: true
       queue_size: 100000
       num_consumers: 20
`)
			otlpreceiver.WithTimeout(`
    timeout: 0s
`)
			processors := []ProcessorNameAndConfigBody{
				{
					Name: "memory_limiter",
					Body: `
  memory_limiter:
    check_interval: 1s
    limit_mib: 300
    spike_limit_mib: 150
`,
				},
			}
			ScenarioMemoryLimiterHit(
				t,
				test.sender,
				otlpreceiver,
				testbed.LoadOptions{
					DataItemsPerSecond: 100000,
					ItemsPerBatch:      1000,
					Parallel:           1,
					MaxDelay:           20 * time.Second,
				},
				performanceResultsSummary, 100, processors)
		})
	}
}
