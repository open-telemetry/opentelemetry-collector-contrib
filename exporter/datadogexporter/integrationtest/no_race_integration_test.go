// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !race

package integrationtest // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/integrationtest"

import (
	"context"
	"encoding/json"
	"runtime"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/testutil"
	"github.com/stretchr/testify/assert"

	commonTestutil "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestIntegrationInternalMetrics(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("flaky test on windows https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/34836")
	}
	// 1. Set up mock Datadog server
	seriesRec := &testutil.HTTPRequestRecorderWithChan{Pattern: testutil.MetricV2Endpoint, ReqChan: make(chan []byte, 100)}
	tracesRec := &testutil.HTTPRequestRecorderWithChan{Pattern: testutil.TraceEndpoint, ReqChan: make(chan []byte, 100)}
	server := testutil.DatadogServerMock(seriesRec.HandlerFunc, tracesRec.HandlerFunc)
	defer server.Close()
	t.Setenv("SERVER_URL", server.URL)
	t.Setenv("PROM_SERVER", commonTestutil.GetAvailableLocalAddress(t))
	t.Setenv("OTLP_HTTP_SERVER", commonTestutil.GetAvailableLocalAddress(t))
	otlpGRPCEndpoint := commonTestutil.GetAvailableLocalAddress(t)
	t.Setenv("OTLP_GRPC_SERVER", otlpGRPCEndpoint)

	// 2. Start in-process collector
	factories := getIntegrationTestComponents(t)
	app := getIntegrationTestCollector(t, "integration_test_internal_metrics_config.yaml", factories)
	go func() {
		assert.NoError(t, app.Run(context.Background()))
	}()
	defer app.Shutdown()

	waitForReadiness(app)

	// 3. Generate and send traces
	sendTraces(t, otlpGRPCEndpoint)

	// 4. Validate Datadog trace agent & OTel internal metrics are sent to the mock server
	expectedMetrics := map[string]struct{}{
		// Datadog internal metrics on trace and stats writers
		"otelcol_datadog_otlp_translator_resources_missing_source": {},
		"otelcol_datadog_trace_agent_stats_writer_bytes":           {},
		"otelcol_datadog_trace_agent_stats_writer_retries":         {},
		"otelcol_datadog_trace_agent_stats_writer_stats_buckets":   {},
		"otelcol_datadog_trace_agent_stats_writer_stats_entries":   {},
		"otelcol_datadog_trace_agent_stats_writer_payloads":        {},
		"otelcol_datadog_trace_agent_stats_writer_client_payloads": {},
		"otelcol_datadog_trace_agent_stats_writer_errors":          {},
		"otelcol_datadog_trace_agent_stats_writer_splits":          {},
		"otelcol_datadog_trace_agent_trace_writer_bytes":           {},
		"otelcol_datadog_trace_agent_trace_writer_retries":         {},
		"otelcol_datadog_trace_agent_trace_writer_spans":           {},
		"otelcol_datadog_trace_agent_trace_writer_traces":          {},
		"otelcol_datadog_trace_agent_trace_writer_payloads":        {},
		"otelcol_datadog_trace_agent_trace_writer_errors":          {},
		"otelcol_datadog_trace_agent_trace_writer_events":          {},

		// OTel collector internal metrics
		"otelcol_process_memory_rss":                     {},
		"otelcol_process_runtime_total_sys_memory_bytes": {},
		"otelcol_process_uptime":                         {},
		"otelcol_process_cpu_seconds":                    {},
		"otelcol_process_runtime_heap_alloc_bytes":       {},
		"otelcol_process_runtime_total_alloc_bytes":      {},
		"otelcol_receiver_accepted_metric_points":        {},
		"otelcol_receiver_accepted_spans":                {},
		"otelcol_exporter_queue_capacity":                {},
		"otelcol_exporter_queue_size":                    {},
		"otelcol_exporter_sent_spans":                    {},
		"otelcol_exporter_sent_metric_points":            {},
	}

	metricMap := make(map[string]series)
	for len(metricMap) < len(expectedMetrics) {
		select {
		case <-tracesRec.ReqChan:
			// Drain the channel, no need to look into the traces
		case metricsBytes := <-seriesRec.ReqChan:
			var metrics seriesSlice
			gz := getGzipReader(t, metricsBytes)
			dec := json.NewDecoder(gz)
			assert.NoError(t, dec.Decode(&metrics))
			for _, s := range metrics.Series {
				if _, ok := expectedMetrics[s.Metric]; ok {
					metricMap[s.Metric] = s
				}
			}
		case <-time.After(60 * time.Second):
			t.Fail()
		}
	}
}
