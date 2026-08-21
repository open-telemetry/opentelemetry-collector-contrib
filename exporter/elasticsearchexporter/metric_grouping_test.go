// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metadata"
)

// newGroupingMetricsExporter builds a factory-wired metrics exporter whose
// batcher deterministically merges everything sent within the flush window:
// min_size is set high enough that only the flush timeout triggers a flush, so
// payloads from separate ConsumeMetrics calls are always batched together.
func newGroupingMetricsExporter(t *testing.T) (*bulkRecorder, func(...pmetric.Metrics)) {
	rec := newBulkRecorder()
	server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
		rec.Record(docs)
		return itemsAllOK(docs)
	})

	cfg := withDefaultConfig(func(cfg *Config) {
		cfg.Endpoints = []string{server.URL}
		qc := cfg.QueueBatchConfig.Get()
		qc.NumConsumers = 1
		batch := qc.Batch.Get()
		batch.FlushTimeout = 100 * time.Millisecond
		batch.MinSize = 1e8
		batch.MaxSize = 2e8
	})

	exp, err := NewFactory().CreateMetrics(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, exp.Shutdown(context.WithoutCancel(t.Context()))) })

	return rec, func(payloads ...pmetric.Metrics) {
		for _, m := range payloads {
			require.NoError(t, exp.ConsumeMetrics(t.Context(), m))
		}
	}
}

// groupingGauges builds a metrics payload with one resource/scope and one gauge
// data point per name, all sharing the same timestamp and attributes so they
// belong to a single document group. mode optionally sets the scope mapping
// mode attribute (e.g. "ecs"); empty means the default (otel).
func groupingGauges(mode string, ts time.Time, names []string, values []float64) pmetric.Metrics {
	m := pmetric.NewMetrics()
	rm := m.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "grouping-test")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("grouping-scope")
	if mode != "" {
		sm.Scope().Attributes().PutStr(elasticsearch.MappingModeAttributeName, mode)
	}
	for i, name := range names {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName(name)
		dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		dp.SetDoubleValue(values[i])
		dp.Attributes().PutStr("host.name", "host-1")
	}
	return m
}

func decodeGroupingDocs(t *testing.T, rec *bulkRecorder, n int) []map[string]any {
	items := rec.WaitItems(n)
	require.Len(t, items, n)
	docs := make([]map[string]any, n)
	for i, item := range items {
		require.NoError(t, json.Unmarshal(item.Document, &docs[i]))
	}
	return docs
}

// TestMetricGroupingAcrossBatchedPayloads pins batch-wide document grouping:
// data points that share a document identity (resource, scope, attributes and
// timestamp) but arrive in separate ConsumeMetrics calls within one batch must
// be grouped into a single document — identical, including its
// _metric_names_hash (the document's TSDB identity), to the document produced
// when the same data points arrive in one payload. Splitting a group into
// multiple documents per timestamp+dimensions would collide in TSDB metrics
// data streams.
func TestMetricGroupingAcrossBatchedPayloads(t *testing.T) {
	ts := time.Unix(1719000000, 0).UTC()

	recFull, sendFull := newGroupingMetricsExporter(t)
	sendFull(groupingGauges("", ts, []string{"m.a", "m.b"}, []float64{1.5, 2.5}))
	fullDoc := decodeGroupingDocs(t, recFull, 1)[0]

	recSplit, sendSplit := newGroupingMetricsExporter(t)
	sendSplit(
		groupingGauges("", ts, []string{"m.a"}, []float64{1.5}),
		groupingGauges("", ts, []string{"m.b"}, []float64{2.5}),
	)
	splitDoc := decodeGroupingDocs(t, recSplit, 1)[0]

	require.Equal(t, fullDoc, splitDoc)
	require.NotEmpty(t, fullDoc["_metric_names_hash"])
	require.Equal(t, map[string]any{"m.a": 1.5, "m.b": 2.5}, splitDoc["metrics"])
}

// TestMetricGroupingAcrossBatchedPayloads_ECSMode is the same batch-wide
// grouping property for scopes routed to the ECS mapping mode via the scope
// attribute.
func TestMetricGroupingAcrossBatchedPayloads_ECSMode(t *testing.T) {
	ts := time.Unix(1719000000, 0).UTC()

	recFull, sendFull := newGroupingMetricsExporter(t)
	sendFull(groupingGauges("ecs", ts, []string{"metric.a", "metric.b"}, []float64{1.5, 2.5}))
	fullDoc := decodeGroupingDocs(t, recFull, 1)[0]

	recSplit, sendSplit := newGroupingMetricsExporter(t)
	sendSplit(
		groupingGauges("ecs", ts, []string{"metric.a"}, []float64{1.5}),
		groupingGauges("ecs", ts, []string{"metric.b"}, []float64{2.5}),
	)
	splitDoc := decodeGroupingDocs(t, recSplit, 1)[0]

	require.Equal(t, fullDoc, splitDoc)
	// The ECS serializer de-dots metric names into nested objects.
	require.Equal(t, map[string]any{"a": 1.5, "b": 2.5}, splitDoc["metric"])
}

// TestMixedMappingModePayload pins per-scope mapping-mode routing: one payload
// containing an OTel-mode scope and an ECS-mode scope must produce exactly one
// document per scope, each in its mode's shape.
func TestMixedMappingModePayload(t *testing.T) {
	ts := time.Unix(1719000000, 0).UTC()
	rec, send := newGroupingMetricsExporter(t)

	m := groupingGauges("", ts, []string{"otel.metric"}, []float64{1})
	ecsSM := m.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty()
	ecsSM.Scope().SetName("ecs-scope")
	ecsSM.Scope().Attributes().PutStr(elasticsearch.MappingModeAttributeName, "ecs")
	metric := ecsSM.Metrics().AppendEmpty()
	metric.SetName("ecs.metric")
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
	dp.SetDoubleValue(2)

	send(m)
	docs := decodeGroupingDocs(t, rec, 2)

	var otelDocs, ecsDocs int
	for _, doc := range docs {
		if _, ok := doc["metrics"]; ok {
			otelDocs++
		} else {
			require.Equal(t, map[string]any{"metric": 2.0}, doc["ecs"])
			ecsDocs++
		}
	}
	require.Equal(t, 1, otelDocs)
	require.Equal(t, 1, ecsDocs)
}

// TestMetricNameSetsKeepDistinctTSDBIdentity pins the _metric_names_hash
// workaround for TSDB deduplication: documents that share a timestamp and
// dimensions but carry different metric-name sets (here because they were
// flushed in separate batches) must be stored with distinct
// _metric_names_hash values, so Elasticsearch keeps both.
func TestMetricNameSetsKeepDistinctTSDBIdentity(t *testing.T) {
	ts := time.Unix(1719000000, 0).UTC()
	rec, send := newGroupingMetricsExporter(t)

	send(groupingGauges("", ts, []string{"m.a"}, []float64{1}))
	rec.WaitItems(1) // force the second payload into its own batch
	send(groupingGauges("", ts, []string{"m.b"}, []float64{2}))
	docs := decodeGroupingDocs(t, rec, 2)

	hashes := make(map[string]int)
	for _, doc := range docs {
		hash, ok := doc["_metric_names_hash"].(string)
		require.True(t, ok)
		hashes[hash]++
	}
	require.Len(t, hashes, 2)
}
