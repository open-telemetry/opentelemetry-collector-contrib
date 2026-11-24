// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"

const (
	// MetricMetadataTypeKey is the key used to store the original Prometheus
	// type in metric metadata:
	// https://github.com/open-telemetry/opentelemetry-specification/blob/e6eccba97ebaffbbfad6d4358408a2cead0ec2df/specification/compatibility/prometheus_and_openmetrics.md#metric-metadata
	MetricMetadataTypeKey = "prometheus.type"
	// ExemplarTraceIDKey is the key used to store the trace ID in Prometheus
	// exemplars:
	// https://github.com/open-telemetry/opentelemetry-specification/blob/e6eccba97ebaffbbfad6d4358408a2cead0ec2df/specification/compatibility/prometheus_and_openmetrics.md#exemplars
	ExemplarTraceIDKey = "trace_id"
	// ExemplarSpanIDKey is the key used to store the Span ID in Prometheus
	// exemplars:
	// https://github.com/open-telemetry/opentelemetry-specification/blob/e6eccba97ebaffbbfad6d4358408a2cead0ec2df/specification/compatibility/prometheus_and_openmetrics.md#exemplars
	ExemplarSpanIDKey = "span_id"
	// ScopeInfoMetricName is the name of the metric used to preserve scope
	// attributes in Prometheus format:
	// https://github.com/open-telemetry/opentelemetry-specification/blob/e6eccba97ebaffbbfad6d4358408a2cead0ec2df/specification/compatibility/prometheus_and_openmetrics.md#instrumentation-scope
	ScopeInfoMetricName = "otel_scope_info"
	// ScopeLabelPrefix prefixes instrumentation scope labels in Prometheus
	// format:
	// https://github.com/open-telemetry/opentelemetry-specification/blob/fd43145dde7e5192ebc59a20992d98a3e6af5553/specification/compatibility/prometheus_and_openmetrics.md#instrumentation-scope
	ScopeLabelPrefix = "otel_scope_"
	// ScopeNameLabelKey is the name of the label key used to identify the name
	// of the OpenTelemetry scope which produced the metric:
	// https://github.com/open-telemetry/opentelemetry-specification/blob/e6eccba97ebaffbbfad6d4358408a2cead0ec2df/specification/compatibility/prometheus_and_openmetrics.md#instrumentation-scope
	ScopeNameLabelKey = "otel_scope_name"
	// ScopeVersionLabelKey is the name of the label key used to identify the
	// version of the OpenTelemetry scope which produced the metric:
	// https://github.com/open-telemetry/opentelemetry-specification/blob/e6eccba97ebaffbbfad6d4358408a2cead0ec2df/specification/compatibility/prometheus_and_openmetrics.md#instrumentation-scope
	ScopeVersionLabelKey = "otel_scope_version"
	// ScopeSchemaURLLabelKey is the name of the label key used to identify the
	// schema URL of the OpenTelemetry scope which produced the metric:
	// https://github.com/open-telemetry/opentelemetry-specification/blob/e6eccba97ebaffbbfad6d4358408a2cead0ec2df/specification/compatibility/prometheus_and_openmetrics.md#instrumentation-scope
	ScopeSchemaURLLabelKey = "otel_scope_schema_url"
	// TargetInfoMetricName is the name of the metric used to preserve resource
	// attributes in Prometheus format:
	// https://github.com/open-telemetry/opentelemetry-specification/blob/e6eccba97ebaffbbfad6d4358408a2cead0ec2df/specification/compatibility/prometheus_and_openmetrics.md#resource-attributes-1
	// It originates from OpenMetrics:
	// https://github.com/OpenObservability/OpenMetrics/blob/1386544931307dff279688f332890c31b6c5de36/specification/OpenMetrics.md#supporting-target-metadata-in-both-push-based-and-pull-based-systems
	TargetInfoMetricName = "target_info"
	// EntityInfoMetricName is the name of the metric used to preserve OpenTelemetry
	// entity metadata in Prometheus format:
	// https://github.com/open-telemetry/opentelemetry-specification/pull/5462
	EntityInfoMetricName = "otel_entity_info"
	// EntityTypeLabelKey is the label key for the entity's type on otel_entity_info.
	EntityTypeLabelKey = "otel_entity_type"
	// EntitySchemaURLLabelKey is the label key for the entity's schema URL on otel_entity_info.
	EntitySchemaURLLabelKey = "otel_entity_schema_url"
	// EntityIDKeysLabelKey is the label key for the JSON array of identifying attribute keys on otel_entity_info.
	EntityIDKeysLabelKey = "otel_entity_id_keys"
	// EntityDescriptionKeysLabelKey is the label key for the JSON array of descriptive attribute keys on otel_entity_info.
	EntityDescriptionKeysLabelKey = "otel_entity_description_keys"
)
