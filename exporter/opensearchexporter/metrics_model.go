// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import "time"

// metricBucket is a bucket of a histogram or exponential histogram data point
// with materialized boundaries, following the SS4O metrics schema and the
// Data Prepper OTel v1 metrics schema.
// See:
//
//	https://github.com/opensearch-project/opensearch-catalog/tree/main/schema/observability/metrics
//	https://github.com/opensearch-project/data-prepper/blob/main/data-prepper-plugins/opensearch/src/main/resources/index-template/metrics-otel-v1-index-standard-template.json
type metricBucket struct {
	Count uint64  `json:"count"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
}

// metricQuantile is a single quantile of a summary data point.
type metricQuantile struct {
	Quantile float64 `json:"quantile"`
	Value    float64 `json:"value"`
}

// metricExemplar is an exemplar attached to a number or histogram data point.
type metricExemplar struct {
	Time       time.Time      `json:"time"`
	Value      *float64       `json:"value,omitempty"`
	TraceID    string         `json:"traceId,omitempty"`
	SpanID     string         `json:"spanId,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// metricDocBase holds the fields shared by the SS4O and OTel v1 metric
// document schemas. One document is emitted per metric data point; fields that
// do not apply to the data point type are omitted.
type metricDocBase struct {
	Name                   string           `json:"name"`
	Description            string           `json:"description,omitempty"`
	Unit                   string           `json:"unit,omitempty"`
	Kind                   string           `json:"kind"`
	AggregationTemporality string           `json:"aggregationTemporality,omitempty"`
	Monotonic              *bool            `json:"monotonic,omitempty"`
	StartTime              time.Time        `json:"startTime"`
	Timestamp              time.Time        `json:"@timestamp"`
	ServiceName            string           `json:"serviceName,omitempty"`
	ValueInt               *int64           `json:"value@int,omitempty"`
	ValueDouble            *float64         `json:"value@double,omitempty"`
	Buckets                []metricBucket   `json:"buckets,omitempty"`
	BucketCount            *int             `json:"bucketCount,omitempty"`
	BucketCountsList       []uint64         `json:"bucketCountsList,omitempty"`
	ExplicitBoundsList     []float64        `json:"explicitBoundsList,omitempty"`
	ExplicitBoundsCount    *int             `json:"explicitBoundsCount,omitempty"`
	Quantiles              []metricQuantile `json:"quantiles,omitempty"`
	QuantileValuesCount    *int             `json:"quantileValuesCount,omitempty"`
	PositiveBuckets        []metricBucket   `json:"positiveBuckets,omitempty"`
	NegativeBuckets        []metricBucket   `json:"negativeBuckets,omitempty"`
	PositiveOffset         *int32           `json:"positiveOffset,omitempty"`
	NegativeOffset         *int32           `json:"negativeOffset,omitempty"`
	ZeroCount              *uint64          `json:"zeroCount,omitempty"`
	Scale                  *int32           `json:"scale,omitempty"`
	Max                    *float64         `json:"max,omitempty"`
	Min                    *float64         `json:"min,omitempty"`
	Sum                    *float64         `json:"sum,omitempty"`
	Count                  *uint64          `json:"count,omitempty"`
	Exemplars              []metricExemplar `json:"exemplars,omitempty"`
	Attributes             map[string]any   `json:"attributes,omitempty"`
	SchemaURL              string           `json:"schemaUrl,omitempty"`
}

// ssoMetric is a metric document following the Simple Schema for Observability.
// See: https://github.com/opensearch-project/opensearch-catalog/tree/main/schema/observability/metrics
type ssoMetric struct {
	metricDocBase
	ObservedTimestamp    *time.Time        `json:"observedTimestamp,omitempty"`
	Resource             map[string]string `json:"resource,omitempty"`
	InstrumentationScope struct {
		Attributes map[string]any `json:"attributes,omitempty"`
		Name       string         `json:"name,omitempty"`
		SchemaURL  string         `json:"schemaUrl,omitempty"`
		Version    string         `json:"version,omitempty"`
	} `json:"instrumentationScope,omitzero"`
}

// otelV1Metric is a metric document following the Data Prepper OTel v1 metrics schema.
type otelV1Metric struct {
	metricDocBase
	Time                 time.Time      `json:"time"`
	Flags                int64          `json:"flags"`
	Value                *float64       `json:"value,omitempty"`
	Resource             otelV1Resource `json:"resource"`
	InstrumentationScope otelV1Scope    `json:"instrumentationScope"`
}
