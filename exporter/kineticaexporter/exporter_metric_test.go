// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kineticaexporter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap/zaptest"
)

var testServer *httptest.Server
var baseURL string

var showTableMetricSummaryResponse = "\x04OK\x00&show_table_response\xfe\x13&otel.metric_summary\x02&otel.metric_summary\x00\x02\x00\x00\x02(14940230562727554683\x00\x02\xc6\x03{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"summary_id\",\"type\":\"string\"},{\"name\":\"metric_name\",\"type\":\"string\"},{\"name\":\"metric_description\",\"type\":[\"string\",\"null\"]},{\"name\":\"metric_unit\",\"type\":[\"string\",\"null\"]}]}\x00\x02\x00\x00\x02\x08$metric_description\x06\x08data\x0echar256\x10nullable\x00\x16metric_name\x04\x08data\x0echar256\x00\x16metric_unit\x06\x08data\x0echar256\x10nullable\x00\x14summary_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\x00\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06784\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xe2\x02{\"table_name\":\"otel.metric_summary\",\"type_id\":\"14940230562727554683\",\"options\":{\"foreign_keys\":\"\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
var showTableMetricSummaryDatapointResponse = "\x04OK\x00&show_table_response\xae\x19:otel.metric_summary_datapoint\x02:otel.metric_summary_datapoint\x00\x02\x00\x00\x02(11442617697458528032\x00\x02\x88\x05{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"summary_id\",\"type\":\"string\"},{\"name\":\"id\",\"type\":\"string\"},{\"name\":\"start_time_unix\",\"type\":[\"long\",\"null\"]},{\"name\":\"time_unix\",\"type\":\"long\"},{\"name\":\"count\",\"type\":[\"long\",\"null\"]},{\"name\":\"data_sum\",\"type\":[\"double\",\"null\"]},{\"name\":\"flags\",\"type\":[\"int\",\"null\"]}]}\x00\x02\x00\x00\x02\x0e\ncount\x04\x08data\x10nullable\x00\x10data_sum\x04\x08data\x10nullable\x00\nflags\x04\x08data\x10nullable\x00\x04id\x06\x08data\x16primary_key\x08uuid\x00\x1estart_time_unix\x06\x08data\x12timestamp\x10nullable\x00\x14summary_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x12time_unix\x04\x08data\x12timestamp\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\x9e\x01(summary_id) references otel.metric_summary(summary_id) as fk_summary_datapoint\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x0468\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\x94\x04{\"table_name\":\"otel.metric_summary_datapoint\",\"type_id\":\"11442617697458528032\",\"options\":{\"foreign_keys\":\"(summary_id) references otel.metric_summary(summary_id) as fk_summary_datapoint\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
var showTableMetricSummaryDatapointAttributeResponse = "\x04OK\x00&show_table_response\x96\x1cNotel.metric_summary_datapoint_attribute\x02Notel.metric_summary_datapoint_attribute\x00\x02\x00\x00\x02$893873402458912716\x00\x02\x88\x06{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"summary_id\",\"type\":\"string\"},{\"name\":\"datapoint_id\",\"type\":\"string\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x10\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18datapoint_id\x06\x08data\x16primary_key\x08uuid\x00\x18double_value\x04\x08data\x10nullable\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x14summary_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xb2\x01(summary_id) references otel.metric_summary(summary_id) as fk_summary_datapoint_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06429\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xb8\x04{\"table_name\":\"otel.metric_summary_datapoint_attribute\",\"type_id\":\"893873402458912716\",\"options\":{\"foreign_keys\":\"(summary_id) references otel.metric_summary(summary_id) as fk_summary_datapoint_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
var showTableMetricSummaryDatapointQuantileValuesResponse = "\x04OK\x00&show_table_response\xc2\x18Zotel.metric_summary_datapoint_quantile_values\x02Zotel.metric_summary_datapoint_quantile_values\x00\x02\x00\x00\x02&9163876868172339469\x00\x02\xf6\x03{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"summary_id\",\"type\":\"string\"},{\"name\":\"datapoint_id\",\"type\":\"string\"},{\"name\":\"quantile_id\",\"type\":\"string\"},{\"name\":\"quantile\",\"type\":[\"double\",\"null\"]},{\"name\":\"value\",\"type\":[\"double\",\"null\"]}]}\x00\x02\x00\x00\x02\n\x18datapoint_id\x06\x08data\x16primary_key\x08uuid\x00\x10quantile\x04\x08data\x10nullable\x00\x16quantile_id\x06\x08data\x16primary_key\x08uuid\x00\x14summary_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\nvalue\x04\x08data\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xb0\x01(summary_id) references otel.metric_summary(summary_id) as fk_summary_datapoint_quantile\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x0464\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xc4\x04{\"table_name\":\"otel.metric_summary_datapoint_quantile_values\",\"type_id\":\"9163876868172339469\",\"options\":{\"foreign_keys\":\"(summary_id) references otel.metric_summary(summary_id) as fk_summary_datapoint_quantile\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
var showTableMetricSummaryResourceAttributesResponse = "\x04OK\x00&show_table_response\xfa\x1aLotel.metric_summary_resource_attribute\x02Lotel.metric_summary_resource_attribute\x00\x02\x00\x00\x02(11898281161813583768\x00\x02\xb8\x05{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"summary_id\",\"type\":\"string\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x0e\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18double_value\x04\x08data\x10nullable\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x14summary_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xb0\x01(summary_id) references otel.metric_summary(summary_id) as fk_summary_resource_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06413\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xb8\x04{\"table_name\":\"otel.metric_summary_resource_attribute\",\"type_id\":\"11898281161813583768\",\"options\":{\"foreign_keys\":\"(summary_id) references otel.metric_summary(summary_id) as fk_summary_resource_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
var showTableMetricSummaryScopeAttributesResponse = "\x04OK\x00&show_table_response\xf8\x1cFotel.metric_summary_scope_attribute\x02Fotel.metric_summary_scope_attribute\x00\x02\x00\x00\x02$474749891710437496\x00\x02\xe2\x06{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"summary_id\",\"type\":\"string\"},{\"name\":\"name\",\"type\":[\"string\",\"null\"]},{\"name\":\"version\",\"type\":[\"string\",\"null\"]},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x12\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18double_value\x04\x08data\x10nullable\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x08name\x06\x08data\x0echar256\x10nullable\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x14summary_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x0eversion\x06\x08data\x0echar256\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xaa\x01(summary_id) references otel.metric_summary(summary_id) as fk_summary_scope_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06925\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xa8\x04{\"table_name\":\"otel.metric_summary_scope_attribute\",\"type_id\":\"474749891710437496\",\"options\":{\"foreign_keys\":\"(summary_id) references otel.metric_summary(summary_id) as fk_summary_scope_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"

