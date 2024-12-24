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
	"strings"
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
	resourceSpec := testbed.ResourceSpec{
		ExpectedMaxCPU: 30,
		ExpectedMaxRAM: 120,
	}
	processors := []ProcessorNameAndConfigBody{}
	tests := getBatcherTestSpecs(resourceSpec, processors)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runBatcherPerfTest(t, test)
		})
	}
}

func TestLog10kDPSWithProcessors(t *testing.T) {
	resourceSpec := testbed.ResourceSpec{
		ExpectedMaxCPU: 30,
		ExpectedMaxRAM: 120,
	}
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
	tests := getBatcherTestSpecs(resourceSpec, processors)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runBatcherPerfTest(t, test)
		})
	}
}

func TestLog10kDPSWithHeavyProcessing(t *testing.T) {
	ottlStatementCount := 50
	transformProcessor := ProcessorNameAndConfigBody{
		Name: "transform",
		Body: `
  transform:
    log_statements:
      - context: log
        statements:
`,
	}
	for i := 0; i < ottlStatementCount; i++ {
		transformProcessor.Body += strings.Repeat(" ", ottlStatementCount) + "- set(attributes[\"counter\"], ExtractPatterns(body, \"Load Generator Counter (?P<counter>.+)\"))\n"
	}
	processors := []ProcessorNameAndConfigBody{transformProcessor}
	resourceSpec := testbed.ResourceSpec{
		ExpectedMaxCPU: 120,
		ExpectedMaxRAM: 120,
	}
	tests := getBatcherTestSpecs(resourceSpec, processors)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runBatcherPerfTest(t, test)
		})
	}

}

func TestLog10kDPSWith20Processors(t *testing.T) {
	processorCount := 20
	initialProcessors := []ProcessorNameAndConfigBody{
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
	processors := make([]ProcessorNameAndConfigBody, 0, processorCount)
	for i := 0; i < processorCount/len(initialProcessors); i++ {
		for _, processor := range initialProcessors {
			processorCopy := processor
			processorCopy.Name = fmt.Sprintf("%s/%d", processor.Name, i)
			processorCopy.Body = strings.ReplaceAll(processorCopy.Body, processor.Name, processorCopy.Name)
			processors = append(processors, processorCopy)
		}
	}
	resourceSpec := testbed.ResourceSpec{
		ExpectedMaxCPU: 50,
		ExpectedMaxRAM: 120,
	}
	tests := getBatcherTestSpecs(resourceSpec, processors)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runBatcherPerfTest(t, test)
		})
	}
}

func getBatcherTestSpecs(resourceSpec testbed.ResourceSpec, processors []ProcessorNameAndConfigBody) []batcherTestSpec {
	testSpecs := []batcherTestSpec{
		{
			name:         "No batching, no queue",
			resourceSpec: resourceSpec,
			processors:   processors,
		},
		{
			name:         "No batching, queue",
			withQueue:    true,
			resourceSpec: resourceSpec,
			processors:   processors,
		},
		{
			name:               "Batch size 1000 with batch processor, no queue",
			batchSize:          1000,
			withBatchProcessor: true,
			resourceSpec:       resourceSpec,
			processors:         processors,
		},
		{
			name:               "Batch size 1000 with batch processor, queue",
			batchSize:          1000,
			withBatchProcessor: true,
			withQueue:          true,
			resourceSpec:       resourceSpec,
			processors:         processors,
		},
		{
			name:                "Batch size 1000 with exporter batcher, no queue",
			withExporterBatcher: true,
			batchSize:           1000,
			resourceSpec:        resourceSpec,
			processors:          processors,
		},
		{
			name:                "Batch size 1000 with exporter batcher, queue",
			withExporterBatcher: true,
			withQueue:           true,
			batchSize:           1000,
			resourceSpec:        resourceSpec,
			processors:          processors,
		},
	}
	return testSpecs
}

func runBatcherPerfTest(t *testing.T, spec batcherTestSpec) {
	t.Helper()
	sender := testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
	receiver := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))
	receiver.WithRetry(`
    retry_on_failure:
      enabled: true
`)
	if spec.withQueue {
		receiver.WithQueue(`
    sending_queue:
      enabled: true
`)
	}

	if spec.withExporterBatcher {
		receiver.WithBatcher(fmt.Sprintf(`
    batcher:
      enabled: true
      min_size_items: %d
`, spec.batchSize))
	}

	processors := slices.Clone(spec.processors)
	if spec.withBatchProcessor {
		processors = slices.Insert(processors, 0, ProcessorNameAndConfigBody{
			Name: "batch",
			Body: fmt.Sprintf(`
  batch:
    send_batch_size: %d
`, spec.batchSize),
		})
	}
	loadOptions := &testbed.LoadOptions{
		Parallel:      10,
		ItemsPerBatch: 10,
	}
	Scenario10kItemsPerSecond(t, sender, receiver, spec.resourceSpec, performanceResultsSummary, processors, spec.extensions, loadOptions)
}
