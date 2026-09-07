// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// These load tests exercise OTTL through the filter processor so that its CPU and
// memory usage for traces, metrics, and logs is published to the load test benchmark
// dashboard. The conditions are evaluated for every item but are written so that they
// never match the generated telemetry, so nothing is dropped and the sent and received
// counters still match.

func TestFilterProcessorTraces(t *testing.T) {
	sender := testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
	receiver := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))

	processors := []ProcessorNameAndConfigBody{
		{
			Name: "filter",
			Body: `
  filter:
    error_mode: ignore
    traces:
      span:
        - span.name == "nonexistent-span-name"
        - span.attributes["load_generator.span_seq_num"] < 0
        - span.attributes["nonexistent.attr"] == "value"
        - IsMatch(span.name, "^this-will-never-match-[0-9]+$")
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

func TestFilterProcessorMetrics(t *testing.T) {
	sender := testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
	receiver := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))

	processors := []ProcessorNameAndConfigBody{
		{
			Name: "filter",
			Body: `
  filter:
    error_mode: ignore
    metrics:
      metric:
        - metric.name == "nonexistent_metric"
        - IsMatch(metric.name, "^never_match_[0-9]+$")
      datapoint:
        - datapoint.attributes["item_index"] == "item_does_not_exist"
        - datapoint.value_int < 0
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

func TestFilterProcessorLogs(t *testing.T) {
	sender := testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
	receiver := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))

	processors := []ProcessorNameAndConfigBody{
		{
			Name: "filter",
			Body: `
  filter:
    error_mode: ignore
    logs:
      log_record:
        - log.body == "this message never appears"
        - log.attributes["c"] > 100
        - log.severity_number == 17
        - IsMatch(log.body, "^never-[0-9]+$")
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
