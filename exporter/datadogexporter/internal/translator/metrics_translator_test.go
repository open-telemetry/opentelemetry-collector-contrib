// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package translator

import (
	"context"
	"math"
	"testing"
	"time"

	gocache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/attributes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"
)

func TestMetricValue(t *testing.T) {
	var (
		name  = "name"
		value = math.Pi
		ts    = uint64(time.Now().UnixNano())
		tags  = []string{"tool:opentelemetry", "version:0.1.0"}
	)

	metric := metrics.NewGauge(name, ts, value, tags)
	assert.Equal(t, string(metrics.Gauge), metric.GetType())
	assert.Equal(t, tags, metric.Tags)
}

func TestGetTags(t *testing.T) {
	attributes := pdata.NewAttributeMapFromMap(map[string]pdata.AttributeValue{
		"key1": pdata.NewAttributeValueString("val1"),
		"key2": pdata.NewAttributeValueString("val2"),
		"key3": pdata.NewAttributeValueString(""),
	})

	assert.ElementsMatch(t,
		getTags(attributes),
		[...]string{"key1:val1", "key2:val2", "key3:n/a"},
	)
}

func TestIsCumulativeMonotonic(t *testing.T) {
	// Some of these examples are from the hostmetrics receiver
	// and reflect the semantic meaning of the metrics there.
	//
	// If the receiver changes these examples should be added here too

	{ // Sum: Cumulative but not monotonic
		metric := pdata.NewMetric()
		metric.SetName("system.filesystem.usage")
		metric.SetDescription("Filesystem bytes used.")
		metric.SetUnit("bytes")
		metric.SetDataType(pdata.MetricDataTypeSum)
		sum := metric.Sum()
		sum.SetIsMonotonic(false)
		sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)

		assert.False(t, isCumulativeMonotonic(metric))
	}

	{ // Sum: Cumulative and monotonic
		metric := pdata.NewMetric()
		metric.SetName("system.network.packets")
		metric.SetDescription("The number of packets transferred.")
		metric.SetUnit("1")
		metric.SetDataType(pdata.MetricDataTypeSum)
		sum := metric.Sum()
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)

		assert.True(t, isCumulativeMonotonic(metric))
	}

	{ // DoubleSumL Cumulative and monotonic
		metric := pdata.NewMetric()
		metric.SetName("metric.example")
		metric.SetDataType(pdata.MetricDataTypeSum)
		sum := metric.Sum()
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)

		assert.True(t, isCumulativeMonotonic(metric))
	}

	{ // Not IntSum
		metric := pdata.NewMetric()
		metric.SetName("system.cpu.load_average.1m")
		metric.SetDescription("Average CPU Load over 1 minute.")
		metric.SetUnit("1")
		metric.SetDataType(pdata.MetricDataTypeGauge)

		assert.False(t, isCumulativeMonotonic(metric))
	}
}

type testProvider string

func (t testProvider) Hostname(context.Context) (string, error) {
	return string(t), nil
}

func newTranslator(t *testing.T, logger *zap.Logger) *Translator {
	params := component.ExporterCreateSettings{
		BuildInfo: component.BuildInfo{
			Version: "1.0",
		},
		TelemetrySettings: component.TelemetrySettings{
			Logger: logger,
		},
	}

	tr, err := New(
		params,
		WithFallbackHostnameProvider(testProvider("fallbackHostname")),
		WithCountSumMetrics(),
		WithHistogramMode(HistogramModeNoBuckets),
		WithNumberMode(NumberModeCumulativeToDelta),
	)

	require.NoError(t, err)
	return tr
}

func TestMapIntMetrics(t *testing.T) {
	ts := pdata.NewTimestampFromTime(time.Now())
	slice := pdata.NewNumberDataPointSlice()
	point := slice.AppendEmpty()
	point.SetIntVal(17)
	point.SetTimestamp(ts)
	tr := newTranslator(t, zap.NewNop())

	assert.ElementsMatch(t,
		tr.mapNumberMetrics("int64.test", metrics.Gauge, slice, []string{}),
		[]datadog.Metric{metrics.NewGauge("int64.test", uint64(ts), 17, []string{})},
	)

	assert.ElementsMatch(t,
		tr.mapNumberMetrics("int64.delta.test", metrics.Count, slice, []string{}),
		[]datadog.Metric{metrics.NewCount("int64.delta.test", uint64(ts), 17, []string{})},
	)

	// With attribute tags
	assert.ElementsMatch(t,
		tr.mapNumberMetrics("int64.test", metrics.Gauge, slice, []string{"attribute_tag:attribute_value"}),
		[]datadog.Metric{metrics.NewGauge("int64.test", uint64(ts), 17, []string{"attribute_tag:attribute_value"})},
	)
}

