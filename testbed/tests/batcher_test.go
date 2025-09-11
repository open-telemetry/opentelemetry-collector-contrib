// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package tests contains test cases. To run the tests go to tests directory and run:
// RUN_TESTBED=1 go test -v

//go:build batcher
// +build batcher

package tests

// The tests in this file measure the effect of batching on collector performance.
// Their primary intent is to measure the performance impact of https://github.com/open-telemetry/opentelemetry-collector/issues/8122.

import (
	"fmt"
	"slices"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

type batcherTestSpec struct {
	name                string
	withQueue           bool
	withBatchProcessor  bool
	withExporterBatcher bool
	batchSize           int
	processors          []ProcessorNameAndConfigBody
	resourceSpec        testbed.ResourceSpec
	extensions          map[string]string
}

func TestLog10kDPSNoProcessors(t *testing.T) {
	tests := []batcherTestSpec{
		{
			name: "No batching, no queue",
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 30,
				ExpectedMaxRAM: 120,
			},
		},
		{
			name:      "No batching, queue",
			withQueue: true,
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 30,
				ExpectedMaxRAM: 120,
			},
		},
		{
			name:               "Batch size 1000 with batch processor, no queue",
			batchSize:          1000,
			withBatchProcessor: true,
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 30,
				ExpectedMaxRAM: 120,
			},
		},
		{
			name:               "Batch size 1000 with batch processor, queue",
			batchSize:          1000,
			withBatchProcessor: true,
			withQueue:          true,
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 30,
				ExpectedMaxRAM: 120,
			},
		},
		{
			name:                "Batch size 1000 with exporter batcher, no queue",
			withExporterBatcher: true,
			batchSize:           1000,
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 30,
				ExpectedMaxRAM: 120,
			},
		},
		{
			name:                "Batch size 1000 with exporter batcher, queue",
			withExporterBatcher: true,
			withQueue:           true,
			batchSize:           1000,
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 30,
				ExpectedMaxRAM: 120,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sender := testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
			receiver := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))
			receiver.WithRetry(`
    retry_on_failure:
      enabled: true
`)
			if test.withQueue {
				receiver.WithQueue(`
    sending_queue:
      enabled: true
`)
			}

			if test.withExporterBatcher {
				receiver.WithBatcher(fmt.Sprintf(`
    batcher:
      enabled: true
      min_size: %d
`, test.batchSize))
			}

			processors := slices.Clone(test.processors)
			if test.withBatchProcessor {
				processors = slices.Insert(processors, 0, ProcessorNameAndConfigBody{
					Name: "batch",
					Body: fmt.Sprintf(`
  batch:
    send_batch_size: %d
`, test.batchSize),
				})
			}
			loadOptions := &testbed.LoadOptions{
				Parallel:      10,
				ItemsPerBatch: 10,
			}
			Scenario10kItemsPerSecond(t, sender, receiver, test.resourceSpec, performanceResultsSummary, processors, test.extensions, loadOptions)
		})
	}
}

func TestLog10kDPSWithProcessors(t *testing.T) {
	processors := []ProcessorNameAndConfigBody{
		{
			Name: "filter",
			Body: `
  filter:
    logs:
      log_record:
        - not IsMatch(attributes["batch_index"], "batch_.+")
`,
		},
		{
			Name: "transform",
			Body: `
  transform:
    log_statements:
      - context: log
        statements:
          - set(resource.attributes["batch_index"], attributes["batch_index"])
          - set(attributes["counter"], ExtractPatterns(body, "Load Generator Counter (?P<counter>.+)"))
`,
		},
	}
	tests := []batcherTestSpec{
		{
			name:       "No batching, no queue",
			processors: processors,
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 30,
				ExpectedMaxRAM: 120,
			},
		},
		{
			name:       "No batching, queue",
			processors: processors,
			withQueue:  true,
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 30,
				ExpectedMaxRAM: 120,
			},
		},
		{
			name:               "Batch size 1000 with batch processor, no queue",
			processors:         processors,
			batchSize:          1000,
			withBatchProcessor: true,
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 30,
				ExpectedMaxRAM: 120,
			},
		},
		{
			name:               "Batch size 1000 with batch processor, queue",
			processors:         processors,
			batchSize:          1000,
			withBatchProcessor: true,
			withQueue:          true,
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 30,
				ExpectedMaxRAM: 120,
			},
		},
		{
			name:                "Batch size 1000 with exporter batcher, no queue",
			processors:          processors,
			withExporterBatcher: true,
			batchSize:           1000,
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 30,
				ExpectedMaxRAM: 120,
			},
		},
		{
			name:                "Batch size 1000 with exporter batcher, queue",
			processors:          processors,
			withExporterBatcher: true,
			withQueue:           true,
			batchSize:           1000,
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 30,
				ExpectedMaxRAM: 120,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sender := testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
			receiver := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))
			receiver.WithRetry(`
    retry_on_failure:
      enabled: true
`)
			if test.withQueue {
				receiver.WithQueue(`
    sending_queue:
      enabled: true
      queue_size: 10
`)
			}

			if test.withExporterBatcher {
				receiver.WithBatcher(fmt.Sprintf(`
    batcher:
      enabled: true
      min_size: %d
`, test.batchSize))
			}

			testProcessors := slices.Clone(test.processors)
			if test.withBatchProcessor {
				processors = slices.Insert(testProcessors, 0, ProcessorNameAndConfigBody{
					Name: "batch",
					Body: fmt.Sprintf(`
  batch:
    send_batch_size: %d
`, test.batchSize),
				})
			}
			loadOptions := &testbed.LoadOptions{
				Parallel:      10,
				ItemsPerBatch: 10,
			}
			Scenario10kItemsPerSecond(t, sender, receiver, test.resourceSpec, performanceResultsSummary, testProcessors, test.extensions, loadOptions)
		})
	}
}
