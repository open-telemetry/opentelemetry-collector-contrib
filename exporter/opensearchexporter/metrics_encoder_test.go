// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func testMetricEncodeModel() *encodeModel {
	return &encodeModel{
		sso:       true,
		dataset:   "default",
		namespace: "namespace",
	}
}

func testMetricResourceAndScope() (pcommon.Resource, pcommon.InstrumentationScope) {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	resource := rm.Resource()
	resource.Attributes().PutStr("service.name", "test-service")
	resource.Attributes().PutStr("host.name", "test-host")

	sm := rm.ScopeMetrics().AppendEmpty()
	scope := sm.Scope()
	scope.SetName("test-scope")
	scope.SetVersion("1.0.0")

	return resource, scope
}

func encodeToMap(t *testing.T, payload []byte, err error) map[string]any {
	t.Helper()
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(payload, &doc))
	return doc
}

func TestEncodeMetric_Gauge(t *testing.T) {
	model := testMetricEncodeModel()
	resource, scope := testMetricResourceAndScope()

	metric := pmetric.NewMetric()
	metric.SetName("system.cpu.usage")
	metric.SetDescription("CPU usage")
	metric.SetUnit("1")
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1700000000, 0)))
	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1699999000, 0)))
	dp.SetDoubleValue(0.42)
	dp.Attributes().PutStr("cpu", "cpu0")

	payload, err := model.encodeMetric(resource, scope, "schema-url", metric, dp)
	doc := encodeToMap(t, payload, err)

	assert.Equal(t, "system.cpu.usage", doc["name"])
	assert.Equal(t, "CPU usage", doc["description"])
	assert.Equal(t, "1", doc["unit"])
	assert.Equal(t, "GAUGE", doc["kind"])
	assert.Equal(t, 0.42, doc["value@double"])
	assert.NotContains(t, doc, "value@int")
	assert.NotContains(t, doc, "monotonic")
	assert.Equal(t, "test-service", doc["serviceName"])
	assert.Equal(t, "schema-url", doc["schemaUrl"])

	attrs, ok := doc["attributes"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "cpu0", attrs["cpu"])

	ds, ok := attrs["data_stream"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "metric", ds["type"])
	assert.Equal(t, "default", ds["dataset"])
	assert.Equal(t, "namespace", ds["namespace"])

	res, ok := doc["resource"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "test-service", res["service.name"])

	is, ok := doc["instrumentationScope"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "test-scope", is["name"])
	assert.Equal(t, "1.0.0", is["version"])
}

func TestEncodeMetric_GaugeIntValue(t *testing.T) {
	model := testMetricEncodeModel()
	resource, scope := testMetricResourceAndScope()

	metric := pmetric.NewMetric()
	metric.SetName("queue.size")
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetIntValue(17)

	payload, err := model.encodeMetric(resource, scope, "", metric, dp)
	doc := encodeToMap(t, payload, err)

	assert.Equal(t, float64(17), doc["value@int"])
	assert.NotContains(t, doc, "value@double")
}

func TestEncodeMetric_Sum(t *testing.T) {
	model := testMetricEncodeModel()
	resource, scope := testMetricResourceAndScope()

	metric := pmetric.NewMetric()
	metric.SetName("http.requests")
	sum := metric.SetEmptySum()
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	sum.SetIsMonotonic(true)
	dp := sum.DataPoints().AppendEmpty()
	dp.SetIntValue(1234)

	payload, err := model.encodeMetric(resource, scope, "", metric, dp)
	doc := encodeToMap(t, payload, err)

	assert.Equal(t, "SUM", doc["kind"])
	assert.Equal(t, "AGGREGATION_TEMPORALITY_CUMULATIVE", doc["aggregationTemporality"])
	assert.Equal(t, true, doc["monotonic"])
	assert.Equal(t, float64(1234), doc["value@int"])
}