func TestMapDoubleMetrics(t *testing.T) {
	ts := pdata.NewTimestampFromTime(time.Now())
	slice := pdata.NewNumberDataPointSlice()
	point := slice.AppendEmpty()
	point.SetDoubleVal(math.Pi)
	point.SetTimestamp(ts)
	tr := newTranslator(t, zap.NewNop())

	assert.ElementsMatch(t,
		tr.mapNumberMetrics("float64.test", metrics.Gauge, slice, []string{}),
		[]datadog.Metric{metrics.NewGauge("float64.test", uint64(ts), math.Pi, []string{})},
	)

	assert.ElementsMatch(t,
		tr.mapNumberMetrics("float64.delta.test", metrics.Count, slice, []string{}),
		[]datadog.Metric{metrics.NewCount("float64.delta.test", uint64(ts), math.Pi, []string{})},
	)

	// With attribute tags
	assert.ElementsMatch(t,
		tr.mapNumberMetrics("float64.test", metrics.Gauge, slice, []string{"attribute_tag:attribute_value"}),
		[]datadog.Metric{metrics.NewGauge("float64.test", uint64(ts), math.Pi, []string{"attribute_tag:attribute_value"})},
	)
}

func seconds(i int) pdata.Timestamp {
	return pdata.NewTimestampFromTime(time.Unix(int64(i), 0))
}

func TestMapIntMonotonicMetrics(t *testing.T) {
	// Create list of values
	deltas := []int64{1, 2, 200, 3, 7, 0}
	cumulative := make([]int64, len(deltas)+1)
	cumulative[0] = 0
	for i := 1; i < len(cumulative); i++ {
		cumulative[i] = cumulative[i-1] + deltas[i-1]
	}

	//Map to OpenTelemetry format
	slice := pdata.NewNumberDataPointSlice()
	slice.EnsureCapacity(len(cumulative))
	for i, val := range cumulative {
		point := slice.AppendEmpty()
		point.SetIntVal(val)
		point.SetTimestamp(seconds(i))
	}

	// Map to Datadog format
	metricName := "metric.example"
	expected := make([]datadog.Metric, len(deltas))
	for i, val := range deltas {
		expected[i] = metrics.NewCount(metricName, uint64(seconds(i+1)), float64(val), []string{})
	}

	tr := newTranslator(t, zap.NewNop())
	output := tr.mapNumberMonotonicMetrics(metricName, slice, []string{})

	assert.ElementsMatch(t, output, expected)
}

func TestMapIntMonotonicDifferentDimensions(t *testing.T) {
	metricName := "metric.example"
	slice := pdata.NewNumberDataPointSlice()

	// No tags
	point := slice.AppendEmpty()
	point.SetTimestamp(seconds(0))

	point = slice.AppendEmpty()
	point.SetIntVal(20)
	point.SetTimestamp(seconds(1))

	// One tag: valA
	point = slice.AppendEmpty()
	point.SetTimestamp(seconds(0))
	point.Attributes().InsertString("key1", "valA")

	point = slice.AppendEmpty()
	point.SetIntVal(30)
	point.SetTimestamp(seconds(1))
	point.Attributes().InsertString("key1", "valA")

	// same tag: valB
	point = slice.AppendEmpty()
	point.SetTimestamp(seconds(0))
	point.Attributes().InsertString("key1", "valB")

	point = slice.AppendEmpty()
	point.SetIntVal(40)
	point.SetTimestamp(seconds(1))
	point.Attributes().InsertString("key1", "valB")

	tr := newTranslator(t, zap.NewNop())

	assert.ElementsMatch(t,
		tr.mapNumberMonotonicMetrics(metricName, slice, []string{}),
		[]datadog.Metric{
			metrics.NewCount(metricName, uint64(seconds(1)), 20, []string{}),
			metrics.NewCount(metricName, uint64(seconds(1)), 30, []string{"key1:valA"}),
			metrics.NewCount(metricName, uint64(seconds(1)), 40, []string{"key1:valB"}),
		},
	)
}

func TestMapIntMonotonicWithReboot(t *testing.T) {
	values := []int64{0, 30, 0, 20}
	metricName := "metric.example"
	slice := pdata.NewNumberDataPointSlice()
	slice.EnsureCapacity(len(values))

	for i, val := range values {
		point := slice.AppendEmpty()
		point.SetTimestamp(seconds(i))
		point.SetIntVal(val)
	}

	tr := newTranslator(t, zap.NewNop())
	assert.ElementsMatch(t,
		tr.mapNumberMonotonicMetrics(metricName, slice, []string{}),
		[]datadog.Metric{
			metrics.NewCount(metricName, uint64(seconds(1)), 30, []string{}),
			metrics.NewCount(metricName, uint64(seconds(3)), 20, []string{}),
		},
	)
}

func TestMapIntMonotonicOutOfOrder(t *testing.T) {
	stamps := []int{1, 0, 2, 3}
	values := []int64{0, 1, 2, 3}

	metricName := "metric.example"
	slice := pdata.NewNumberDataPointSlice()
	slice.EnsureCapacity(len(values))

	for i, val := range values {
		point := slice.AppendEmpty()
		point.SetTimestamp(seconds(stamps[i]))
		point.SetIntVal(val)
	}

	tr := newTranslator(t, zap.NewNop())
	assert.ElementsMatch(t,
		tr.mapNumberMonotonicMetrics(metricName, slice, []string{}),
		[]datadog.Metric{
			metrics.NewCount(metricName, uint64(seconds(2)), 2, []string{}),
			metrics.NewCount(metricName, uint64(seconds(3)), 1, []string{}),
		},
	)
}