func TestExporter_pushMetricsData(t *testing.T) {
	t.Run("push success", func(t *testing.T) {
		exporter := newTestMetricsExporter(t)
		mustPushMetricsData(t, exporter, simpleMetrics(3))

		require.Equal(t, 15, 15)
	})
}

func Benchmark_pushMetricsData(b *testing.B) {
	pm := simpleMetrics(1)
	exporter := newTestMetricsExporter(&testing.T{})
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err := exporter.pushMetricsData(context.TODO(), pm)
		require.NoError(b, err)
	}
}

// simpleMetrics there will be added two ResourceMetrics and each of them have count data point
func simpleMetrics(count int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "demo 1")
	rm.Resource().Attributes().PutStr("Resource Attributes 1", "value1")
	rm.Resource().SetDroppedAttributesCount(10)
	rm.SetSchemaUrl("Resource SchemaUrl 1")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("Scope SchemaUrl 1")
	sm.Scope().Attributes().PutStr("Scope Attributes 1", "value1")
	sm.Scope().SetDroppedAttributesCount(10)
	sm.Scope().SetName("Scope name 1")
	sm.Scope().SetVersion("Scope version 1")
	for i := 0; i < count; i++ {
		// gauge
		m := sm.Metrics().AppendEmpty()
		m.SetName("gauge metrics")
		m.SetUnit("count")
		m.SetDescription("This is a gauge metrics")
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("gauge_label_1", "1")
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		exemplars := dp.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// sum
		m = sm.Metrics().AppendEmpty()
		m.SetName("sum metrics")
		m.SetUnit("count")
		m.SetDescription("This is a sum metrics")
		dp = m.SetEmptySum().DataPoints().AppendEmpty()
		dp.SetDoubleValue(11.234)
		dp.Attributes().PutStr("sum_label_1", "1")
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		exemplars = dp.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a histogram metrics")
		dpHisto := m.SetEmptyHistogram().DataPoints().AppendEmpty()
		dpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpHisto.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpHisto.SetCount(1)
		dpHisto.SetSum(1)
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.ExplicitBounds().FromRaw([]float64{0, 0, 0, 0, 0})
		dpHisto.BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpHisto.SetMin(0)
		dpHisto.SetMax(1)
		exemplars = dpHisto.Exemplars().AppendEmpty()
		exemplars.SetDoubleValue(55.22)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// exp histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("exp histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a exp histogram metrics")
		dpExpHisto := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
		dpExpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpExpHisto.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpExpHisto.SetSum(1)
		dpExpHisto.SetMin(0)
		dpExpHisto.SetMax(1)
		dpExpHisto.SetZeroCount(0)
		dpExpHisto.SetCount(1)
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Negative().SetOffset(1)
		dpExpHisto.Negative().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpExpHisto.Positive().SetOffset(1)
		dpExpHisto.Positive().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})

		exemplars = dpExpHisto.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// summary
		m = sm.Metrics().AppendEmpty()
		m.SetName("summary metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a summary metrics")
		summary := m.SetEmptySummary().DataPoints().AppendEmpty()
		summary.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		summary.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		summary.Attributes().PutStr("key", "value")
		summary.Attributes().PutStr("key2", "value2")
		summary.SetCount(1)
		summary.SetSum(1)
		quantileValues := summary.QuantileValues().AppendEmpty()
		quantileValues.SetValue(1)
		quantileValues.SetQuantile(1)
	}

	rm = metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "demo 2")
	rm.Resource().Attributes().PutStr("Resource Attributes 2", "value2")
	rm.Resource().SetDroppedAttributesCount(20)
	rm.SetSchemaUrl("Resource SchemaUrl 2")
	sm = rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("Scope SchemaUrl 2")
	sm.Scope().Attributes().PutStr("Scope Attributes 2", "value2")
	sm.Scope().SetDroppedAttributesCount(20)
	sm.Scope().SetName("Scope name 2")
	sm.Scope().SetVersion("Scope version 2")
	for i := 0; i < count; i++ {
		// gauge
		m := sm.Metrics().AppendEmpty()
		m.SetName("gauge metrics")
		m.SetUnit("count")
		m.SetDescription("This is a gauge metrics")
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("gauge_label_2", "2")
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		exemplars := dp.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// sum
		m = sm.Metrics().AppendEmpty()
		m.SetName("sum metrics")
		m.SetUnit("count")
		m.SetDescription("This is a sum metrics")
		dp = m.SetEmptySum().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("sum_label_2", "2")
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		exemplars = dp.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a histogram metrics")
		dpHisto := m.SetEmptyHistogram().DataPoints().AppendEmpty()
		dpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpHisto.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpHisto.SetCount(1)
		dpHisto.SetSum(1)
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.ExplicitBounds().FromRaw([]float64{0, 0, 0, 0, 0})
		dpHisto.BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpHisto.SetMin(0)
		dpHisto.SetMax(1)
		exemplars = dpHisto.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// exp histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("exp histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a exp histogram metrics")
		dpExpHisto := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
		dpExpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpExpHisto.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpExpHisto.SetSum(1)
		dpExpHisto.SetMin(0)
		dpExpHisto.SetMax(1)
		dpExpHisto.SetZeroCount(0)
		dpExpHisto.SetCount(1)
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Negative().SetOffset(1)
		dpExpHisto.Negative().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpExpHisto.Positive().SetOffset(1)
		dpExpHisto.Positive().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})

		exemplars = dpExpHisto.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// summary
		m = sm.Metrics().AppendEmpty()
		m.SetName("summary histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a summary metrics")
		summary := m.SetEmptySummary().DataPoints().AppendEmpty()
		summary.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		summary.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		summary.Attributes().PutStr("key", "value")
		summary.Attributes().PutStr("key2", "value2")
		summary.SetCount(1)
		summary.SetSum(1)
		quantileValues := summary.QuantileValues().AppendEmpty()
		quantileValues.SetValue(1)
		quantileValues.SetQuantile(1)
	}

	// add a different scope metrics
	sm = rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("Scope SchemaUrl 3")
	sm.Scope().Attributes().PutStr("Scope Attributes 3", "value3")
	sm.Scope().SetDroppedAttributesCount(20)
	sm.Scope().SetName("Scope name 3")
	sm.Scope().SetVersion("Scope version 3")
	for i := 0; i < count; i++ {
		// gauge
		m := sm.Metrics().AppendEmpty()
		m.SetName("gauge metrics")
		m.SetUnit("count")
		m.SetDescription("This is a gauge metrics")
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("gauge_label_3", "3")
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		exemplars := dp.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// sum
		m = sm.Metrics().AppendEmpty()
		m.SetName("sum metrics")
		m.SetUnit("count")
		m.SetDescription("This is a sum metrics")
		dp = m.SetEmptySum().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("sum_label_2", "2")
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		exemplars = dp.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a histogram metrics")
		dpHisto := m.SetEmptyHistogram().DataPoints().AppendEmpty()
		dpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpHisto.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpHisto.SetCount(1)
		dpHisto.SetSum(1)
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.ExplicitBounds().FromRaw([]float64{0, 0, 0, 0, 0})
		dpHisto.BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpHisto.SetMin(0)
		dpHisto.SetMax(1)
		exemplars = dpHisto.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// exp histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("exp histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a exp histogram metrics")
		dpExpHisto := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
		dpExpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpExpHisto.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpExpHisto.SetSum(1)
		dpExpHisto.SetMin(0)
		dpExpHisto.SetMax(1)
		dpExpHisto.SetZeroCount(0)
		dpExpHisto.SetCount(1)
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Negative().SetOffset(1)
		dpExpHisto.Negative().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpExpHisto.Positive().SetOffset(1)
		dpExpHisto.Positive().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})

		exemplars = dpExpHisto.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// summary
		m = sm.Metrics().AppendEmpty()
		m.SetName("summary histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a summary metrics")
		summary := m.SetEmptySummary().DataPoints().AppendEmpty()
		summary.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		summary.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		summary.Attributes().PutStr("key", "value")
		summary.Attributes().PutStr("key2", "value2")
		summary.SetCount(1)
		summary.SetSum(1)
		quantileValues := summary.QuantileValues().AppendEmpty()
		quantileValues.SetValue(1)
		quantileValues.SetQuantile(1)
	}
	return metrics
}