func TestEncodeMetric_Histogram(t *testing.T) {
	model := testMetricEncodeModel()
	resource, scope := testMetricResourceAndScope()

	metric := pmetric.NewMetric()
	metric.SetName("http.request.duration")
	histogram := metric.SetEmptyHistogram()
	histogram.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	dp := histogram.DataPoints().AppendEmpty()
	dp.SetCount(10)
	dp.SetSum(55.5)
	dp.SetMin(1)
	dp.SetMax(20)
	dp.ExplicitBounds().FromRaw([]float64{5, 10})
	dp.BucketCounts().FromRaw([]uint64{2, 5, 3})

	payload, err := model.encodeMetric(resource, scope, "", metric, dp)
	doc := encodeToMap(t, payload, err)

	assert.Equal(t, "HISTOGRAM", doc["kind"])
	assert.Equal(t, "AGGREGATION_TEMPORALITY_DELTA", doc["aggregationTemporality"])
	assert.Equal(t, float64(10), doc["count"])
	assert.Equal(t, 55.5, doc["sum"])
	assert.Equal(t, float64(1), doc["min"])
	assert.Equal(t, float64(20), doc["max"])
	assert.Equal(t, float64(3), doc["bucketCount"])
	assert.Equal(t, float64(2), doc["explicitBoundsCount"])
	assert.Equal(t, []any{float64(2), float64(5), float64(3)}, doc["bucketCountsList"])
	assert.Equal(t, []any{float64(5), float64(10)}, doc["explicitBoundsList"])

	buckets, ok := doc["buckets"].([]any)
	require.True(t, ok)
	require.Len(t, buckets, 3)
	middle := buckets[1].(map[string]any)
	assert.Equal(t, float64(5), middle["min"])
	assert.Equal(t, float64(10), middle["max"])
	assert.Equal(t, float64(5), middle["count"])
}

func TestEncodeMetric_ExponentialHistogram(t *testing.T) {
	model := testMetricEncodeModel()
	resource, scope := testMetricResourceAndScope()

	metric := pmetric.NewMetric()
	metric.SetName("latency")
	histogram := metric.SetEmptyExponentialHistogram()
	histogram.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	dp := histogram.DataPoints().AppendEmpty()
	dp.SetCount(6)
	dp.SetSum(30)
	dp.SetZeroCount(1)
	dp.SetScale(0) // base = 2
	dp.Positive().SetOffset(0)
	dp.Positive().BucketCounts().FromRaw([]uint64{1, 2, 3})

	payload, err := model.encodeMetric(resource, scope, "", metric, dp)
	doc := encodeToMap(t, payload, err)

	assert.Equal(t, "EXPONENTIAL_HISTOGRAM", doc["kind"])
	assert.Equal(t, float64(0), doc["scale"])
	assert.Equal(t, float64(1), doc["zeroCount"])
	assert.Equal(t, float64(0), doc["positiveOffset"])

	buckets, ok := doc["positiveBuckets"].([]any)
	require.True(t, ok)
	require.Len(t, buckets, 3)
	// scale 0 -> base 2: buckets (1,2], (2,4], (4,8]
	first := buckets[0].(map[string]any)
	assert.Equal(t, float64(1), first["min"])
	assert.Equal(t, float64(2), first["max"])
	third := buckets[2].(map[string]any)
	assert.Equal(t, float64(4), third["min"])
	assert.Equal(t, float64(8), third["max"])
	assert.Equal(t, float64(3), third["count"])
	assert.NotContains(t, doc, "negativeBuckets")
}

