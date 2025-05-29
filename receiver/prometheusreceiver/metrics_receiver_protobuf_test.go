// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"math"
	"testing"

	types "github.com/gogo/protobuf/types"
	"github.com/prometheus/prometheus/config"
	dto "github.com/prometheus/prometheus/prompb/io/prometheus/client"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestScrapeViaProtobuf(t *testing.T) {
	mf := &dto.MetricFamily{
		Name: "test_counter",
		Type: dto.MetricType_COUNTER,
		Metric: []dto.Metric{
			{
				Label: []dto.LabelPair{
					{
						Name:  "foo",
						Value: "bar",
					},
				},
				Counter: &dto.Counter{
					Value: 1234,
				},
			},
		},
	}
	buffer := prometheusMetricFamilyToProtoBuf(t, nil, mf)

	mf = &dto.MetricFamily{
		Name: "test_gauge",
		Type: dto.MetricType_GAUGE,
		Metric: []dto.Metric{
			{
				Gauge: &dto.Gauge{
					Value: 400.8,
				},
			},
		},
	}
	prometheusMetricFamilyToProtoBuf(t, buffer, mf)

	mf = &dto.MetricFamily{
		Name: "test_summary",
		Type: dto.MetricType_SUMMARY,
		Metric: []dto.Metric{
			{
				Summary: &dto.Summary{
					SampleCount: 1213,
					SampleSum:   456,
					Quantile: []dto.Quantile{
						{
							Quantile: 0.5,
							Value:    789,
						},
						{
							Quantile: 0.9,
							Value:    1011,
						},
					},
				},
			},
		},
	}
	prometheusMetricFamilyToProtoBuf(t, buffer, mf)

	mf = &dto.MetricFamily{
		Name: "test_histogram",
		Type: dto.MetricType_HISTOGRAM,
		Metric: []dto.Metric{
			{
				Histogram: &dto.Histogram{
					SampleCount: 1213,
					SampleSum:   456,
					Bucket: []dto.Bucket{
						{
							UpperBound:      0.5,
							CumulativeCount: 789,
						},
						{
							UpperBound:      10,
							CumulativeCount: 1011,
						},
						{
							UpperBound:      math.Inf(1),
							CumulativeCount: 1213,
						},
					},
				},
			},
		},
	}
	prometheusMetricFamilyToProtoBuf(t, buffer, mf)

	expectations := []metricExpectation{
		{
			"test_counter",
			pmetric.MetricTypeSum,
			"",
			[]dataPointExpectation{{
				numberPointComparator: []numberPointComparator{
					compareDoubleValue(1234),
				},
			}},
			nil,
		},
		{
			"test_gauge",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{{
				numberPointComparator: []numberPointComparator{
					compareDoubleValue(400.8),
				},
			}},
			nil,
		},
		{
			"test_summary",
			pmetric.MetricTypeSummary,
			"",
			[]dataPointExpectation{{
				summaryPointComparator: []summaryPointComparator{
					compareSummary(1213, 456, [][]float64{{0.5, 789}, {0.9, 1011}}),
				},
			}},
			nil,
		},
		{
			"test_histogram",
			pmetric.MetricTypeHistogram,
			"",
			[]dataPointExpectation{{
				histogramPointComparator: []histogramPointComparator{
					compareHistogram(1213, 456, []float64{0.5, 10}, []uint64{789, 222, 202}),
				},
			}},
			nil,
		},
	}

	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, useProtoBuf: true, buf: buffer.Bytes()},
			},
			validateFunc: func(t *testing.T, td *testData, result []pmetric.ResourceMetrics) {
				verifyNumValidScrapeResults(t, td, result)
				doCompare(t, "target1", td.attributes, result[0], expectations)
			},
		},
	}

	testComponent(t, targets, func(c *Config) {
		c.PrometheusConfig.GlobalConfig.ScrapeProtocols = []config.ScrapeProtocol{config.PrometheusProto}
	})
}