func mustPushMetricsData(t *testing.T, exporter *kineticaMetricsExporter, md pmetric.Metrics) {
	err := exporter.pushMetricsData(context.TODO(), md)
	require.NoError(t, err)
}

func newTestMetricsExporter(t *testing.T) *kineticaMetricsExporter {
	exporter := newMetricsExporter(zaptest.NewLogger(t), withTestExporterConfig()(baseURL))
	require.NoError(t, exporter.start(context.TODO(), nil))

	t.Cleanup(func() { _ = exporter.shutdown(context.TODO()) })
	return exporter
}

func withTestExporterConfig(fns ...func(*Config)) func(string) *Config {
	return func(endpoint string) *Config {
		var configMods []func(*Config)
		configMods = append(configMods, func(cfg *Config) {
			cfg.Host = endpoint
		})
		configMods = append(configMods, fns...)
		return withDefaultConfig(configMods...)
	}
}

// Handler functions for different endpoints

func handleInsertRecords(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Custom handling for endpoint 2 POST request
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("\x04OK\x00.insert_records_response\x08\x00\x06\x00\x00\x00"))
	if err != nil {
		http.Error(w, "Error wrting reesponse", http.StatusInternalServerError)
		return
	}
}

func handleExecuteSQL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	responseBytes := []byte("\x04OK\x00(execute_sql_response\xd4\x05\x02\xf6\x03{\"type\":\"record\",\"name\":\"generic_response\",\"fields\":[{\"name\":\"column_1\",\"type\":{\"type\":\"array\",\"items\":\"string\"}},{\"name\":\"column_headers\",\"type\":{\"type\":\"array\",\"items\":\"string\"}},{\"name\":\"column_datatypes\",\"type\":{\"type\":\"array\",\"items\":\"string\"}}]}$\x00\x02\ndummy\x00\x02\fstring\x00\x00\x01\x00\x00\b X-Kinetica-Group\x06DDL\ncount\x020\x1alast_endpoint\x1a/create/table.total_number_of_records\x020\x00\x00")
	_, err := w.Write(responseBytes)
	if err != nil {
		http.Error(w, "Error wrting reesponse", http.StatusInternalServerError)
		return
	}
}

