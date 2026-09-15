// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package k8sattributesprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const scanTestService = "scan-test-service"

func TestScanTracesForAttributesSkipsIncompleteResource(t *testing.T) {
	sink := new(consumertest.TracesSink)
	require.NoError(t, sink.ConsumeTraces(context.Background(), tracesWithResource(true)))
	require.NoError(t, sink.ConsumeTraces(context.Background(), tracesWithResource(false)))

	scanTracesForAttributes(t, sink, scanTestService, scanTestAttributes())
}

func TestScanMetricsForAttributesSkipsIncompleteResource(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	require.NoError(t, sink.ConsumeMetrics(context.Background(), metricsWithResource(true)))
	require.NoError(t, sink.ConsumeMetrics(context.Background(), metricsWithResource(false)))

	scanMetricsForAttributes(t, sink, scanTestService, scanTestAttributes())
}

func TestScanLogsForAttributesSkipsIncompleteResource(t *testing.T) {
	sink := new(consumertest.LogsSink)
	require.NoError(t, sink.ConsumeLogs(context.Background(), logsWithResource(true)))
	require.NoError(t, sink.ConsumeLogs(context.Background(), logsWithResource(false)))

	scanLogsForAttributes(t, sink, scanTestService, scanTestAttributes())
}

func scanTestAttributes() map[string]*expectedValue {
	return map[string]*expectedValue{
		"k8s.pod.name": newExpectedValue(exist, ""),
	}
}

func tracesWithResource(complete bool) ptrace.Traces {
	traces := ptrace.NewTraces()
	resource := traces.ResourceSpans().AppendEmpty().Resource()
	resource.Attributes().PutStr("service.name", scanTestService)
	if complete {
		resource.Attributes().PutStr("k8s.pod.name", "pod")
	}
	return traces
}

func metricsWithResource(complete bool) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	resource := metrics.ResourceMetrics().AppendEmpty().Resource()
	resource.Attributes().PutStr("service.name", scanTestService)
	if complete {
		resource.Attributes().PutStr("k8s.pod.name", "pod")
	}
	return metrics
}

func logsWithResource(complete bool) plog.Logs {
	logs := plog.NewLogs()
	resource := logs.ResourceLogs().AppendEmpty().Resource()
	resource.Attributes().PutStr("service.name", scanTestService)
	if complete {
		resource.Attributes().PutStr("k8s.pod.name", "pod")
	}
	return logs
}