func TestNativeVsClassicHistogramScrapeViaProtobuf(t *testing.T) {
	classicHistogram := &dto.MetricFamily{
		Name: "test_classic_histogram",
		Type: dto.MetricType_HISTOGRAM,
		Metric: []dto.Metric{
			{
				Histogram: &dto.Histogram{
					SampleCount: 1213,
					SampleSum:   456,
					Bucket: []dto.Bucket{
						{
							UpperBound:      0.5,
							CumulativeCount: 789,
						},
						{
							UpperBound:      10,
							CumulativeCount: 1011,
							Exemplar: &dto.Exemplar{
								Value: 8.1,
								Label: []dto.LabelPair{{Name: "tch_e1", Value: "v1"}},
							},
						},
						{
							UpperBound:      math.Inf(1),
							CumulativeCount: 1213,
							Exemplar: &dto.Exemplar{
								Value: 18,
								Label: []dto.LabelPair{{Name: "tch_e2", Value: "v2"}},
							},
						},
					},
				},
			},
		},
	}
	buffer := prometheusMetricFamilyToProtoBuf(t, nil, classicHistogram)

	mixedHistogram := &dto.MetricFamily{
		Name: "test_mixed_histogram",
		Type: dto.MetricType_HISTOGRAM,
		Metric: []dto.Metric{
			{
				Histogram: &dto.Histogram{
					SampleCount: 1213,
					SampleSum:   456,
					Bucket: []dto.Bucket{
						{
							UpperBound:      0.5,
							CumulativeCount: 789,
							Exemplar: &dto.Exemplar{
								Value: 0.1,
								Label: []dto.LabelPair{{Name: "tmhc_e1", Value: "v1"}},
							},
						},
						{
							UpperBound:      10,
							CumulativeCount: 1011,
						},
						{
							UpperBound:      math.Inf(1),
							CumulativeCount: 1213,
						},
					},
					// Integer counter histogram definition
					Schema:        3,
					ZeroThreshold: 0.001,
					ZeroCount:     2,
					NegativeSpan: []dto.BucketSpan{
						{Offset: 0, Length: 1},
						{Offset: 1, Length: 1},
					},
					NegativeDelta: []int64{1, 1},
					PositiveSpan: []dto.BucketSpan{
						{Offset: -2, Length: 1},
						{Offset: 1, Length: 1},
					},
					PositiveDelta: []int64{1, 0},
					Exemplars: []*dto.Exemplar{
						{
							Value:     0.2,
							Timestamp: &types.Timestamp{Seconds: 123, Nanos: 456},
							Label:     []dto.LabelPair{{Name: "tmhn_e1", Value: "mv1"}},
						},
						{
							Value:     1.3,
							Timestamp: &types.Timestamp{Seconds: 321, Nanos: 456},
							Label:     []dto.LabelPair{{Name: "tmhn_e2", Value: "mv2"}},
						},
					},
				},
			},
		},
	}
	prometheusMetricFamilyToProtoBuf(t, buffer, mixedHistogram)

	nativeHistogram := &dto.MetricFamily{
		Name: "test_native_histogram",
		Type: dto.MetricType_HISTOGRAM,
		Metric: []dto.Metric{
			{
				Histogram: &dto.Histogram{
					SampleCount: 1214,
					SampleSum:   3456,
					// Integer counter histogram definition
					Schema:        3,
					ZeroThreshold: 0.001,
					ZeroCount:     5,
					NegativeSpan: []dto.BucketSpan{
						{Offset: -2, Length: 1},
						{Offset: 1, Length: 1},
					},
					NegativeDelta: []int64{1, 1},
					PositiveSpan: []dto.BucketSpan{
						{Offset: 3, Length: 1},
						{Offset: 2, Length: 1},
					},
					PositiveDelta: []int64{1, 0},
					Exemplars: []*dto.Exemplar{
						{
							Value:     0.8,
							Label:     []dto.LabelPair{{Name: "tnh_e1", Value: "mv1"}},
							Timestamp: &types.Timestamp{Seconds: 234, Nanos: 456},
						},
						{
							Value:     111,
							Label:     []dto.LabelPair{{Name: "tnh_e2", Value: "mv2"}},
							Timestamp: &types.Timestamp{Seconds: 432, Nanos: 456},
						},
					},
				},
			},
		},
	}
	prometheusMetricFamilyToProtoBuf(t, buffer, nativeHistogram)

	// Exemplar check function to check that the right values go to the
	// right datapoints.
	checkClassicHistogramExemplars := func(t *testing.T, hdp pmetric.HistogramDataPoint) {
		require.Equal(t, 2, hdp.Exemplars().Len())
		values := []float64{hdp.Exemplars().At(0).DoubleValue(), hdp.Exemplars().At(1).DoubleValue()}
		require.Contains(t, values, 8.1)
		require.Contains(t, values, 18.0)
	}
	checkMixedHistogramClassicExemplars := func(t *testing.T, hdp pmetric.HistogramDataPoint) {
		require.Equal(t, 1, hdp.Exemplars().Len())
		require.Equal(t, 0.1, hdp.Exemplars().At(0).DoubleValue())
	}
	checkMixedHistogramNativeExemplars := func(t *testing.T, hdp pmetric.ExponentialHistogramDataPoint) {
		require.Equal(t, 2, hdp.Exemplars().Len())
		values := []float64{hdp.Exemplars().At(0).DoubleValue(), hdp.Exemplars().At(1).DoubleValue()}
		require.Contains(t, values, 0.2)
		require.Contains(t, values, 1.3)
	}
	checkNativeHistogramExemplars := func(t *testing.T, hdp pmetric.ExponentialHistogramDataPoint) {
		require.Equal(t, 2, hdp.Exemplars().Len())
		values := []float64{hdp.Exemplars().At(0).DoubleValue(), hdp.Exemplars().At(1).DoubleValue()}
		require.Contains(t, values, 0.8)
		require.Contains(t, values, 111.0)
	}

	testCases := map[string]struct {
		mutCfg                 func(*PromConfig)
		enableNativeHistograms bool
		expected               []metricExpectation
	}{
		"feature enabled scrape classic off": {
			enableNativeHistograms: true,
			expected: []metricExpectation{
				{ // Scrape classic only histograms as is.
					"test_classic_histogram",
					pmetric.MetricTypeHistogram,
					"",
					[]dataPointExpectation{{
						histogramPointComparator: []histogramPointComparator{
							compareHistogram(1213, 456, []float64{0.5, 10}, []uint64{789, 222, 202}),
							checkClassicHistogramExemplars,
						},
					}},
					nil,
				},
				{ // Only scrape native buckets from mixed histograms.
					"test_mixed_histogram",
					pmetric.MetricTypeExponentialHistogram,
					"",
					[]dataPointExpectation{{
						exponentialHistogramComparator: []exponentialHistogramComparator{
							compareExponentialHistogram(3, 1213, 456, 2, -1, []uint64{1, 0, 2}, -3, []uint64{1, 0, 1}),
							checkMixedHistogramNativeExemplars,
						},
					}},
					nil,
				},
				{ // Scrape native only histograms as is.
					"test_native_histogram",
					pmetric.MetricTypeExponentialHistogram,
					"",
					[]dataPointExpectation{{
						exponentialHistogramComparator: []exponentialHistogramComparator{
							compareExponentialHistogram(3, 1214, 3456, 5, -3, []uint64{1, 0, 2}, 2, []uint64{1, 0, 0, 1}),
							checkNativeHistogramExemplars,
						},
					}},
					nil,
				},
			},
		},
		"feature disabled scrape classic off": {
			enableNativeHistograms: false,
			expected: []metricExpectation{
				{ // Scrape classic only histograms as is.
					"test_classic_histogram",
					pmetric.MetricTypeHistogram,
					"",
					[]dataPointExpectation{{
						histogramPointComparator: []histogramPointComparator{
							compareHistogram(1213, 456, []float64{0.5, 10}, []uint64{789, 222, 202}),
							checkClassicHistogramExemplars,
						},
					}},
					nil,
				},
				{ // Fallback to scraping classic histograms if feature is off.
					"test_mixed_histogram",
					pmetric.MetricTypeHistogram,
					"",
					[]dataPointExpectation{{
						histogramPointComparator: []histogramPointComparator{
							compareHistogram(1213, 456, []float64{0.5, 10}, []uint64{789, 222, 202}),
							checkMixedHistogramClassicExemplars,
						},
					}},
					nil,
				},
			},
		},
		"feature enabled scrape classic on": {
			mutCfg: func(cfg *PromConfig) {
				for _, scrapeConfig := range cfg.ScrapeConfigs {
					scrapeConfig.AlwaysScrapeClassicHistograms = true
				}
			},
			enableNativeHistograms: true,
			expected: []metricExpectation{
				{ // Scrape classic only histograms as is.
					"test_classic_histogram",
					pmetric.MetricTypeHistogram,
					"",
					[]dataPointExpectation{{
						histogramPointComparator: []histogramPointComparator{
							compareHistogram(1213, 456, []float64{0.5, 10}, []uint64{789, 222, 202}),
							checkClassicHistogramExemplars,
						},
					}},
					nil,
				},
				{ // Scrape both classic and native buckets from mixed histograms.
					"test_mixed_histogram",
					pmetric.MetricTypeHistogram,
					"",
					[]dataPointExpectation{{
						histogramPointComparator: []histogramPointComparator{
							compareHistogram(1213, 456, []float64{0.5, 10}, []uint64{789, 222, 202}),
							checkMixedHistogramClassicExemplars,
						},
					}},
					nil,
				},
				{ // Scrape both classic and native buckets from mixed histograms.
					"test_mixed_histogram",
					pmetric.MetricTypeExponentialHistogram,
					"",
					[]dataPointExpectation{{
						exponentialHistogramComparator: []exponentialHistogramComparator{
							compareExponentialHistogram(3, 1213, 456, 2, -1, []uint64{1, 0, 2}, -3, []uint64{1, 0, 1}),
							checkMixedHistogramNativeExemplars,
						},
					}},
					nil,
				},
				{ // Scrape native only histograms as is.
					"test_native_histogram",
					pmetric.MetricTypeExponentialHistogram,
					"",
					[]dataPointExpectation{{
						exponentialHistogramComparator: []exponentialHistogramComparator{
							compareExponentialHistogram(3, 1214, 3456, 5, -3, []uint64{1, 0, 2}, 2, []uint64{1, 0, 0, 1}),
							checkNativeHistogramExemplars,
						},
					}},
					nil,
				},
			},
		},
		"feature disabled scrape classic on": {
			mutCfg: func(cfg *PromConfig) {
				for _, scrapeConfig := range cfg.ScrapeConfigs {
					scrapeConfig.AlwaysScrapeClassicHistograms = true
				}
			},
			enableNativeHistograms: false,
			expected: []metricExpectation{
				{ // Scrape classic only histograms as is.
					"test_classic_histogram",
					pmetric.MetricTypeHistogram,
					"",
					[]dataPointExpectation{{
						histogramPointComparator: []histogramPointComparator{
							compareHistogram(1213, 456, []float64{0.5, 10}, []uint64{789, 222, 202}),
							checkClassicHistogramExemplars,
						},
					}},
					nil,
				},
				{ // Only scrape classic buckets from mixed histograms.
					"test_mixed_histogram",
					pmetric.MetricTypeHistogram,
					"",
					[]dataPointExpectation{{
						histogramPointComparator: []histogramPointComparator{
							compareHistogram(1213, 456, []float64{0.5, 10}, []uint64{789, 222, 202}),
							checkMixedHistogramClassicExemplars,
						},
					}},
					nil,
				},
			},
		},
	}

	defer func() {
		_ = featuregate.GlobalRegistry().Set("receiver.prometheusreceiver.EnableNativeHistograms", false)
	}()

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := featuregate.GlobalRegistry().Set("receiver.prometheusreceiver.EnableNativeHistograms", tc.enableNativeHistograms)
			require.NoError(t, err)

			targets := []*testData{
				{
					name: "target1",
					pages: []mockPrometheusResponse{
						{code: 200, useProtoBuf: true, buf: buffer.Bytes()},
					},
					validateFunc: func(t *testing.T, td *testData, result []pmetric.ResourceMetrics) {
						verifyNumValidScrapeResults(t, td, result)
						doCompare(t, "target1", td.attributes, result[0], tc.expected)
					},
				},
			}
			mutCfg := tc.mutCfg
			if mutCfg == nil {
				mutCfg = func(*PromConfig) {}
			}
			testComponent(t, targets, func(c *Config) {
				c.PrometheusConfig.GlobalConfig.ScrapeProtocols = []config.ScrapeProtocol{config.PrometheusProto}
			}, mutCfg)
		})
	}
}