func TestEncodeMetric_Summary(t *testing.T) {
	model := testMetricEncodeModel()
	resource, scope := testMetricResourceAndScope()

	metric := pmetric.NewMetric()
	metric.SetName("gc.duration")
	dp := metric.SetEmptySummary().DataPoints().AppendEmpty()
	dp.SetCount(100)
	dp.SetSum(123.4)
	q := dp.QuantileValues().AppendEmpty()
	q.SetQuantile(0.5)
	q.SetValue(1.2)
	q = dp.QuantileValues().AppendEmpty()
	q.SetQuantile(0.99)
	q.SetValue(9.9)

	payload, err := model.encodeMetric(resource, scope, "", metric, dp)
	doc := encodeToMap(t, payload, err)

	assert.Equal(t, "SUMMARY", doc["kind"])
	assert.Equal(t, float64(100), doc["count"])
	assert.Equal(t, 123.4, doc["sum"])
	assert.Equal(t, float64(2), doc["quantileValuesCount"])

	quantiles, ok := doc["quantiles"].([]any)
	require.True(t, ok)
	require.Len(t, quantiles, 2)
	p99 := quantiles[1].(map[string]any)
	assert.Equal(t, 0.99, p99["quantile"])
	assert.Equal(t, 9.9, p99["value"])
}

func TestEncodeMetric_Exemplars(t *testing.T) {
	model := testMetricEncodeModel()
	resource, scope := testMetricResourceAndScope()

	metric := pmetric.NewMetric()
	metric.SetName("with.exemplar")
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(1)
	exemplar := dp.Exemplars().AppendEmpty()
	exemplar.SetDoubleValue(2.5)
	exemplar.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1700000000, 0)))
	exemplar.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	exemplar.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	exemplar.FilteredAttributes().PutStr("sampled", "true")

	payload, err := model.encodeMetric(resource, scope, "", metric, dp)
	doc := encodeToMap(t, payload, err)

	exemplars, ok := doc["exemplars"].([]any)
	require.True(t, ok)
	require.Len(t, exemplars, 1)
	e := exemplars[0].(map[string]any)
	assert.Equal(t, 2.5, e["value"])
	assert.Equal(t, "0102030405060708090a0b0c0d0e0f10", e["traceId"])
	assert.Equal(t, "0102030405060708", e["spanId"])
}

func TestEncodeMetric_OTelV1(t *testing.T) {
	model := &encodeModel{otelV1: true}
	resource, scope := testMetricResourceAndScope()

	metric := pmetric.NewMetric()
	metric.SetName("http.requests")
	metric.SetUnit("1")
	sum := metric.SetEmptySum()
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	sum.SetIsMonotonic(true)
	dp := sum.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1700000000, 0)))
	dp.SetIntValue(42)

	payload, err := model.encodeMetric(resource, scope, "schema-url", metric, dp)
	doc := encodeToMap(t, payload, err)

	assert.Equal(t, "SUM", doc["kind"])
	// Data Prepper emits a plain double `value` only; the SS4O-style typed
	// value@int / value@double fields must not appear in otel-v1 documents.
	assert.Equal(t, float64(42), doc["value"])
	assert.NotContains(t, doc, "value@int")
	assert.NotContains(t, doc, "value@double")
	assert.Contains(t, doc, "time")
	assert.Contains(t, doc, "flags")
	assert.Equal(t, "test-service", doc["serviceName"])

	res, ok := doc["resource"].(map[string]any)
	require.True(t, ok)
	resAttrs, ok := res["attributes"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "test-service", resAttrs["service.name"])

	is, ok := doc["instrumentationScope"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "test-scope", is["name"])
}

func TestEncodeMetric_UnsupportedMode(t *testing.T) {
	model := &encodeModel{} // neither sso nor otel-v1, e.g. ecs
	resource, scope := testMetricResourceAndScope()

	metric := pmetric.NewMetric()
	metric.SetName("m")
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()

	_, err := model.encodeMetric(resource, scope, "", metric, dp)
	assert.ErrorIs(t, err, errMetricsMappingModeUnsupported)
}

func TestEncodeMetric_BodyMapUnsupported(t *testing.T) {
	model := &bodyMapMappingModel{}
	resource, scope := testMetricResourceAndScope()

	metric := pmetric.NewMetric()
	metric.SetName("m")
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()

	_, err := model.encodeMetric(resource, scope, "", metric, dp)
	assert.Error(t, err)
}