func TestMapDoubleMonotonicMetrics(t *testing.T) {
	deltas := []float64{1, 2, 200, 3, 7, 0}
	cumulative := make([]float64, len(deltas)+1)
	cumulative[0] = 0
	for i := 1; i < len(cumulative); i++ {
		cumulative[i] = cumulative[i-1] + deltas[i-1]
	}

	//Map to OpenTelemetry format
	slice := pdata.NewNumberDataPointSlice()
	slice.EnsureCapacity(len(cumulative))
	for i, val := range cumulative {
		point := slice.AppendEmpty()
		point.SetDoubleVal(val)
		point.SetTimestamp(seconds(i))
	}

	// Map to Datadog format
	metricName := "metric.example"
	expected := make([]datadog.Metric, len(deltas))
	for i, val := range deltas {
		expected[i] = metrics.NewCount(metricName, uint64(seconds(i+1)), val, []string{})
	}

	tr := newTranslator(t, zap.NewNop())
	output := tr.mapNumberMonotonicMetrics(metricName, slice, []string{})

	assert.ElementsMatch(t, expected, output)
}

func TestMapDoubleMonotonicDifferentDimensions(t *testing.T) {
	metricName := "metric.example"
	slice := pdata.NewNumberDataPointSlice()

	// No tags
	point := slice.AppendEmpty()
	point.SetTimestamp(seconds(0))

	point = slice.AppendEmpty()
	point.SetDoubleVal(20)
	point.SetTimestamp(seconds(1))

	// One tag: valA
	point = slice.AppendEmpty()
	point.SetTimestamp(seconds(0))
	point.Attributes().InsertString("key1", "valA")

	point = slice.AppendEmpty()
	point.SetDoubleVal(30)
	point.SetTimestamp(seconds(1))
	point.Attributes().InsertString("key1", "valA")

	// one tag: valB
	point = slice.AppendEmpty()
	point.SetTimestamp(seconds(0))
	point.Attributes().InsertString("key1", "valB")

	point = slice.AppendEmpty()
	point.SetDoubleVal(40)
	point.SetTimestamp(seconds(1))
	point.Attributes().InsertString("key1", "valB")

	tr := newTranslator(t, zap.NewNop())
	assert.ElementsMatch(t,
		tr.mapNumberMonotonicMetrics(metricName, slice, []string{}),
		[]datadog.Metric{
			metrics.NewCount(metricName, uint64(seconds(1)), 20, []string{}),
			metrics.NewCount(metricName, uint64(seconds(1)), 30, []string{"key1:valA"}),
			metrics.NewCount(metricName, uint64(seconds(1)), 40, []string{"key1:valB"}),
		},
	)
}

func TestMapDoubleMonotonicWithReboot(t *testing.T) {
	values := []float64{0, 30, 0, 20}
	metricName := "metric.example"
	slice := pdata.NewNumberDataPointSlice()
	slice.EnsureCapacity(len(values))

	for i, val := range values {
		point := slice.AppendEmpty()
		point.SetTimestamp(seconds(2 * i))
		point.SetDoubleVal(val)
	}

	tr := newTranslator(t, zap.NewNop())
	assert.ElementsMatch(t,
		tr.mapNumberMonotonicMetrics(metricName, slice, []string{}),
		[]datadog.Metric{
			metrics.NewCount(metricName, uint64(seconds(2)), 30, []string{}),
			metrics.NewCount(metricName, uint64(seconds(6)), 20, []string{}),
		},
	)
}

func TestMapDoubleMonotonicOutOfOrder(t *testing.T) {
	stamps := []int{1, 0, 2, 3}
	values := []float64{0, 1, 2, 3}

	metricName := "metric.example"
	slice := pdata.NewNumberDataPointSlice()
	slice.EnsureCapacity(len(values))

	for i, val := range values {
		point := slice.AppendEmpty()
		point.SetTimestamp(seconds(stamps[i]))
		point.SetDoubleVal(val)
	}

	tr := newTranslator(t, zap.NewNop())
	assert.ElementsMatch(t,
		tr.mapNumberMonotonicMetrics(metricName, slice, []string{}),
		[]datadog.Metric{
			metrics.NewCount(metricName, uint64(seconds(2)), 2, []string{}),
			metrics.NewCount(metricName, uint64(seconds(3)), 1, []string{}),
		},
	)
}

