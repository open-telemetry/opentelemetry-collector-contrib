// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package tests contains test cases. To run the tests go to tests directory and run:
// RUN_TESTBED=1 go test -v

package tests

import (
	"testing"

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
			sender:   testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t)),
			receiver: testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 30,
				ExpectedMaxRAM: 120,
			},
		},
		{
			name:     "OTLP-HTTP",
			sender:   testbed.NewOTLPHTTPLogsDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t)),
			receiver: testbed.NewOTLPHTTPDataReceiver(testbed.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 30,
				ExpectedMaxRAM: 120,
			},
		},
		{
			name:     "filelog",
			sender:   datasenders.NewFileLogWriter(),
			receiver: testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 50,
				ExpectedMaxRAM: 120,
			},
		},
		{
			name:     "filelog checkpoints",
			sender:   datasenders.NewFileLogWriter(),
			receiver: testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 50,
				ExpectedMaxRAM: 120,
			},
			extensions: datasenders.NewLocalFileStorageExtension(),
		},
		{
			name:     "kubernetes containers",
			sender:   datasenders.NewKubernetesContainerWriter(),
			receiver: testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 110,
				ExpectedMaxRAM: 150,
			},
		},
		{
			name:     "k8s CRI-Containerd",
			sender:   datasenders.NewKubernetesCRIContainerdWriter(),
			receiver: testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 100,
				ExpectedMaxRAM: 150,
			},
		},
		{
			name:     "k8s CRI-Containerd no attr ops",
			sender:   datasenders.NewKubernetesCRIContainerdNoAttributesOpsWriter(),
			receiver: testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 100,
				ExpectedMaxRAM: 150,
			},
		},
		{
			name:     "CRI-Containerd",
			sender:   datasenders.NewCRIContainerdWriter(),
			receiver: testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 100,
				ExpectedMaxRAM: 150,
			},
		},
		{
			name:     "syslog-tcp-batch-1",
			sender:   datasenders.NewTCPUDPWriter("tcp", testbed.DefaultHost, testbed.GetAvailablePort(t), 1),
			receiver: testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 80,
				ExpectedMaxRAM: 150,
			},
		},
		{
			name:     "syslog-tcp-batch-100",
			sender:   datasenders.NewTCPUDPWriter("tcp", testbed.DefaultHost, testbed.GetAvailablePort(t), 100),
			receiver: testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 80,
				ExpectedMaxRAM: 150,
			},
		},
		{
			name:     "FluentForward-SplunkHEC",
			sender:   datasenders.NewFluentLogsForwarder(t, testbed.GetAvailablePort(t)),
			receiver: datareceivers.NewSplunkHECDataReceiver(testbed.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 60,
				ExpectedMaxRAM: 150,
			},
		},
		{
			name:     "tcp-batch-1",
			sender:   datasenders.NewTCPUDPWriter("tcp", testbed.DefaultHost, testbed.GetAvailablePort(t), 1),
			receiver: testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 80,
				ExpectedMaxRAM: 150,
			},
		},
		{
			name:     "tcp-batch-100",
			sender:   datasenders.NewTCPUDPWriter("tcp", testbed.DefaultHost, testbed.GetAvailablePort(t), 100),
			receiver: testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 80,
				ExpectedMaxRAM: 150,
			},
		},
	}

	processors := map[string]string{
		"batch": `
  batch:
`,
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
			)
		})
	}
}

func TestLogOtlpSendingQueue(t *testing.T) {
	otlpreceiver10 := testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t))
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
			testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t)),
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

	otlpreceiver100 := testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t))
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
			testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t)),
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
