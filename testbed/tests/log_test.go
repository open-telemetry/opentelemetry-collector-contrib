// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