func handleShowTable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	requestBody := string(body)

	// Print the request body
	// fmt.Println("Request Body:", requestBody)

	w.Header().Set("Content-Type", "application/octet-stream")

	finalResponseBytes := getShowTableResponse(requestBody)

	// fmt.Println("Response bytes => ", string(finalResponseBytes))

	_, err = w.Write(finalResponseBytes)
	if err != nil {
		http.Error(w, "Error wrting reesponse", http.StatusInternalServerError)
		return
	}

}

func getShowTableResponse(requestBody string) []byte {
	switch {
	case strings.Contains(requestBody, "metric_summary_scope_attribute"):
		return []byte(showTableMetricSummaryScopeAttributesResponse)
	case strings.Contains(requestBody, "metric_summary_resource_attribute"):
		return []byte(showTableMetricSummaryResourceAttributesResponse)
	case strings.Contains(requestBody, "metric_summary_datapoint_quantile_values"):
		return []byte(showTableMetricSummaryDatapointQuantileValuesResponse)
	case strings.Contains(requestBody, "metric_summary_datapoint_attribute"):
		return []byte(showTableMetricSummaryDatapointAttributeResponse)
	case strings.Contains(requestBody, "metric_summary_datapoint"):
		return []byte(showTableMetricSummaryDatapointResponse)
	case strings.Contains(requestBody, "metric_summary"):
		return []byte(showTableMetricSummaryResponse)
	default:
		return []byte("")
	}

}

// Setup function (runs before tests start)
func TestMain(m *testing.M) {
	// Create a test server with a simple handler function
	testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("In main ...")
		switch r.URL.Path {
		case "/insert/records":
			handleInsertRecords(w, r)
		case "/show/table":
			handleShowTable(w, r)
		case "/execute/sql":
			handleExecuteSQL(w, r)
		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	}))

	baseURL = testServer.URL // Store the base URL for test requests

	// Run the tests
	code := m.Run()

	// Teardown: Close the test server after all tests are finished
	testServer.Close()
	// Perform any other teardown operations here if needed

	// Exit with the test status code
	// This allows TestMain to report the result of the tests
	// You can also perform further actions based on the test results
	os.Exit(code)
}
