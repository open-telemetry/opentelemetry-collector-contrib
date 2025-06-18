// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"

const (
	// metricMetadataTypeKey is the key used to store the original Prometheus
	// type in metric metadata:
	// https://github.com/open-telemetry/opentelemetry-specification/blob/e6eccba97ebaffbbfad6d4358408a2cead0ec2df/specification/compatibility/prometheus_and_openmetrics.md#metric-metadata
	metricMetadataTypeKey = "prometheus.type"
	// exemplarTraceIDKey is the key used to store the trace ID in Prometheus
	// exemplars:
	// https://github.com/open-telemetry/opentelemetry-specification/blob/e6eccba97ebaffbbfad6d4358408a2cead0ec2df/specification/compatibility/prometheus_and_openmetrics.md#exemplars
	exemplarTraceIDKey = "trace_id"
	// exemplarSpanIDKey is the key used to store the Span ID in Prometheus
	// exemplars:
	// https://github.com/open-telemetry/opentelemetry-specification/blob/e6eccba97ebaffbbfad6d4358408a2cead0ec2df/specification/compatibility/prometheus_and_openmetrics.md#exemplars
	exemplarSpanIDKey = "span_id"
	// scopeInfoMetricName is the name of the metric used to preserve scope
	// attributes in Prometheus format:
	// https://github.com/open-telemetry/opentelemetry-specification/blob/e6eccba97ebaffbbfad6d4358408a2cead0ec2df/specification/compatibility/prometheus_and_openmetrics.md#instrumentation-scope
	scopeInfoMetricName = "otel_scope_info"
	// scopeNameLabelKey is the name of the label key used to identify the name
	// of the OpenTelemetry scope which produced the metric:
	// https://github.com/open-telemetry/opentelemetry-specification/blob/e6eccba97ebaffbbfad6d4358408a2cead0ec2df/specification/compatibility/prometheus_and_openmetrics.md#instrumentation-scope
	scopeNameLabelKey = "otel_scope_name"
	// scopeVersionLabelKey is the name of the label key used to identify the
	// version of the OpenTelemetry scope which produced the metric:
	// https://github.com/open-telemetry/opentelemetry-specification/blob/e6eccba97ebaffbbfad6d4358408a2cead0ec2df/specification/compatibility/prometheus_and_openmetrics.md#instrumentation-scope
	scopeVersionLabelKey = "otel_scope_version"
	// targetInfoMetricName is the name of the metric used to preserve resource
	// attributes in Prometheus format:
	// https://github.com/open-telemetry/opentelemetry-specification/blob/e6eccba97ebaffbbfad6d4358408a2cead0ec2df/specification/compatibility/prometheus_and_openmetrics.md#resource-attributes-1
	// It originates from OpenMetrics:
	// https://github.com/OpenObservability/OpenMetrics/blob/1386544931307dff279688f332890c31b6c5de36/specification/OpenMetrics.md#supporting-target-metadata-in-both-push-based-and-pull-based-systems
	targetInfoMetricName = "target_info"
)