func TestMapDeltaHistogramMetrics(t *testing.T) {
	ts := pdata.NewTimestampFromTime(time.Now())
	slice := pdata.NewHistogramDataPointSlice()
	point := slice.AppendEmpty()
	point.SetCount(20)
	point.SetSum(math.Pi)
	point.SetBucketCounts([]uint64{2, 18})
	point.SetExplicitBounds([]float64{0})
	point.SetTimestamp(ts)

	noBuckets := []datadog.Metric{
		metrics.NewCount("doubleHist.test.count", uint64(ts), 20, []string{}),
		metrics.NewCount("doubleHist.test.sum", uint64(ts), math.Pi, []string{}),
	}

	buckets := []datadog.Metric{
		metrics.NewCount("doubleHist.test.bucket", uint64(ts), 2, []string{"lower_bound:-inf", "upper_bound:0"}),
		metrics.NewCount("doubleHist.test.bucket", uint64(ts), 18, []string{"lower_bound:0", "upper_bound:inf"}),
	}

	tr := newTranslator(t, zap.NewNop())
	delta := true

	tr.cfg.HistMode = HistogramModeNoBuckets
	res, sl := tr.mapHistogramMetrics("doubleHist.test", slice, delta, []string{})
	require.Empty(t, sl)
	assert.ElementsMatch(t,
		res, // No buckets
		noBuckets,
	)

	tr.cfg.HistMode = HistogramModeCounters
	res, sl = tr.mapHistogramMetrics("doubleHist.test", slice, delta, []string{})
	require.Empty(t, sl)
	assert.ElementsMatch(t,
		res, // buckets
		append(noBuckets, buckets...),
	)

	// With attribute tags
	noBucketsAttributeTags := []datadog.Metric{
		metrics.NewCount("doubleHist.test.count", uint64(ts), 20, []string{"attribute_tag:attribute_value"}),
		metrics.NewCount("doubleHist.test.sum", uint64(ts), math.Pi, []string{"attribute_tag:attribute_value"}),
	}

	bucketsAttributeTags := []datadog.Metric{
		metrics.NewCount("doubleHist.test.bucket", uint64(ts), 2, []string{"lower_bound:-inf", "upper_bound:0", "attribute_tag:attribute_value"}),
		metrics.NewCount("doubleHist.test.bucket", uint64(ts), 18, []string{"lower_bound:0", "upper_bound:inf", "attribute_tag:attribute_value"}),
	}

	tr.cfg.HistMode = HistogramModeNoBuckets
	res, sl = tr.mapHistogramMetrics("doubleHist.test", slice, delta, []string{"attribute_tag:attribute_value"})
	require.Empty(t, sl)
	assert.ElementsMatch(t,
		res, // No buckets
		noBucketsAttributeTags,
	)

	tr.cfg.HistMode = HistogramModeCounters
	res, sl = tr.mapHistogramMetrics("doubleHist.test", slice, delta, []string{"attribute_tag:attribute_value"})
	require.Empty(t, sl)
	assert.ElementsMatch(t,
		res, // buckets
		append(noBucketsAttributeTags, bucketsAttributeTags...),
	)
}

func TestMapCumulativeHistogramMetrics(t *testing.T) {
	slice := pdata.NewHistogramDataPointSlice()
	point := slice.AppendEmpty()
	point.SetCount(20)
	point.SetSum(math.Pi)
	point.SetBucketCounts([]uint64{2, 18})
	point.SetExplicitBounds([]float64{0})
	point.SetTimestamp(seconds(0))

	point = slice.AppendEmpty()
	point.SetCount(20 + 30)
	point.SetSum(math.Pi + 20)
	point.SetBucketCounts([]uint64{2 + 11, 18 + 2})
	point.SetExplicitBounds([]float64{0})
	point.SetTimestamp(seconds(2))

	expected := []datadog.Metric{
		metrics.NewCount("doubleHist.test.count", uint64(seconds(2)), 30, []string{}),
		metrics.NewCount("doubleHist.test.sum", uint64(seconds(2)), 20, []string{}),
		metrics.NewCount("doubleHist.test.bucket", uint64(seconds(2)), 11, []string{"lower_bound:-inf", "upper_bound:0"}),
		metrics.NewCount("doubleHist.test.bucket", uint64(seconds(2)), 2, []string{"lower_bound:0", "upper_bound:inf"}),
	}

	tr := newTranslator(t, zap.NewNop())
	delta := false

	tr.cfg.HistMode = HistogramModeCounters
	res, sl := tr.mapHistogramMetrics("doubleHist.test", slice, delta, []string{})
	require.Empty(t, sl)
	assert.ElementsMatch(t,
		res,
		expected,
	)
}

func TestLegacyBucketsTags(t *testing.T) {
	// Test that passing the same tags slice doesn't reuse the slice.
	tr := newTranslator(t, zap.NewNop())

	tags := make([]string, 0, 10)

	pointOne := pdata.NewHistogramDataPoint()
	pointOne.SetBucketCounts([]uint64{2, 18})
	pointOne.SetExplicitBounds([]float64{0})
	pointOne.SetTimestamp(seconds(0))
	seriesOne := tr.getLegacyBuckets("test.histogram.one", pointOne, true, tags)

	pointTwo := pdata.NewHistogramDataPoint()
	pointTwo.SetBucketCounts([]uint64{2, 18})
	pointTwo.SetExplicitBounds([]float64{1})
	pointTwo.SetTimestamp(seconds(0))
	seriesTwo := tr.getLegacyBuckets("test.histogram.two", pointTwo, true, tags)

	assert.ElementsMatch(t, seriesOne[0].Tags, []string{"lower_bound:-inf", "upper_bound:0"})
	assert.ElementsMatch(t, seriesTwo[0].Tags, []string{"lower_bound:-inf", "upper_bound:1.0"})
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		f float64
		s string
	}{
		{f: 0, s: "0"},
		{f: 0.001, s: "0.001"},
		{f: 0.9, s: "0.9"},
		{f: 0.95, s: "0.95"},
		{f: 0.99, s: "0.99"},
		{f: 0.999, s: "0.999"},
		{f: 1, s: "1.0"},
		{f: 2, s: "2.0"},
		{f: math.Inf(1), s: "inf"},
		{f: math.Inf(-1), s: "-inf"},
		{f: math.NaN(), s: "nan"},
		{f: 1e-10, s: "1e-10"},
	}

	for _, test := range tests {
		assert.Equal(t, test.s, formatFloat(test.f))
	}
}