func TestMakeExplicitBuckets(t *testing.T) {
	buckets := makeExplicitBuckets([]uint64{1, 2, 3}, []float64{10, 20})
	require.Len(t, buckets, 3)
	assert.Equal(t, float64(10), buckets[0].Max)
	assert.Equal(t, float64(10), buckets[1].Min)
	assert.Equal(t, float64(20), buckets[1].Max)
	assert.Equal(t, float64(20), buckets[2].Min)
	assert.Equal(t, uint64(3), buckets[2].Count)

	// Unbounded edges must stay within the float32 range required by the
	// SS4O and Data Prepper index templates.
	assert.Equal(t, -float64(math.MaxFloat32), buckets[0].Min)
	assert.Equal(t, float64(math.MaxFloat32), buckets[2].Max)

	assert.Nil(t, makeExplicitBuckets(nil, nil))
}

func TestClampToFloat32(t *testing.T) {
	assert.Equal(t, float64(math.MaxFloat32), clampToFloat32(math.MaxFloat64))
	assert.Equal(t, -float64(math.MaxFloat32), clampToFloat32(-math.MaxFloat64))
	assert.Equal(t, float64(math.MaxFloat32), clampToFloat32(math.Inf(1)))
	assert.Equal(t, -float64(math.MaxFloat32), clampToFloat32(math.Inf(-1)))
	assert.Equal(t, float64(0), clampToFloat32(math.NaN()))
	assert.Equal(t, 42.5, clampToFloat32(42.5))
}

func TestEncodeMetric_GaugeNaNValue(t *testing.T) {
	resource, scope := testMetricResourceAndScope()

	metric := pmetric.NewMetric()
	metric.SetName("stale.gauge")
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(math.NaN())

	// A NaN point value (e.g. a Prometheus staleness marker) must not drop
	// the document; the value fields are omitted instead.
	payload, err := testMetricEncodeModel().encodeMetric(resource, scope, "", metric, dp)
	doc := encodeToMap(t, payload, err)
	assert.Equal(t, "GAUGE", doc["kind"])
	assert.NotContains(t, doc, "value@double")
	assert.NotContains(t, doc, "value@int")

	otelV1 := &encodeModel{otelV1: true}
	payload, err = otelV1.encodeMetric(resource, scope, "", metric, dp)
	doc = encodeToMap(t, payload, err)
	assert.Equal(t, "GAUGE", doc["kind"])
	assert.NotContains(t, doc, "value")
	assert.NotContains(t, doc, "value@double")
	assert.NotContains(t, doc, "value@int")
}

func TestEncodeMetric_GaugeInfValue(t *testing.T) {
	model := testMetricEncodeModel()
	resource, scope := testMetricResourceAndScope()

	metric := pmetric.NewMetric()
	metric.SetName("overflow.gauge")
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(math.Inf(1))

	payload, err := model.encodeMetric(resource, scope, "", metric, dp)
	doc := encodeToMap(t, payload, err)
	assert.Equal(t, math.MaxFloat64, doc["value@double"])

	dp.SetDoubleValue(math.Inf(-1))
	payload, err = model.encodeMetric(resource, scope, "", metric, dp)
	doc = encodeToMap(t, payload, err)
	assert.Equal(t, -math.MaxFloat64, doc["value@double"])
}

