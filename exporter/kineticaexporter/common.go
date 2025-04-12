// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kineticaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kineticaexporter"

import (
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

const (
	MeasurementSpans     = "spans"
	MeasurementSpanLinks = "span-links"
	MeasurementLogs      = "logs"

	// These attribute key names are influenced by the proto message keys.
	AttributeTime                   = "time"
	AttributeStartTimeUnixNano      = "start_time_unix_nano"
	AttributeTraceID                = "trace_id"
	AttributeSpanID                 = "span_id"
	AttributeTraceState             = "trace_state"
	AttributeParentSpanID           = "parent_span_id"
	AttributeParentServiceName      = "parent_service_name"
	AttributeChildServiceName       = "child_service_name"
	AttributeCallCount              = "call_count"
	AttributeSpansQueueDepth        = "spans_queue_depth"
	AttributeSpansDropped           = "spans_dropped"
	AttributeName                   = "name"
	AttributeSpanKind               = "kind"
	AttributeEndTimeUnixNano        = "end_time_unix_nano"
	AttributeDurationNano           = "duration_nano"
	AttributeDroppedAttributesCount = "dropped_attributes_count"
	AttributeDroppedEventsCount     = "dropped_events_count"
	AttributeDroppedLinksCount      = "dropped_links_count"
	AttributeAttributes             = "attributes"
	AttributeLinkedTraceID          = "linked_trace_id"
	AttributeLinkedSpanID           = "linked_span_id"
	AttributeSeverityNumber         = "severity_number"
	AttributeSeverityText           = "severity_text"
	AttributeBody                   = "body"

	LogTable                  = "log"
	LogAttributeTable         = "log_attribute"
	LogResourceAttributeTable = "log_resource_attribute"
	LogScopeAttributeTable    = "log_scope_attribute"

	TraceSpanTable              = "trace_span"
	TraceSpanAttributeTable     = "trace_span_attribute"
	TraceResourceAttributeTable = "trace_resource_attribute"
	TraceScopeAttributeTable    = "trace_scope_attribute"
	TraceEventAttributeTable    = "trace_event_attribute"
	TraceLinkAttributeTable     = "trace_link_attribute"

	GaugeTable                           = "metric_gauge"
	GaugeDatapointTable                  = "metric_gauge_datapoint"
	GaugeDatapointAttributeTable         = "metric_gauge_datapoint_attribute"
	GaugeDatapointExemplarTable          = "metric_gauge_datapoint_exemplar"
	GaugeDatapointExemplarAttributeTable = "metric_gauge_datapoint_exemplar_attribute"
	GaugeResourceAttributeTable          = "metric_gauge_resource_attribute"
	GaugeScopeAttributeTable             = "metric_gauge_scope_attribute"

	SumTable                           = "metric_sum"
	SumResourceAttributeTable          = "metric_sum_resource_attribute"
	SumScopeAttributeTable             = "metric_sum_scope_attribute"
	SumDatapointTable                  = "metric_sum_datapoint"
	SumDatapointAttributeTable         = "metric_sum_datapoint_attribute"
	SumDatapointExemplarTable          = "metric_sum_datapoint_exemplar"
	SumDataPointExemplarAttributeTable = "metric_sum_datapoint_exemplar_attribute"

	HistogramTable                           = "metric_histogram"
	HistogramResourceAttributeTable          = "metric_histogram_resource_attribute"
	HistogramScopeAttributeTable             = "metric_histogram_scope_attribute"
	HistogramDatapointTable                  = "metric_histogram_datapoint"
	HistogramDatapointAttributeTable         = "metric_histogram_datapoint_attribute"
	HistogramBucketCountsTable               = "metric_histogram_datapoint_bucket_count"
	HistogramExplicitBoundsTable             = "metric_histogram_datapoint_explicit_bound"
	HistogramDatapointExemplarTable          = "metric_histogram_datapoint_exemplar"
	HistogramDataPointExemplarAttributeTable = "metric_histogram_datapoint_exemplar_attribute"

	ExpHistogramTable                           = "metric_exp_histogram"
	ExpHistogramResourceAttributeTable          = "metric_exp_histogram_resource_attribute"
	ExpHistogramScopeAttributeTable             = "metric_exp_histogram_scope_attribute"
	ExpHistogramDatapointTable                  = "metric_exp_histogram_datapoint"
	ExpHistogramDatapointAttributeTable         = "metric_exp_histogram_datapoint_attribute"
	ExpHistogramPositiveBucketCountsTable       = "metric_exp_histogram_datapoint_bucket_positive_count"
	ExpHistogramNegativeBucketCountsTable       = "metric_exp_histogram_datapoint_bucket_negative_count"
	ExpHistogramDatapointExemplarTable          = "metric_exp_histogram_datapoint_exemplar"
	ExpHistogramDataPointExemplarAttributeTable = "metric_exp_histogram_datapoint_exemplar_attribute"

	SummaryTable                       = "metric_summary"
	SummaryResourceAttributeTable      = "metric_summary_resource_attribute"
	SummaryScopeAttributeTable         = "metric_summary_scope_attribute"
	SummaryDatapointTable              = "metric_summary_datapoint"
	SummaryDatapointAttributeTable     = "metric_summary_datapoint_attribute"
	SummaryDatapointQuantileValueTable = "metric_summary_datapoint_quantile_values"

	ChunkSize = 10000
)

const (
	CreateSchema string = "create schema if not exists %s;"

	HasTable string = "execute endpoint '/has/table' JSON '{\"table_name\":\"%s\"}'"

	// Metrics - DDLs
	// Gauge
	CreateGauge string = `CREATE TABLE %smetric_gauge
	(
		gauge_id UUID (primary_key, shard_key) not null,
		metric_name varchar(256) not null,
		metric_description varchar (256),
		metric_unit varchar (256)
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateGaugeDatapoint string = `CREATE TABLE %smetric_gauge_datapoint
	(
		gauge_id UUID (primary_key, shard_key) not null,
		id UUID (primary_key) not null,
		start_time_unix TIMESTAMP NOT NULL,
		time_unix TIMESTAMP NOT NULL,
		gauge_value DOUBLE,
		flags INT,
		FOREIGN KEY (gauge_id) references %smetric_gauge(gauge_id) as fk_gauge_datapoint
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateGaugeDatapointAttribute string = `CREATE TABLE %smetric_gauge_datapoint_attribute
	(
		"gauge_id" UUID (primary_key, shard_key) NOT NULL,
		datapoint_id uuid (primary_key) not null,
		"key" VARCHAR (primary_key, 128, dict) NOT NULL,
		"string_value" VARCHAR (256),
		"bool_value" BOOLEAN,
		"int_value" INTEGER,
		"double_value" DOUBLE,
		"bytes_value" BLOB (store_only),
		FOREIGN KEY (gauge_id) references %smetric_gauge(gauge_id) as fk_gauge_datapoint_attribute
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateGaugeDatapointExemplar string = `CREATE TABLE %smetric_gauge_datapoint_exemplar
	(
		"gauge_id" UUID (primary_key, shard_key) NOT NULL,
		datapoint_id uuid (primary_key) not null,
		exemplar_id UUID (primary_key) not null,
		time_unix TIMESTAMP NOT NULL,
		gauge_value DOUBLE,
		"trace_id" VARCHAR (32),
		"span_id" VARCHAR (16),
		FOREIGN KEY (gauge_id) references %smetric_gauge(gauge_id) as fk_gauge_datapoint_exemplar
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateGaugeDatapointExemplarAttribute string = `CREATE TABLE %smetric_gauge_datapoint_exemplar_attribute
	(
		"gauge_id" UUID (primary_key, shard_key) NOT NULL,
		datapoint_id uuid (primary_key) not null,
		exemplar_id UUID (primary_key) not null,
		"key" VARCHAR (primary_key, 128, dict) NOT NULL,
		"string_value" VARCHAR (256),
		"bool_value" BOOLEAN,
		"int_value" INTEGER,
		"double_value" DOUBLE,
		"bytes_value" BLOB (store_only),
		FOREIGN KEY (gauge_id) references %smetric_gauge(gauge_id) as fk_gauge_datapoint_exemplar_attribute
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateGaugeResourceAttribute string = `CREATE TABLE %smetric_gauge_resource_attribute
	(
		"gauge_id" UUID (primary_key) NOT NULL,
		"key" VARCHAR (primary_key, 128, dict) NOT NULL,
		"string_value" VARCHAR (256),
		"bool_value" BOOLEAN,
		"int_value" INTEGER,
		"double_value" DOUBLE,
		"bytes_value" BLOB (store_only),
		SHARD KEY (gauge_id),
		FOREIGN KEY (gauge_id) references %smetric_gauge(gauge_id) as fk_gauge_resource_attribute
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateGaugeScopeAttribute string = `CREATE TABLE %smetric_gauge_scope_attribute
	(
		"gauge_id" UUID (primary_key) NOT NULL,
		"name" VARCHAR (256),
		"version" VARCHAR (256),
		"key" VARCHAR (primary_key, 128, dict) NOT NULL,
		"string_value" VARCHAR (256),
		"bool_value" BOOLEAN,
		"int_value" INTEGER,
		"double_value" DOUBLE,
		"bytes_value" BLOB (store_only),
		SHARD KEY (gauge_id),
		FOREIGN KEY (gauge_id) references %smetric_gauge(gauge_id) as fk_gauge_scope_attribute
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	// Sum

	CreateSum string = `CREATE TABLE %smetric_sum
	(
		sum_id UUID (primary_key, shard_key) not null,
		metric_name varchar (256) not null,
		metric_description varchar (256),
		metric_unit varchar (256),
		aggregation_temporality INTEGER,
		is_monotonic BOOLEAN
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateSumDatapoint string = `CREATE TABLE %smetric_sum_datapoint
	(
		sum_id UUID (primary_key, shard_key) not null,
		id UUID (primary_key) not null,
		start_time_unix TIMESTAMP NOT NULL,
		time_unix TIMESTAMP NOT NULL,
		sum_value DOUBLE,
		flags INT,
		FOREIGN KEY (sum_id) references %smetric_sum(sum_id) as fk_sum_datapoint
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateSumDatapointAttribute string = `CREATE TABLE %smetric_sum_datapoint_attribute
	(
		"sum_id" UUID (primary_key, shard_key) NOT NULL,
		datapoint_id uuid (primary_key) not null,
		"key" VARCHAR (primary_key, 128, dict) NOT NULL,
		"string_value" VARCHAR (256),
		"bool_value" BOOLEAN,
		"int_value" INTEGER,
		"double_value" DOUBLE,
		"bytes_value" BLOB (store_only),
		FOREIGN KEY (sum_id) references %smetric_sum(sum_id) as fk_sum_datapoint_attribute
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateSumDatapointExemplar string = `CREATE TABLE %smetric_sum_datapoint_exemplar
	(
		"sum_id" UUID (primary_key, shard_key) NOT NULL,
		datapoint_id uuid (primary_key) not null,
		exemplar_id UUID (primary_key) not null,
		time_unix TIMESTAMP NOT NULL,
		sum_value DOUBLE,
		"trace_id" VARCHAR (32),
		"span_id" VARCHAR (16),
		FOREIGN KEY (sum_id) references %smetric_sum(sum_id) as fk_sum_datapoint_exemplar
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateSumDatapointExemplarAttribute string = `CREATE TABLE %smetric_sum_datapoint_exemplar_attribute
	(
		"sum_id" UUID (primary_key, shard_key) NOT NULL,
		datapoint_id uuid (primary_key) not null,
		exemplar_id UUID (primary_key) not null,
		"key" VARCHAR (primary_key, 128, dict) NOT NULL,
		"string_value" VARCHAR (256),
		"bool_value" BOOLEAN,
		"int_value" INTEGER,
		"double_value" DOUBLE,
		"bytes_value" BLOB (store_only),
		FOREIGN KEY (sum_id) references %smetric_sum(sum_id) as fk_sum_datapoint_exemplar_attribute
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateSumResourceAttribute string = `CREATE TABLE %smetric_sum_resource_attribute
	(
		"sum_id" UUID (primary_key) NOT NULL,
		"key" VARCHAR (primary_key, 128, dict) NOT NULL,
		"string_value" VARCHAR (256),
		"bool_value" BOOLEAN,
		"int_value" INTEGER,
		"double_value" DOUBLE,
		"bytes_value" BLOB (store_only),
		SHARD KEY (sum_id),
		FOREIGN KEY (sum_id) references %smetric_sum(sum_id) as fk_sum_resource_attribute
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateSumScopeAttribute string = `CREATE TABLE %smetric_sum_scope_attribute
	(
		"sum_id" UUID (primary_key) NOT NULL,
		"name" VARCHAR (256),
		"version" VARCHAR (256),
		"key" VARCHAR (primary_key, 128, dict) NOT NULL,
		"string_value" VARCHAR (256),
		"bool_value" BOOLEAN,
		"int_value" INTEGER,
		"double_value" DOUBLE,
		"bytes_value" BLOB (store_only),
		SHARD KEY (sum_id),
		FOREIGN KEY (sum_id) references %smetric_sum(sum_id) as fk_sum_scope_attribute
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`

	// Histogram

	CreateHistogram string = `CREATE TABLE %smetric_histogram
	(
		histogram_id UUID (primary_key, shard_key) not null,
		metric_name varchar (256) not null,
		metric_description varchar (256),
		metric_unit varchar (256),
		aggregation_temporality int8
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`

	CreateHistogramDatapoint string = `CREATE TABLE %smetric_histogram_datapoint
	(
		histogram_id UUID (primary_key, shard_key) not null,
		id UUID (primary_key) not null,
		start_time_unix TIMESTAMP,
		time_unix TIMESTAMP NOT NULL,
		count LONG,
		data_sum DOUBLE,
		data_min DOUBLE,
		data_max DOUBLE,
		flags INT,
		FOREIGN KEY (histogram_id) references %smetric_histogram(histogram_id) as fk_histogram_datapoint
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateHistogramDatapointBucketCount string = `CREATE TABLE %smetric_histogram_datapoint_bucket_count
	(
		histogram_id UUID (primary_key, shard_key) not null,
		datapoint_id UUID (primary_key) not null,
		count_id UUID (primary_key) not null,
		count LONG,
		FOREIGN KEY (histogram_id) references %smetric_histogram(histogram_id) as fk_histogram_datapoint_bucket_count
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateHistogramDatapointExplicitBound string = `CREATE TABLE %smetric_histogram_datapoint_explicit_bound
	(
		histogram_id UUID (primary_key, shard_key) not null,
		datapoint_id UUID (primary_key) not null,
		bound_id UUID (primary_key) not null,
		explicit_bound DOUBLE,
		FOREIGN KEY (histogram_id) references %smetric_histogram(histogram_id) as fk_histogram_datapoint_explicit_bound
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateHistogramDatapointAttribute string = `CREATE TABLE %smetric_histogram_datapoint_attribute
	(
		"histogram_id" UUID (primary_key, shard_key) NOT NULL,
		datapoint_id uuid (primary_key) not null,
		"key" VARCHAR (primary_key, 128, dict) NOT NULL,
		"string_value" VARCHAR (256),
		"bool_value" BOOLEAN,
		"int_value" INTEGER,
		"double_value" DOUBLE,
		"bytes_value" BLOB (store_only),
		FOREIGN KEY (histogram_id) references %smetric_histogram(histogram_id) as fk_histogram_datapoint_attribute
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateHistogramDatapointExemplar string = `CREATE TABLE %smetric_histogram_datapoint_exemplar
	(
		"histogram_id" UUID (primary_key, shard_key) NOT NULL,
		datapoint_id uuid (primary_key) not null,
		exemplar_id UUID (primary_key) not null,
		time_unix TIMESTAMP NOT NULL,
		histogram_value DOUBLE,
		"trace_id" VARCHAR (32),
		"span_id" VARCHAR (16),
		FOREIGN KEY (histogram_id) references %smetric_histogram(histogram_id) as fk_histogram_datapoint_exemplar
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateHistogramDatapointExemplarAttribute string = `CREATE TABLE %smetric_histogram_datapoint_exemplar_attribute
	(
		"histogram_id" UUID (primary_key, shard_key) NOT NULL,
		datapoint_id uuid (primary_key) not null,
		exemplar_id UUID (primary_key) not null,
		"key" VARCHAR (primary_key, 128, dict) NOT NULL,
		"string_value" VARCHAR (256),
		"bool_value" BOOLEAN,
		"int_value" INTEGER,
		"double_value" DOUBLE,
		"bytes_value" BLOB (store_only),
		FOREIGN KEY (histogram_id) references %smetric_histogram(histogram_id) as fk_histogram_datapoint_exemplar_attribute
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateHistogramResourceAttribute string = `CREATE TABLE %smetric_histogram_resource_attribute
	(
		"histogram_id" UUID (primary_key) NOT NULL,
		"key" VARCHAR (primary_key, 128, dict) NOT NULL,
		"string_value" VARCHAR (256),
		"bool_value" BOOLEAN,
		"int_value" INTEGER,
		"double_value" DOUBLE,
		"bytes_value" BLOB (store_only),
		SHARD KEY (histogram_id),
		FOREIGN KEY (histogram_id) references %smetric_histogram(histogram_id) as fk_histogram_resource_attribute
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateHistogramScopeAttribute string = `CREATE TABLE %smetric_histogram_scope_attribute
	(
		"histogram_id" UUID (primary_key) NOT NULL,
		"name" VARCHAR (256),
		"version" VARCHAR (256),
		"key" VARCHAR (primary_key, 128, dict) NOT NULL,
		"string_value" VARCHAR (256),
		"bool_value" BOOLEAN,
		"int_value" INTEGER,
		"double_value" DOUBLE,
		"bytes_value" BLOB (store_only),
		SHARD KEY (histogram_id),
		FOREIGN KEY (histogram_id) references %smetric_histogram(histogram_id) as fk_histogram_scope_attribute
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	// exponential Histogram
	CreateExpHistogram string = `CREATE TABLE %smetric_exp_histogram
	(
		histogram_id UUID (primary_key, shard_key) not null,
		metric_name varchar (256) not null,
		metric_description varchar (256),
		metric_unit varchar (256),
		aggregation_temporality int8
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateExpHistogramDatapoint string = `CREATE TABLE %smetric_exp_histogram_datapoint
	(
		histogram_id UUID (primary_key, shard_key) not null,
		id UUID (primary_key) not null,
		start_time_unix TIMESTAMP,
		time_unix TIMESTAMP NOT NULL,
		count LONG,
		data_sum DOUBLE,
		scale INTEGER,
		zero_count LONG,
		buckets_positive_offset INTEGER,
		buckets_negative_offset INTEGER,
		data_min DOUBLE,
		data_max DOUBLE,
		flags INT,
		FOREIGN KEY (histogram_id) references %smetric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateExpHistogramDatapointBucketPositiveCount string = `CREATE TABLE %smetric_exp_histogram_datapoint_bucket_positive_count
	(
		histogram_id UUID (primary_key, shard_key) not null,
		datapoint_id UUID (primary_key) not null,
		count_id UUID (primary_key) not null,
		count LONG,
		FOREIGN KEY (histogram_id) references %smetric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint_bucket_count
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateExpHistogramDatapointBucketNegativeCount string = `CREATE TABLE %smetric_exp_histogram_datapoint_bucket_negative_count
	(
		histogram_id UUID (primary_key, shard_key) not null,
		datapoint_id UUID (primary_key) not null,
		count_id UUID (primary_key) not null,
		count LONG,
		FOREIGN KEY (histogram_id) references %smetric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint_bucket_count
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateExpHistogramDatapointAttribute string = `CREATE TABLE %smetric_exp_histogram_datapoint_attribute
	(
		"histogram_id" UUID (primary_key, shard_key) NOT NULL,
		datapoint_id uuid (primary_key) not null,
		"key" VARCHAR (primary_key, 128, dict) NOT NULL,
		"string_value" VARCHAR (256),
		"bool_value" BOOLEAN,
		"int_value" INTEGER,
		"double_value" DOUBLE,
		"bytes_value" BLOB (store_only),
		FOREIGN KEY (histogram_id) references %smetric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint_attribute
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateExpHistogramDatapointExemplar string = `CREATE TABLE %smetric_exp_histogram_datapoint_exemplar
	(
		"histogram_id" UUID (primary_key, shard_key) NOT NULL,
		datapoint_id uuid (primary_key) not null,
		exemplar_id UUID (primary_key) not null,
		time_unix TIMESTAMP NOT NULL,
		sum_value DOUBLE,
		"trace_id" VARCHAR (32),
		"span_id" VARCHAR (16),
		FOREIGN KEY (histogram_id) references %smetric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint_exemplar
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateExpHistogramDatapointExemplarAttribute string = `CREATE TABLE %smetric_exp_histogram_datapoint_exemplar_attribute
	(
		"histogram_id" UUID (primary_key, shard_key) NOT NULL,
		datapoint_id uuid (primary_key) not null,
		exemplar_id UUID (primary_key) not null,
		"key" VARCHAR (primary_key, 128, dict) NOT NULL,
		"string_value" VARCHAR (256),
		"bool_value" BOOLEAN,
		"int_value" INTEGER,
		"double_value" DOUBLE,
		"bytes_value" BLOB (store_only),
		FOREIGN KEY (histogram_id) references %smetric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint_exemplar_attribute
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateExpHistogramResourceAttribute string = `CREATE TABLE %smetric_exp_histogram_resource_attribute
	(
		"histogram_id" UUID (primary_key) NOT NULL,
		"key" VARCHAR (primary_key, 128, dict) NOT NULL,
		"string_value" VARCHAR (256),
		"bool_value" BOOLEAN,
		"int_value" INTEGER,
		"double_value" DOUBLE,
		"bytes_value" BLOB (store_only),
		SHARD KEY (histogram_id),
		FOREIGN KEY (histogram_id) references %smetric_histogram(histogram_id) as fk_histogram_resource_attribute
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateExpHistogramScopeAttribute string = `CREATE TABLE %smetric_exp_histogram_scope_attribute
	(
		"histogram_id" UUID (primary_key) NOT NULL,
		"name" VARCHAR (256),
		"version" VARCHAR (256),
		"key" VARCHAR (primary_key, 128, dict) NOT NULL,
		"string_value" VARCHAR (256),
		"bool_value" BOOLEAN,
		"int_value" INTEGER,
		"double_value" DOUBLE,
		"bytes_value" BLOB (store_only),
		SHARD KEY (histogram_id),
		FOREIGN KEY (histogram_id) references %smetric_histogram(histogram_id) as fk_histogram_scope_attribute
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	// Summary
	CreateSummary string = `CREATE TABLE %smetric_summary
	(
		summary_id UUID (primary_key, shard_key) not null,
		metric_name varchar (256) not null,
		metric_description varchar (256),
		metric_unit varchar (256)
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateSummaryDatapoint string = `CREATE TABLE %smetric_summary_datapoint
	(
		summary_id UUID (primary_key, shard_key) not null,
		id UUID (primary_key) not null,
		start_time_unix TIMESTAMP,
		time_unix TIMESTAMP NOT NULL,
		count LONG,
		data_sum DOUBLE,
		flags INT,
		FOREIGN KEY (summary_id) references %smetric_summary(summary_id) as fk_summary_datapoint
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateSummaryDatapointAttribute string = `CREATE TABLE %smetric_summary_datapoint_attribute
	(
		"summary_id" UUID (primary_key, shard_key) NOT NULL,
		datapoint_id uuid (primary_key) not null,
		"key" VARCHAR (primary_key, 128, dict) NOT NULL,
		"string_value" VARCHAR (256),
		"bool_value" BOOLEAN,
		"int_value" INTEGER,
		"double_value" DOUBLE,
		"bytes_value" BLOB (store_only),
		FOREIGN KEY (summary_id) references %smetric_summary(summary_id) as fk_summary_datapoint_attribute
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateSummaryDatapointQuantileValues string = `CREATE TABLE %smetric_summary_datapoint_quantile_values
	(
		summary_id UUID (primary_key, shard_key) not null,
		datapoint_id UUID (primary_key) not null,
		quantile_id UUID (primary_key) not null,
		quantile DOUBLE,
		value DOUBLE,
		FOREIGN KEY (summary_id) references %smetric_summary(summary_id) as fk_summary_datapoint_quantile
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateSummaryResourceAttribute string = `CREATE TABLE %smetric_summary_resource_attribute
	(
		"summary_id" UUID (primary_key) NOT NULL,
		"key" VARCHAR (primary_key, 128, dict) NOT NULL,
		"string_value" VARCHAR (256),
		"bool_value" BOOLEAN,
		"int_value" INTEGER,
		"double_value" DOUBLE,
		"bytes_value" BLOB (store_only),
		SHARD KEY (summary_id),
		FOREIGN KEY (summary_id) references %smetric_summary(summary_id) as fk_summary_resource_attribute
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
	CreateSummaryScopeAttribute string = `CREATE TABLE %smetric_summary_scope_attribute
	(
		"summary_id" UUID (primary_key) NOT NULL,
		"name" VARCHAR (256),
		"version" VARCHAR (256),
		"key" VARCHAR (primary_key, 128, dict) NOT NULL,
		"string_value" VARCHAR (256),
		"bool_value" BOOLEAN,
		"int_value" INTEGER,
		"double_value" DOUBLE,
		"bytes_value" BLOB (store_only),
		SHARD KEY (summary_id),
		FOREIGN KEY (summary_id) references %smetric_summary(summary_id) as fk_summary_scope_attribute
	) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);
	`
)

// ValueTypePair - struct to wrap a value as [any] and its type [pcommon.ValueType]
type ValueTypePair struct {
	value     any
	valueType pcommon.ValueType
}

// attributeValueToKineticaFieldValue - Convert an attribute value to a [ValueTypePair] for writing to Kinetica
//
//	@param value
//	@return ValueTypePair
//	@return error
func attributeValueToKineticaFieldValue(value pcommon.Value) (ValueTypePair, error) {
	switch value.Type() {
	case pcommon.ValueTypeStr:
		var val string
		if len(value.Str()) > 256 {
			val = value.Str()[0:255]
		} else {
			val = value.Str()
		}
		return ValueTypePair{val, pcommon.ValueTypeStr}, nil
	case pcommon.ValueTypeInt:
		return ValueTypePair{value.Int(), pcommon.ValueTypeInt}, nil
	case pcommon.ValueTypeDouble:
		return ValueTypePair{value.Double(), pcommon.ValueTypeDouble}, nil
	case pcommon.ValueTypeBool:
		return ValueTypePair{value.Bool(), pcommon.ValueTypeBool}, nil
	case pcommon.ValueTypeMap:
		jsonBytes, err := json.Marshal(otlpKeyValueListToMap(value.Map()))
		if err != nil {
			return ValueTypePair{nil, pcommon.ValueTypeEmpty}, err
		}
		return ValueTypePair{string(jsonBytes), pcommon.ValueTypeStr}, nil
	case pcommon.ValueTypeSlice:
		jsonBytes, err := json.Marshal(otlpArrayToSlice(value.Slice()))
		if err != nil {
			return ValueTypePair{nil, pcommon.ValueTypeEmpty}, err
		}
		return ValueTypePair{string(jsonBytes), pcommon.ValueTypeStr}, nil
	case pcommon.ValueTypeEmpty:
		return ValueTypePair{nil, pcommon.ValueTypeEmpty}, nil
	default:
		return ValueTypePair{nil, pcommon.ValueTypeEmpty}, fmt.Errorf("Unknown value type %v", value)
	}
}

// otlpKeyValueListToMap - Convert an otlp Map to a map[string]interface{} with proper type conversions
//
//	@param kvList
//	@return map
func otlpKeyValueListToMap(kvList pcommon.Map) map[string]any {
	m := make(map[string]any, kvList.Len())
	for k, v := range kvList.All() {
		switch v.Type() {
		case pcommon.ValueTypeStr:
			m[k] = v.Str()
		case pcommon.ValueTypeInt:
			m[k] = v.Int()
		case pcommon.ValueTypeDouble:
			m[k] = v.Double()
		case pcommon.ValueTypeBool:
			m[k] = v.Bool()
		case pcommon.ValueTypeMap:
			m[k] = otlpKeyValueListToMap(v.Map())
		case pcommon.ValueTypeSlice:
			m[k] = otlpArrayToSlice(v.Slice())
		case pcommon.ValueTypeEmpty:
			m[k] = nil
		default:
			m[k] = fmt.Sprintf("<invalid map value> %v", v)
		}
	}
	return m
}

// otlpArrayToSlice - Convert an otlp slice to a slice of interface{} with proper type conversions
//
//	@param arr
//	@return []interface{}
func otlpArrayToSlice(arr pcommon.Slice) []any {
	s := make([]any, 0, arr.Len())
	for i := 0; i < arr.Len(); i++ {
		v := arr.At(i)
		switch v.Type() {
		case pcommon.ValueTypeStr:
			s = append(s, v.Str())
		case pcommon.ValueTypeInt:
			s = append(s, v.Int())
		case pcommon.ValueTypeDouble:
			s = append(s, v.Double())
		case pcommon.ValueTypeBool:
			s = append(s, v.Bool())
		case pcommon.ValueTypeEmpty:
			s = append(s, nil)
		default:
			s = append(s, fmt.Sprintf("<invalid array value> %v", v))
		}
	}
	return s
}

// getAttributeValue
//
//	@param vtPair
//	@return *AttributeValue
//	@return error
func getAttributeValue(vtPair ValueTypePair) (*AttributeValue, error) {
	var av *AttributeValue
	var err error
	switch vtPair.valueType {
	case pcommon.ValueTypeStr:
		value := vtPair.value.(string)
		av = new(AttributeValue)
		av.StringValue = value
	case pcommon.ValueTypeInt:
		value := vtPair.value.(int)
		av = new(AttributeValue)
		av.IntValue = value
	case pcommon.ValueTypeDouble:
		value := vtPair.value.(float64)
		av = new(AttributeValue)
		av.DoubleValue = value
	case pcommon.ValueTypeBool:
		value := vtPair.value.(int8)
		av = new(AttributeValue)
		av.BoolValue = value
	case pcommon.ValueTypeBytes:
		value := vtPair.value.([]byte)
		av = new(AttributeValue)
		copy(av.BytesValue, value)
	case pcommon.ValueTypeMap:
		// value := vtPair.value
		// av = new(AttributeValue)
		// av.SetStringValue(value)
		err = fmt.Errorf("Unhandled value type %v", vtPair.valueType)

	case pcommon.ValueTypeSlice:
		// value := vtPair.value.(string)
		// av = new(AttributeValue)
		// av.SetStringValue(value)
		err = fmt.Errorf("Unhandled value type %v", vtPair.valueType)

	default:
		err = fmt.Errorf("Unknown value type %v", vtPair.valueType)
	}

	if err != nil {
		return nil, err
	}

	return av, nil
}

// chunkBySize - Splits a slice into multiple slices of the given size
//
//	@param items
//	@param chunkSize
//	@return [][]T
func chunkBySize[T any](items []T, chunkSize int) [][]T {
	_chunks := make([][]T, 0, (len(items)/chunkSize)+1)
	for chunkSize < len(items) {
		items, _chunks = items[chunkSize:], append(_chunks, items[0:chunkSize:chunkSize])
	}
	return append(_chunks, items)
}