func exampleSummaryDataPointSlice(ts pdata.Timestamp, sum float64, count uint64) pdata.SummaryDataPointSlice {
	slice := pdata.NewSummaryDataPointSlice()
	point := slice.AppendEmpty()
	point.SetCount(count)
	point.SetSum(sum)
	qSlice := point.QuantileValues()

	qMin := qSlice.AppendEmpty()
	qMin.SetQuantile(0.0)
	qMin.SetValue(0)

	qMedian := qSlice.AppendEmpty()
	qMedian.SetQuantile(0.5)
	qMedian.SetValue(100)

	q999 := qSlice.AppendEmpty()
	q999.SetQuantile(0.999)
	q999.SetValue(500)

	qMax := qSlice.AppendEmpty()
	qMax.SetQuantile(1)
	qMax.SetValue(600)
	point.SetTimestamp(ts)
	return slice
}

func TestMapSummaryMetrics(t *testing.T) {
	ts := pdata.NewTimestampFromTime(time.Now())
	slice := exampleSummaryDataPointSlice(ts, 10_001, 101)

	newTranslator := func(tags []string, quantiles bool) *Translator {
		c := newTestCache()
		c.cache.Set(c.metricDimensionsToMapKey("summary.example.count", tags), numberCounter{0, 1}, gocache.NoExpiration)
		c.cache.Set(c.metricDimensionsToMapKey("summary.example.sum", tags), numberCounter{0, 1}, gocache.NoExpiration)
		options := []Option{WithFallbackHostnameProvider(testProvider("fallbackHostname"))}
		if quantiles {
			options = append(options, WithQuantiles())
		}
		tr, err := New(componenttest.NewNopExporterCreateSettings(), options...)
		require.NoError(t, err)
		tr.prevPts = c
		return tr
	}

	noQuantiles := []datadog.Metric{
		metrics.NewCount("summary.example.count", uint64(ts), 100, []string{}),
		metrics.NewCount("summary.example.sum", uint64(ts), 10_000, []string{}),
	}
	quantiles := []datadog.Metric{
		metrics.NewGauge("summary.example.quantile", uint64(ts), 0, []string{"quantile:0"}),
		metrics.NewGauge("summary.example.quantile", uint64(ts), 100, []string{"quantile:0.5"}),
		metrics.NewGauge("summary.example.quantile", uint64(ts), 500, []string{"quantile:0.999"}),
		metrics.NewGauge("summary.example.quantile", uint64(ts), 600, []string{"quantile:1.0"}),
	}
	tr := newTranslator([]string{}, false)
	assert.ElementsMatch(t,
		tr.mapSummaryMetrics("summary.example", slice, []string{}),
		noQuantiles,
	)
	tr = newTranslator([]string{}, true)
	assert.ElementsMatch(t,
		tr.mapSummaryMetrics("summary.example", slice, []string{}),
		append(noQuantiles, quantiles...),
	)

	noQuantilesAttr := []datadog.Metric{
		metrics.NewCount("summary.example.count", uint64(ts), 100, []string{"attribute_tag:attribute_value"}),
		metrics.NewCount("summary.example.sum", uint64(ts), 10_000, []string{"attribute_tag:attribute_value"}),
	}
	quantilesAttr := []datadog.Metric{
		metrics.NewGauge("summary.example.quantile", uint64(ts), 0, []string{"quantile:0", "attribute_tag:attribute_value"}),
		metrics.NewGauge("summary.example.quantile", uint64(ts), 100, []string{"quantile:0.5", "attribute_tag:attribute_value"}),
		metrics.NewGauge("summary.example.quantile", uint64(ts), 500, []string{"quantile:0.999", "attribute_tag:attribute_value"}),
		metrics.NewGauge("summary.example.quantile", uint64(ts), 600, []string{"quantile:1.0", "attribute_tag:attribute_value"}),
	}
	tr = newTranslator([]string{"attribute_tag:attribute_value"}, false)
	assert.ElementsMatch(t,
		tr.mapSummaryMetrics("summary.example", slice, []string{"attribute_tag:attribute_value"}),
		noQuantilesAttr,
	)
	tr = newTranslator([]string{"attribute_tag:attribute_value"}, true)
	assert.ElementsMatch(t,
		tr.mapSummaryMetrics("summary.example", slice, []string{"attribute_tag:attribute_value"}),
		append(noQuantilesAttr, quantilesAttr...),
	)
}