func TestEncodeMetric_HistogramNonFiniteBounds(t *testing.T) {
	model := testMetricEncodeModel()
	resource, scope := testMetricResourceAndScope()

	metric := pmetric.NewMetric()
	metric.SetName("weird.histogram")
	histogram := metric.SetEmptyHistogram()
	histogram.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	dp := histogram.DataPoints().AppendEmpty()
	dp.SetCount(6)
	dp.SetSum(math.NaN())
	dp.SetMin(math.Inf(-1))
	dp.SetMax(math.Inf(1))
	dp.ExplicitBounds().FromRaw([]float64{math.NaN(), math.Inf(1)})
	dp.BucketCounts().FromRaw([]uint64{1, 2, 3})

	payload, err := model.encodeMetric(resource, scope, "", metric, dp)
	doc := encodeToMap(t, payload, err)

	// Non-finite bounds are sanitized (NaN -> 0, +Inf -> MaxFloat32) instead
	// of failing the JSON encoding and dropping the document.
	assert.Equal(t, []any{float64(0), float64(math.MaxFloat32)}, doc["explicitBoundsList"])
	assert.NotContains(t, doc, "sum")
	assert.Equal(t, -math.MaxFloat64, doc["min"])
	assert.Equal(t, math.MaxFloat64, doc["max"])

	buckets, ok := doc["buckets"].([]any)
	require.True(t, ok)
	require.Len(t, buckets, 3)
	first := buckets[0].(map[string]any)
	assert.Equal(t, float64(0), first["max"])
	last := buckets[2].(map[string]any)
	assert.Equal(t, float64(math.MaxFloat32), last["min"])
	assert.Equal(t, float64(math.MaxFloat32), last["max"])
}

func TestEncodeMetric_SummaryNaNQuantile(t *testing.T) {
	model := testMetricEncodeModel()
	resource, scope := testMetricResourceAndScope()

	metric := pmetric.NewMetric()
	metric.SetName("empty.summary")
	dp := metric.SetEmptySummary().DataPoints().AppendEmpty()
	dp.SetCount(0)
	dp.SetSum(math.NaN())
	q := dp.QuantileValues().AppendEmpty()
	q.SetQuantile(0.5)
	q.SetValue(math.NaN()) // quantile of an empty summary
	q = dp.QuantileValues().AppendEmpty()
	q.SetQuantile(0.99)
	q.SetValue(7.7)

	payload, err := model.encodeMetric(resource, scope, "", metric, dp)
	doc := encodeToMap(t, payload, err)

	assert.NotContains(t, doc, "sum")
	assert.Equal(t, float64(1), doc["quantileValuesCount"])
	quantiles, ok := doc["quantiles"].([]any)
	require.True(t, ok)
	require.Len(t, quantiles, 1)
	kept := quantiles[0].(map[string]any)
	assert.Equal(t, 0.99, kept["quantile"])
	assert.Equal(t, 7.7, kept["value"])
}

func TestEncodeMetric_ExemplarNaNValue(t *testing.T) {
	model := testMetricEncodeModel()
	resource, scope := testMetricResourceAndScope()

	metric := pmetric.NewMetric()
	metric.SetName("with.nan.exemplar")
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(1)
	exemplar := dp.Exemplars().AppendEmpty()
	exemplar.SetDoubleValue(math.NaN())
	exemplar.SetTraceID([16]byte{1})

	payload, err := model.encodeMetric(resource, scope, "", metric, dp)
	doc := encodeToMap(t, payload, err)

	exemplars, ok := doc["exemplars"].([]any)
	require.True(t, ok)
	require.Len(t, exemplars, 1)
	e := exemplars[0].(map[string]any)
	assert.NotContains(t, e, "value")
	assert.Equal(t, "01000000000000000000000000000000", e["traceId"])
}

func TestTemporalityString(t *testing.T) {
	assert.Equal(t, "AGGREGATION_TEMPORALITY_DELTA", temporalityString(pmetric.AggregationTemporalityDelta))
	assert.Equal(t, "AGGREGATION_TEMPORALITY_CUMULATIVE", temporalityString(pmetric.AggregationTemporalityCumulative))
	assert.Equal(t, "AGGREGATION_TEMPORALITY_UNSPECIFIED", temporalityString(pmetric.AggregationTemporalityUnspecified))
}
