// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// These load tests exercise OTTL through the transform processor so that its CPU and
// memory usage for traces, metrics, and logs is published to the load test benchmark
// dashboard. The statements mutate telemetry but never drop items, so the sent and
// received counters still match.

func TestTransformProcessorTraces(t *testing.T) {
	sender := testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
	receiver := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))

	processors := []ProcessorNameAndConfigBody{
		{
			Name: "transform",
			Body: `
  transform:
    error_mode: ignore
    trace_statements:
      - set(resource.attributes["transform.env"], "benchmark")
      - set(span.attributes["transform.processed"], "true")
      - set(span.attributes["transform.seq_num"], span.attributes["load_generator.span_seq_num"]) where span.attributes["load_generator.span_seq_num"] != nil
      - replace_pattern(span.name, "load-generator-span", "span-")
      - limit(span.attributes, 100, [])
      - truncate_all(span.attributes, 4096)
`,
		},
	}

	Scenario10kItemsPerSecond(
		t,
		sender,
		receiver,
		testbed.ResourceSpec{ExpectedMaxCPU: 200, ExpectedMaxRAM: 200},
		performanceResultsSummary,
		processors,
		nil,
		nil,
	)
}

func TestTransformProcessorMetrics(t *testing.T) {
	sender := testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
	receiver := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))

	processors := []ProcessorNameAndConfigBody{
		{
			Name: "transform",
			Body: `
  transform:
    error_mode: ignore
    metric_statements:
      - set(resource.attributes["transform.env"], "benchmark")
      - set(metric.description, "benchmark metric")
      - set(datapoint.attributes["transform.processed"], "true")
      - truncate_all(datapoint.attributes, 4096)
`,
		},
	}

	Scenario10kItemsPerSecond(
		t,
		sender,
		receiver,
		testbed.ResourceSpec{ExpectedMaxCPU: 200, ExpectedMaxRAM: 200},
		performanceResultsSummary,
		processors,
		nil,
		nil,
	)
}

func TestTransformProcessorLogs(t *testing.T) {
	sender := testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
	receiver := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))

	processors := []ProcessorNameAndConfigBody{
		{
			Name: "transform",
			Body: `
  transform:
    error_mode: ignore
    log_statements:
      - set(resource.attributes["transform.env"], "benchmark")
      - set(log.attributes["transform.processed"], "true")
      - set(log.severity_text, "WARN") where log.attributes["c"] == 3
      - replace_pattern(log.body, "Counter", "Ctr")
      - truncate_all(log.attributes, 4096)
`,
		},
	}

	Scenario10kItemsPerSecond(
		t,
		sender,
		receiver,
		testbed.ResourceSpec{ExpectedMaxCPU: 200, ExpectedMaxRAM: 200},
		performanceResultsSummary,
		processors,
		nil,
		nil,
	)
}