func TestStaleExponentialHistogram(t *testing.T) {
	mf := &dto.MetricFamily{
		Name: "test_counter",
		Type: dto.MetricType_COUNTER,
		Metric: []dto.Metric{
			{
				Label: []dto.LabelPair{
					{
						Name:  "foo",
						Value: "bar",
					},
				},
				Counter: &dto.Counter{
					Value: 1234,
				},
			},
		},
	}
	buffer1 := prometheusMetricFamilyToProtoBuf(t, nil, mf)
	nativeHistogram := &dto.MetricFamily{
		Name: "test_native_histogram",
		Type: dto.MetricType_HISTOGRAM,
		Metric: []dto.Metric{
			{
				Histogram: &dto.Histogram{
					SampleCount: 1213,
					SampleSum:   456,
					// Integer counter histogram definition
					Schema:        3,
					ZeroThreshold: 0.001,
					ZeroCount:     2,
					NegativeSpan: []dto.BucketSpan{
						{Offset: 0, Length: 1},
						{Offset: 1, Length: 1},
					},
					NegativeDelta: []int64{1, 1},
					PositiveSpan: []dto.BucketSpan{
						{Offset: -2, Length: 1},
						{Offset: 2, Length: 1},
					},
					PositiveDelta: []int64{1, 0},
				},
			},
		},
	}
	prometheusMetricFamilyToProtoBuf(t, buffer1, nativeHistogram)

	mf = &dto.MetricFamily{
		Name: "test_counter",
		Type: dto.MetricType_COUNTER,
		Metric: []dto.Metric{
			{
				Label: []dto.LabelPair{
					{
						Name:  "foo",
						Value: "bar",
					},
				},
				Counter: &dto.Counter{
					Value: 2222,
				},
			},
		},
	}
	buffer2 := prometheusMetricFamilyToProtoBuf(t, nil, mf)

	expectations1 := []metricExpectation{
		{
			"test_counter",
			pmetric.MetricTypeSum,
			"",
			nil,
			nil,
		},
		{
			"test_native_histogram",
			pmetric.MetricTypeExponentialHistogram,
			"",
			[]dataPointExpectation{{
				exponentialHistogramComparator: []exponentialHistogramComparator{
					compareExponentialHistogram(3, 1213, 456, 2, -1, []uint64{1, 0, 2}, -3, []uint64{1, 0, 0, 1}),
				},
			}},
			nil,
		},
	}

	expectations2 := []metricExpectation{
		{
			"test_counter",
			pmetric.MetricTypeSum,
			"",
			nil,
			nil,
		},
		{
			"test_native_histogram",
			pmetric.MetricTypeExponentialHistogram,
			"",
			[]dataPointExpectation{{
				exponentialHistogramComparator: []exponentialHistogramComparator{
					assertExponentialHistogramPointFlagNoRecordedValue(),
				},
			}},
			nil,
		},
	}

	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, useProtoBuf: true, buf: buffer1.Bytes()},
				{code: 200, useProtoBuf: true, buf: buffer2.Bytes()},
			},
			validateFunc: func(t *testing.T, td *testData, result []pmetric.ResourceMetrics) {
				verifyNumValidScrapeResults(t, td, result)
				doCompare(t, "target11", td.attributes, result[0], expectations1)
				doCompare(t, "target12", td.attributes, result[1], expectations2)
			},
		},
	}
	err := featuregate.GlobalRegistry().Set("receiver.prometheusreceiver.EnableNativeHistograms", true)
	require.NoError(t, err)
	defer func() {
		_ = featuregate.GlobalRegistry().Set("receiver.prometheusreceiver.EnableNativeHistograms", false)
	}()
	testComponent(t, targets, func(c *Config) {
		c.PrometheusConfig.GlobalConfig.ScrapeProtocols = []config.ScrapeProtocol{config.PrometheusProto}
	})
}