func TestRunningMetrics(t *testing.T) {
	ms := pdata.NewMetrics()
	rms := ms.ResourceMetrics()

	rm := rms.AppendEmpty()
	resAttrs := rm.Resource().Attributes()
	resAttrs.Insert(attributes.AttributeDatadogHostname, pdata.NewAttributeValueString("resource-hostname-1"))

	rm = rms.AppendEmpty()
	resAttrs = rm.Resource().Attributes()
	resAttrs.Insert(attributes.AttributeDatadogHostname, pdata.NewAttributeValueString("resource-hostname-1"))

	rm = rms.AppendEmpty()
	resAttrs = rm.Resource().Attributes()
	resAttrs.Insert(attributes.AttributeDatadogHostname, pdata.NewAttributeValueString("resource-hostname-2"))

	rms.AppendEmpty()
	ctx := context.Background()
	tr := newTranslator(t, zap.NewNop())

	series, sl, err := tr.MapMetrics(ctx, ms)
	require.Empty(t, sl)
	require.NoError(t, err)

	runningHostnames := []string{}

	for _, metric := range series {
		if *metric.Metric == "otel.datadog_exporter.metrics.running" {
			if metric.Host != nil {
				runningHostnames = append(runningHostnames, *metric.Host)
			}
		}
	}

	assert.ElementsMatch(t,
		runningHostnames,
		[]string{"fallbackHostname", "resource-hostname-1", "resource-hostname-2"},
	)

}

const (
	testHostname = "res-hostname"
)

func createTestMetrics() pdata.Metrics {
	md := pdata.NewMetrics()
	rms := md.ResourceMetrics()
	rm := rms.AppendEmpty()

	attrs := rm.Resource().Attributes()
	attrs.InsertString(attributes.AttributeDatadogHostname, testHostname)
	ilms := rm.InstrumentationLibraryMetrics()

	metricsArray := ilms.AppendEmpty().Metrics()
	metricsArray.AppendEmpty() // first one is TypeNone to test that it's ignored

	// IntGauge
	met := metricsArray.AppendEmpty()
	met.SetName("int.gauge")
	met.SetDataType(pdata.MetricDataTypeGauge)
	dpsInt := met.Gauge().DataPoints()
	dpInt := dpsInt.AppendEmpty()
	dpInt.SetTimestamp(seconds(0))
	dpInt.SetIntVal(1)

	// DoubleGauge
	met = metricsArray.AppendEmpty()
	met.SetName("double.gauge")
	met.SetDataType(pdata.MetricDataTypeGauge)
	dpsDouble := met.Gauge().DataPoints()
	dpDouble := dpsDouble.AppendEmpty()
	dpDouble.SetTimestamp(seconds(0))
	dpDouble.SetDoubleVal(math.Pi)

	// aggregation unspecified sum
	met = metricsArray.AppendEmpty()
	met.SetName("unspecified.sum")
	met.SetDataType(pdata.MetricDataTypeSum)
	met.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityUnspecified)

	// Int Sum (delta)
	met = metricsArray.AppendEmpty()
	met.SetName("int.delta.sum")
	met.SetDataType(pdata.MetricDataTypeSum)
	met.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
	dpsInt = met.Sum().DataPoints()
	dpInt = dpsInt.AppendEmpty()
	dpInt.SetTimestamp(seconds(0))
	dpInt.SetIntVal(2)

	// Double Sum (delta)
	met = metricsArray.AppendEmpty()
	met.SetName("double.delta.sum")
	met.SetDataType(pdata.MetricDataTypeSum)
	met.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
	dpsDouble = met.Sum().DataPoints()
	dpDouble = dpsDouble.AppendEmpty()
	dpDouble.SetTimestamp(seconds(0))
	dpDouble.SetDoubleVal(math.E)

	// Int Sum (delta monotonic)
	met = metricsArray.AppendEmpty()
	met.SetName("int.delta.monotonic.sum")
	met.SetDataType(pdata.MetricDataTypeSum)
	met.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
	dpsInt = met.Sum().DataPoints()
	dpInt = dpsInt.AppendEmpty()
	dpInt.SetTimestamp(seconds(0))
	dpInt.SetIntVal(2)

	// Double Sum (delta monotonic)
	met = metricsArray.AppendEmpty()
	met.SetName("double.delta.monotonic.sum")
	met.SetDataType(pdata.MetricDataTypeSum)
	met.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
	dpsDouble = met.Sum().DataPoints()
	dpDouble = dpsDouble.AppendEmpty()
	dpDouble.SetTimestamp(seconds(0))
	dpDouble.SetDoubleVal(math.E)

	// aggregation unspecified histogram
	met = metricsArray.AppendEmpty()
	met.SetName("unspecified.histogram")
	met.SetDataType(pdata.MetricDataTypeHistogram)
	met.Histogram().SetAggregationTemporality(pdata.MetricAggregationTemporalityUnspecified)

	// Histogram (delta)
	met = metricsArray.AppendEmpty()
	met.SetName("double.histogram")
	met.SetDataType(pdata.MetricDataTypeHistogram)
	met.Histogram().SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
	dpsDoubleHist := met.Histogram().DataPoints()
	dpDoubleHist := dpsDoubleHist.AppendEmpty()
	dpDoubleHist.SetCount(20)
	dpDoubleHist.SetSum(math.Phi)
	dpDoubleHist.SetBucketCounts([]uint64{2, 18})
	dpDoubleHist.SetExplicitBounds([]float64{0})
	dpDoubleHist.SetTimestamp(seconds(0))

	// Int Sum (cumulative)
	met = metricsArray.AppendEmpty()
	met.SetName("int.cumulative.sum")
	met.SetDataType(pdata.MetricDataTypeSum)
	met.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
	dpsInt = met.Sum().DataPoints()
	dpsInt.EnsureCapacity(2)
	dpInt = dpsInt.AppendEmpty()
	dpInt.SetTimestamp(seconds(0))
	dpInt.SetIntVal(4)

	// Double Sum (cumulative)
	met = metricsArray.AppendEmpty()
	met.SetName("double.cumulative.sum")
	met.SetDataType(pdata.MetricDataTypeSum)
	met.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
	dpsDouble = met.Sum().DataPoints()
	dpsDouble.EnsureCapacity(2)
	dpDouble = dpsDouble.AppendEmpty()
	dpDouble.SetTimestamp(seconds(0))
	dpDouble.SetDoubleVal(4)

	// Int Sum (cumulative monotonic)
	met = metricsArray.AppendEmpty()
	met.SetName("int.cumulative.monotonic.sum")
	met.SetDataType(pdata.MetricDataTypeSum)
	met.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
	met.Sum().SetIsMonotonic(true)
	dpsInt = met.Sum().DataPoints()
	dpsInt.EnsureCapacity(2)
	dpInt = dpsInt.AppendEmpty()
	dpInt.SetTimestamp(seconds(0))
	dpInt.SetIntVal(4)
	dpInt = dpsInt.AppendEmpty()
	dpInt.SetTimestamp(seconds(2))
	dpInt.SetIntVal(7)

	// Double Sum (cumulative monotonic)
	met = metricsArray.AppendEmpty()
	met.SetName("double.cumulative.monotonic.sum")
	met.SetDataType(pdata.MetricDataTypeSum)
	met.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
	met.Sum().SetIsMonotonic(true)
	dpsDouble = met.Sum().DataPoints()
	dpsDouble.EnsureCapacity(2)
	dpDouble = dpsDouble.AppendEmpty()
	dpDouble.SetTimestamp(seconds(0))
	dpDouble.SetDoubleVal(4)
	dpDouble = dpsDouble.AppendEmpty()
	dpDouble.SetTimestamp(seconds(2))
	dpDouble.SetDoubleVal(4 + math.Pi)

	// Summary
	met = metricsArray.AppendEmpty()
	met.SetName("summary")
	met.SetDataType(pdata.MetricDataTypeSummary)
	slice := exampleSummaryDataPointSlice(seconds(0), 1, 1)
	slice.CopyTo(met.Summary().DataPoints())

	met = metricsArray.AppendEmpty()
	met.SetName("summary")
	met.SetDataType(pdata.MetricDataTypeSummary)
	slice = exampleSummaryDataPointSlice(seconds(2), 10_001, 101)
	slice.CopyTo(met.Summary().DataPoints())
	return md
}

