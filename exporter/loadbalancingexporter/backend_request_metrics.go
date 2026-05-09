// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

const (
	backendRequestSignalLogs    = "logs"
	backendRequestSignalMetrics = "metrics"
)

func backendRequestAttributeSet(signal, endpoint string) attribute.Set {
	return attribute.NewSet(attribute.String("endpoint", endpoint), attribute.String("signal", signal))
}

func backendRequestSignalAttributeSet(signal string) attribute.Set {
	return attribute.NewSet(attribute.String("signal", signal))
}

func backendRequestMetricOptions(attrs attribute.Set) metric.MeasurementOption {
	return metric.WithAttributeSet(attrs)
}

func recordLogBackendRequest(ctx context.Context, tb *metadata.TelemetryBuilder, signalAttrs, endpointAttrs attribute.Set, ld plog.Logs) {
	if tb == nil {
		return
	}

	signalOpts := backendRequestMetricOptions(signalAttrs)
	tb.LoadbalancerBackendRequestBytes.Record(ctx, serializedLogsSize(ld), signalOpts)
	tb.LoadbalancerBackendRequestItems.Record(ctx, int64(ld.LogRecordCount()), signalOpts)
	tb.LoadbalancerBackendRequestTotal.Add(ctx, 1, backendRequestMetricOptions(endpointAttrs))
}

func recordMetricBackendRequest(ctx context.Context, tb *metadata.TelemetryBuilder, signalAttrs, endpointAttrs attribute.Set, md pmetric.Metrics) {
	if tb == nil {
		return
	}

	signalOpts := backendRequestMetricOptions(signalAttrs)
	tb.LoadbalancerBackendRequestBytes.Record(ctx, serializedMetricsSize(md), signalOpts)
	tb.LoadbalancerBackendRequestItems.Record(ctx, int64(md.DataPointCount()), signalOpts)
	tb.LoadbalancerBackendRequestTotal.Add(ctx, 1, backendRequestMetricOptions(endpointAttrs))
}

func serializedLogsSize(ld plog.Logs) int64 {
	marshaler := plog.ProtoMarshaler{}
	return int64(marshaler.LogsSize(ld))
}

func serializedMetricsSize(md pmetric.Metrics) int64 {
	marshaler := pmetric.ProtoMarshaler{}
	return int64(marshaler.MetricsSize(md))
}