func TestFloatCounterHistogram(t *testing.T) {
	nativeHistogram := &dto.MetricFamily{
		Name: "test_native_histogram",
		Type: dto.MetricType_HISTOGRAM,
		Metric: []dto.Metric{
			{
				Histogram: &dto.Histogram{
					SampleCountFloat: 1213.0,
					SampleSum:        456,
					// Float counter histogram definition
					Schema:         -1,
					ZeroThreshold:  0.001,
					ZeroCountFloat: 2.0,
					NegativeSpan: []dto.BucketSpan{
						{Offset: 0, Length: 1},
						{Offset: 1, Length: 1},
					},
					NegativeCount: []float64{1.5, 2.5},
					PositiveSpan: []dto.BucketSpan{
						{Offset: -2, Length: 1},
						{Offset: 2, Length: 1},
					},
					PositiveCount: []float64{1.0, 3.0},
				},
			},
		},
	}
	buffer := prometheusMetricFamilyToProtoBuf(t, nil, nativeHistogram)

	expectations1 := []metricExpectation{
		{
			"test_native_histogram",
			pmetric.MetricTypeExponentialHistogram,
			"",
			[]dataPointExpectation{{
				exponentialHistogramComparator: []exponentialHistogramComparator{
					compareExponentialHistogram(-1, 1213, 456, 2, -1, []uint64{1, 0, 2}, -3, []uint64{1, 0, 0, 3}),
				},
			}},
			nil,
		},
	}

	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, useProtoBuf: true, buf: buffer.Bytes()},
			},
			validateFunc: func(t *testing.T, td *testData, result []pmetric.ResourceMetrics) {
				verifyNumValidScrapeResults(t, td, result)
				doCompare(t, "target1", td.attributes, result[0], expectations1)
			},
		},
	}
	err := featuregate.GlobalRegistry().Set("receiver.prometheusreceiver.EnableNativeHistograms", true)
	require.NoError(t, err)
	defer func() {
		_ = featuregate.GlobalRegistry().Set("receiver.prometheusreceiver.EnableNativeHistograms", false)
	}()
	testComponent(t, targets, func(c *Config) {
		c.PrometheusConfig.GlobalConfig.ScrapeProtocols = []config.ScrapeProtocol{config.PrometheusProto}
	})
}