func removeRunningMetrics(series []datadog.Metric) []datadog.Metric {
	filtered := []datadog.Metric{}
	for _, m := range series {
		if m.GetMetric() != "otel.datadog_exporter.metrics.running" {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

func testGauge(name string, val float64) datadog.Metric {
	m := metrics.NewGauge(name, 0, val, []string{})
	m.SetHost(testHostname)
	return m
}

func testCount(name string, val float64, seconds uint64) datadog.Metric {
	m := metrics.NewCount(name, seconds*1e9, val, []string{})
	m.SetHost(testHostname)
	return m
}

func TestMapMetrics(t *testing.T) {
	md := createTestMetrics()

	core, observed := observer.New(zapcore.DebugLevel)
	testLogger := zap.New(core)
	ctx := context.Background()
	tr := newTranslator(t, testLogger)
	series, sl, err := tr.MapMetrics(ctx, md)
	require.Empty(t, sl)
	require.NoError(t, err)

	filtered := removeRunningMetrics(series)
	assert.ElementsMatch(t, filtered, []datadog.Metric{
		testGauge("int.gauge", 1),
		testGauge("double.gauge", math.Pi),
		testCount("int.delta.sum", 2, 0),
		testCount("double.delta.sum", math.E, 0),
		testCount("int.delta.monotonic.sum", 2, 0),
		testCount("double.delta.monotonic.sum", math.E, 0),
		testCount("double.histogram.sum", math.Phi, 0),
		testCount("double.histogram.count", 20, 0),
		testCount("summary.sum", 10_000, 2),
		testCount("summary.count", 100, 2),
		testGauge("int.cumulative.sum", 4),
		testGauge("double.cumulative.sum", 4),
		testCount("int.cumulative.monotonic.sum", 3, 2),
		testCount("double.cumulative.monotonic.sum", math.Pi, 2),
	})

	// One metric type was unknown or unsupported
	assert.Equal(t, observed.FilterMessage("Unknown or unsupported metric type").Len(), 1)
	// Two metric aggregation temporality was unknown or unsupported
	assert.Equal(t, observed.FilterMessage("Unknown or unsupported aggregation temporality").Len(), 2)
}

func createNaNMetrics() pdata.Metrics {
	md := pdata.NewMetrics()
	rms := md.ResourceMetrics()
	rm := rms.AppendEmpty()

	attrs := rm.Resource().Attributes()
	attrs.InsertString(attributes.AttributeDatadogHostname, testHostname)
	ilms := rm.InstrumentationLibraryMetrics()

	metricsArray := ilms.AppendEmpty().Metrics()

	// DoubleGauge
	met := metricsArray.AppendEmpty()
	met.SetName("nan.gauge")
	met.SetDataType(pdata.MetricDataTypeGauge)
	dpsDouble := met.Gauge().DataPoints()
	dpDouble := dpsDouble.AppendEmpty()
	dpDouble.SetTimestamp(seconds(0))
	dpDouble.SetDoubleVal(math.NaN())

	// Double Sum (delta)
	met = metricsArray.AppendEmpty()
	met.SetName("nan.delta.sum")
	met.SetDataType(pdata.MetricDataTypeSum)
	met.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
	dpsDouble = met.Sum().DataPoints()
	dpDouble = dpsDouble.AppendEmpty()
	dpDouble.SetTimestamp(seconds(0))
	dpDouble.SetDoubleVal(math.NaN())

	// Double Sum (delta monotonic)
	met = metricsArray.AppendEmpty()
	met.SetName("nan.delta.monotonic.sum")
	met.SetDataType(pdata.MetricDataTypeSum)
	met.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
	dpsDouble = met.Sum().DataPoints()
	dpDouble = dpsDouble.AppendEmpty()
	dpDouble.SetTimestamp(seconds(0))
	dpDouble.SetDoubleVal(math.NaN())

	// Histogram
	met = metricsArray.AppendEmpty()
	met.SetName("nan.histogram")
	met.SetDataType(pdata.MetricDataTypeHistogram)
	met.Histogram().SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
	dpsDoubleHist := met.Histogram().DataPoints()
	dpDoubleHist := dpsDoubleHist.AppendEmpty()
	dpDoubleHist.SetCount(20)
	dpDoubleHist.SetSum(math.NaN())
	dpDoubleHist.SetBucketCounts([]uint64{2, 18})
	dpDoubleHist.SetTimestamp(seconds(0))

	// Double Sum (cumulative)
	met = metricsArray.AppendEmpty()
	met.SetName("nan.cumulative.sum")
	met.SetDataType(pdata.MetricDataTypeSum)
	met.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
	dpsDouble = met.Sum().DataPoints()
	dpsDouble.EnsureCapacity(2)
	dpDouble = dpsDouble.AppendEmpty()
	dpDouble.SetTimestamp(seconds(0))
	dpDouble.SetDoubleVal(math.NaN())

	// Double Sum (cumulative monotonic)
	met = metricsArray.AppendEmpty()
	met.SetName("nan.cumulative.monotonic.sum")
	met.SetDataType(pdata.MetricDataTypeSum)
	met.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
	met.Sum().SetIsMonotonic(true)
	dpsDouble = met.Sum().DataPoints()
	dpsDouble.EnsureCapacity(2)
	dpDouble = dpsDouble.AppendEmpty()
	dpDouble.SetTimestamp(seconds(0))
	dpDouble.SetDoubleVal(math.NaN())

	// Summary
	met = metricsArray.AppendEmpty()
	met.SetName("nan.summary")
	met.SetDataType(pdata.MetricDataTypeSummary)
	slice := exampleSummaryDataPointSlice(seconds(0), math.NaN(), 1)
	slice.CopyTo(met.Summary().DataPoints())

	met = metricsArray.AppendEmpty()
	met.SetName("nan.summary")
	met.SetDataType(pdata.MetricDataTypeSummary)
	slice = exampleSummaryDataPointSlice(seconds(2), 10_001, 101)
	slice.CopyTo(met.Summary().DataPoints())
	return md
}

func TestNaNMetrics(t *testing.T) {
	md := createNaNMetrics()

	core, observed := observer.New(zapcore.DebugLevel)
	testLogger := zap.New(core)
	tr := newTranslator(t, testLogger)
	ctx := context.Background()
	series, sl, err := tr.MapMetrics(ctx, md)
	require.Empty(t, sl)
	require.NoError(t, err)

	filtered := removeRunningMetrics(series)
	assert.ElementsMatch(t, filtered, []datadog.Metric{
		testCount("nan.histogram.count", 20, 0),
		testCount("nan.summary.count", 100, 2),
	})

	// One metric type was unknown or unsupported
	assert.Equal(t, observed.FilterMessage("Unsupported metric value").Len(), 7)
}
