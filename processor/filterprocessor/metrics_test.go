// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprocessor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/goldendataset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/metadatatest"
)

type metricNameTest struct {
	name      string
	inc       *filterconfig.MetricMatchProperties
	exc       *filterconfig.MetricMatchProperties
	inMetrics pmetric.Metrics
	outMN     [][]string // output Metric names per Resource
}

type metricWithResource struct {
	metricNames        []string
	resourceAttributes map[string]any
}

var (
	validFilters = []string{
		"prefix/.*",
		"prefix_.*",
		".*/suffix",
		".*_suffix",
		".*/contains/.*",
		".*_contains_.*",
		"full/name/match",
		"full_name_match",
	}

	inMetricNames = []string{
		"full_name_match",
		"not_exact_string_match",
		"prefix/test/match",
		"prefix_test_match",
		"prefixprefix/test/match",
		"test/match/suffix",
		"test_match_suffix",
		"test/match/suffixsuffix",
		"test/contains/match",
		"test_contains_match",
		"random",
		"full/name/match",
		"full_name_match", // repeats
		"not_exact_string_match",
	}

	inMetricForResourceTest = []metricWithResource{
		{
			metricNames: []string{"metric1", "metric2"},
			resourceAttributes: map[string]any{
				"attr1": "attr1/val1",
				"attr2": "attr2/val2",
				"attr3": "attr3/val3",
			},
		},
	}

	inMetricForTwoResource = []metricWithResource{
		{
			metricNames: []string{"metric1", "metric2"},
			resourceAttributes: map[string]any{
				"attr1": "attr1/val1",
			},
		},
		{
			metricNames: []string{"metric3", "metric4"},
			resourceAttributes: map[string]any{
				"attr1": "attr1/val2",
			},
		},
	}

	regexpMetricsFilterProperties = &filterconfig.MetricMatchProperties{
		MatchType:   filterconfig.MetricRegexp,
		MetricNames: validFilters,
	}

	standardTests = []metricNameTest{
		{
			name:      "includeFilter",
			inc:       regexpMetricsFilterProperties,
			inMetrics: testResourceMetrics([]metricWithResource{{metricNames: inMetricNames}}),
			outMN: [][]string{{
				"full_name_match",
				"prefix/test/match",
				"prefix_test_match",
				"prefixprefix/test/match",
				"test/match/suffix",
				"test_match_suffix",
				"test/match/suffixsuffix",
				"test/contains/match",
				"test_contains_match",
				"full/name/match",
				"full_name_match",
			}},
		},
		{
			name:      "excludeFilter",
			exc:       regexpMetricsFilterProperties,
			inMetrics: testResourceMetrics([]metricWithResource{{metricNames: inMetricNames}}),
			outMN: [][]string{{
				"not_exact_string_match",
				"random",
				"not_exact_string_match",
			}},
		},
		{
			name: "includeAndExclude",
			inc:  regexpMetricsFilterProperties,
			exc: &filterconfig.MetricMatchProperties{
				MatchType: filterconfig.MetricStrict,
				MetricNames: []string{
					"prefix_test_match",
					"test_contains_match",
				},
			},
			inMetrics: testResourceMetrics([]metricWithResource{{metricNames: inMetricNames}}),
			outMN: [][]string{{
				"full_name_match",
				"prefix/test/match",
				// "prefix_test_match", excluded by exclude filter
				"prefixprefix/test/match",
				"test/match/suffix",
				"test_match_suffix",
				"test/match/suffixsuffix",
				"test/contains/match",
				// "test_contains_match", excluded by exclude filter
				"full/name/match",
				"full_name_match",
			}},
		},
		{
			name: "includeAndExcludeWithEmptyResourceMetrics",
			inc:  regexpMetricsFilterProperties,
			exc: &filterconfig.MetricMatchProperties{
				MatchType: filterconfig.MetricStrict,
				MetricNames: []string{
					"prefix_test_match",
					"test_contains_match",
				},
			},
			inMetrics: testResourceMetrics([]metricWithResource{{}, {metricNames: inMetricNames}}),
			outMN: [][]string{
				{
					"full_name_match",
					"prefix/test/match",
					// "prefix_test_match", excluded by exclude filter
					"prefixprefix/test/match",
					"test/match/suffix",
					"test_match_suffix",
					"test/match/suffixsuffix",
					"test/contains/match",
					// "test_contains_match", excluded by exclude filter
					"full/name/match",
					"full_name_match",
				},
			},
		},
		{
			name:      "emptyFilterInclude",
			inc:       &filterconfig.MetricMatchProperties{MatchType: filterconfig.MetricStrict},
			inMetrics: testResourceMetrics([]metricWithResource{{metricNames: inMetricNames}}),
			outMN:     [][]string{inMetricNames},
		},
		{
			name:      "emptyFilterExclude",
			exc:       &filterconfig.MetricMatchProperties{MatchType: filterconfig.MetricStrict},
			inMetrics: testResourceMetrics([]metricWithResource{{metricNames: inMetricNames}}),
			outMN:     [][]string{inMetricNames},
		},
		{
			name:      "includeWithNilResourceAttributes",
			inc:       regexpMetricsFilterProperties,
			inMetrics: testResourceMetrics([]metricWithResource{{metricNames: inMetricNames}}),
			outMN: [][]string{{
				"full_name_match",
				"prefix/test/match",
				"prefix_test_match",
				"prefixprefix/test/match",
				"test/match/suffix",
				"test_match_suffix",
				"test/match/suffixsuffix",
				"test/contains/match",
				"test_contains_match",
				"full/name/match",
				"full_name_match",
			}},
		},
		{
			name: "excludeNilWithResourceAttributes",
			exc: &filterconfig.MetricMatchProperties{
				MatchType: filterconfig.MetricStrict,
			},
			inMetrics: testResourceMetrics(inMetricForResourceTest),
			outMN: [][]string{
				{"metric1", "metric2"},
			},
		},
		{
			name: "includeAllWithResourceAttributes",
			inc: &filterconfig.MetricMatchProperties{
				MatchType: filterconfig.MetricStrict,
				MetricNames: []string{
					"metric1",
					"metric2",
				},
				ResourceAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val1"}},
			},
			inMetrics: testResourceMetrics(inMetricForResourceTest),
			outMN: [][]string{
				{"metric1", "metric2"},
			},
		},
		{
			name: "includeAllWithMissingResourceAttributes",
			inc: &filterconfig.MetricMatchProperties{
				MatchType: filterconfig.MetricStrict,
				MetricNames: []string{
					"metric1",
					"metric2",
					"metric3",
					"metric4",
				},
				ResourceAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val1"}},
			},
			inMetrics: testResourceMetrics(inMetricForTwoResource),
			outMN: [][]string{
				{"metric1", "metric2"},
			},
		},
		{
			name: "excludeAllWithMissingResourceAttributes",
			exc: &filterconfig.MetricMatchProperties{
				MatchType:          filterconfig.MetricStrict,
				ResourceAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val1"}},
			},
			inMetrics: testResourceMetrics(inMetricForTwoResource),
			outMN: [][]string{
				{"metric3", "metric4"},
			},
		},
		{
			name: "includeWithRegexResourceAttributes",
			inc: &filterconfig.MetricMatchProperties{
				MatchType: filterconfig.MetricRegexp,
				MetricNames: []string{
					".*",
				},
				ResourceAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val1"}},
			},
			inMetrics: testResourceMetrics(inMetricForTwoResource),
			outMN: [][]string{
				{"metric1", "metric2"},
			},
		},
		{
			name: "includeWithRegexResourceAttributesOnly",
			inc: &filterconfig.MetricMatchProperties{
				MatchType:          filterconfig.MetricRegexp,
				ResourceAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val1"}},
			},
			inMetrics: testResourceMetrics(inMetricForTwoResource),
			outMN: [][]string{
				{"metric1", "metric2"},
			},
		},
		{
			name: "includeWithStrictResourceAttributes",
			inc: &filterconfig.MetricMatchProperties{
				MatchType: filterconfig.MetricStrict,
				MetricNames: []string{
					"metric1",
					"metric2",
				},
				ResourceAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val1"}},
			},
			inMetrics: testResourceMetrics(inMetricForTwoResource),
			outMN: [][]string{
				{"metric1", "metric2"},
			},
		},
		{
			name: "includeWithStrictResourceAttributesOnly",
			inc: &filterconfig.MetricMatchProperties{
				MatchType:          filterconfig.MetricStrict,
				ResourceAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val1"}},
			},
			inMetrics: testResourceMetrics(inMetricForTwoResource),
			outMN: [][]string{
				{"metric1", "metric2"},
			},
		},
	}
)

func TestFilterMetricProcessor(t *testing.T) {
	for _, test := range standardTests {
		t.Run(test.name, func(t *testing.T) {
			// next stores the results of the filter metric processor
			next := new(consumertest.MetricsSink)
			cfg := &Config{
				Metrics: MetricFilters{
					Include: test.inc,
					Exclude: test.exc,
				},
			}
			factory := NewFactory()
			fmp, err := factory.CreateMetrics(
				context.Background(),
				processortest.NewNopSettings(metadata.Type),
				cfg,
				next,
			)
			assert.NotNil(t, fmp)
			assert.NoError(t, err)

			caps := fmp.Capabilities()
			assert.True(t, caps.MutatesData)
			ctx := context.Background()
			assert.NoError(t, fmp.Start(ctx, nil))

			cErr := fmp.ConsumeMetrics(context.Background(), test.inMetrics)
			assert.NoError(t, cErr)
			got := next.AllMetrics()

			if len(test.outMN) == 0 {
				require.Empty(t, got)
				return
			}

			require.Len(t, got, 1)
			require.Equal(t, len(test.outMN), got[0].ResourceMetrics().Len())
			for i, wantOut := range test.outMN {
				gotMetrics := got[0].ResourceMetrics().At(i).ScopeMetrics().At(0).Metrics()
				assert.Equal(t, len(wantOut), gotMetrics.Len())
				for idx := range wantOut {
					assert.Equal(t, wantOut[idx], gotMetrics.At(idx).Name())
				}
			}
			assert.NoError(t, fmp.Shutdown(ctx))
		})
	}
}

func TestFilterMetricProcessorTelemetry(t *testing.T) {
	tel := componenttest.NewTelemetry()
	cfg := &Config{
		Metrics: MetricFilters{
			MetricConditions: []string{
				"name==\"metric1\"",
			},
		},
	}
	fmp, err := newFilterMetricProcessor(metadatatest.NewSettings(tel), cfg)
	assert.NotNil(t, fmp)
	assert.NoError(t, err)

	_, err = fmp.processMetrics(context.Background(), testResourceMetrics([]metricWithResource{
		{
			metricNames: []string{"metric1", "metric2"},
			resourceAttributes: map[string]any{
				"attr1": "attr1/val1",
			},
		},
	}))
	assert.NoError(t, err)

	metadatatest.AssertEqualProcessorFilterDatapointsFiltered(t, tel, []metricdata.DataPoint[int64]{
		{
			Value:      1,
			Attributes: attribute.NewSet(attribute.String("filter", "filter")),
		},
	}, metricdatatest.IgnoreTimestamp())
	require.NoError(t, tel.Shutdown(context.Background()))
}

func testResourceMetrics(mwrs []metricWithResource) pmetric.Metrics {
	md := pmetric.NewMetrics()
	now := time.Now()

	for _, mwr := range mwrs {
		rm := md.ResourceMetrics().AppendEmpty()
		//nolint:errcheck
		rm.Resource().Attributes().FromRaw(mwr.resourceAttributes)
		ms := rm.ScopeMetrics().AppendEmpty().Metrics()
		for _, name := range mwr.metricNames {
			m := ms.AppendEmpty()
			m.SetName(name)
			dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(10 * time.Second)))
			dp.SetDoubleValue(123)
		}
	}
	return md
}

func BenchmarkStrictFilter(b *testing.B) {
	mp := &filterconfig.MetricMatchProperties{
		MatchType:   "strict",
		MetricNames: []string{"p10_metric_0"},
	}
	benchmarkFilter(b, mp)
}

func BenchmarkRegexpFilter(b *testing.B) {
	mp := &filterconfig.MetricMatchProperties{
		MatchType:   "regexp",
		MetricNames: []string{"p10_metric_0"},
	}
	benchmarkFilter(b, mp)
}

func BenchmarkExprFilter(b *testing.B) {
	mp := &filterconfig.MetricMatchProperties{
		MatchType:   "expr",
		Expressions: []string{`MetricName == "p10_metric_0"`},
	}
	benchmarkFilter(b, mp)
}

func benchmarkFilter(b *testing.B, mp *filterconfig.MetricMatchProperties) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	pcfg := cfg.(*Config)
	pcfg.Metrics = MetricFilters{
		Exclude: mp,
	}
	ctx := context.Background()
	proc, _ := factory.CreateMetrics(
		ctx,
		processortest.NewNopSettings(metadata.Type),
		cfg,
		consumertest.NewNop(),
	)
	pdms := metricSlice(128)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, pdm := range pdms {
			_ = proc.ConsumeMetrics(ctx, pdm)
		}
	}
}

func metricSlice(numMetrics int) []pmetric.Metrics {
	var out []pmetric.Metrics
	for i := 0; i < numMetrics; i++ {
		const size = 2
		out = append(out, pdm(fmt.Sprintf("p%d_", i), size))
	}
	return out
}

func pdm(prefix string, size int) pmetric.Metrics {
	c := goldendataset.MetricsCfg{
		MetricDescriptorType: pmetric.MetricTypeGauge,
		MetricValueType:      pmetric.NumberDataPointValueTypeInt,
		MetricNamePrefix:     prefix,
		NumILMPerResource:    size,
		NumMetricsPerILM:     size,
		NumPtLabels:          size,
		NumPtsPerMetric:      size,
		NumResourceAttrs:     size,
		NumResourceMetrics:   size,
	}
	return goldendataset.MetricsFromCfg(c)
}

func TestNilResourceMetrics(t *testing.T) {
	metrics := pmetric.NewMetrics()
	rms := metrics.ResourceMetrics()
	rms.AppendEmpty()
	requireNotPanics(t, metrics)
}

func TestNilILM(t *testing.T) {
	metrics := pmetric.NewMetrics()
	rms := metrics.ResourceMetrics()
	rm := rms.AppendEmpty()
	ilms := rm.ScopeMetrics()
	ilms.AppendEmpty()
	requireNotPanics(t, metrics)
}

func TestNilMetric(t *testing.T) {
	metrics := pmetric.NewMetrics()
	rms := metrics.ResourceMetrics()
	rm := rms.AppendEmpty()
	ilms := rm.ScopeMetrics()
	ilm := ilms.AppendEmpty()
	ms := ilm.Metrics()
	ms.AppendEmpty()
	requireNotPanics(t, metrics)
}

func requireNotPanics(t *testing.T, metrics pmetric.Metrics) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	pcfg := cfg.(*Config)
	pcfg.Metrics = MetricFilters{
		Exclude: &filterconfig.MetricMatchProperties{
			MatchType:   "strict",
			MetricNames: []string{"foo"},
		},
	}
	ctx := context.Background()
	proc, _ := factory.CreateMetrics(
		ctx,
		processortest.NewNopSettings(metadata.Type),
		cfg,
		consumertest.NewNop(),
	)
	require.NotPanics(t, func() {
		_ = proc.ConsumeMetrics(ctx, metrics)
	})
}

var (
	dataPointStartTimestamp = pcommon.NewTimestampFromTime(time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC))
	dataPointTestTimeStamp  = pcommon.NewTimestampFromTime(time.Date(2021, 3, 12, 21, 27, 13, 322, time.UTC))
)

func TestFilterMetricProcessorWithOTTL(t *testing.T) {
	tests := []struct {
		name             string
		conditions       MetricFilters
		filterEverything bool
		want             func(md pmetric.Metrics)
		errorMode        ottl.ErrorMode
	}{
		{
			name: "drop metrics",
			conditions: MetricFilters{
				MetricConditions: []string{
					`name == "operationA"`,
				},
			},
			want: func(md pmetric.Metrics) {
				md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().RemoveIf(func(metric pmetric.Metric) bool {
					return metric.Name() == "operationA"
				})
			},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "drop everything by dropping all metrics",
			conditions: MetricFilters{
				MetricConditions: []string{
					`IsMatch(name, "operation.*")`,
				},
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "drop sum data point",
			conditions: MetricFilters{
				DataPointConditions: []string{
					`metric.type == METRIC_DATA_TYPE_SUM and value_double == 1.0`,
				},
			},
			want: func(md pmetric.Metrics) {
				md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().RemoveIf(func(point pmetric.NumberDataPoint) bool {
					return point.DoubleValue() == 1.0
				})
			},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "drop all sum data points",
			conditions: MetricFilters{
				DataPointConditions: []string{
					`metric.type == METRIC_DATA_TYPE_SUM`,
				},
			},
			want: func(md pmetric.Metrics) {
				md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().RemoveIf(func(metric pmetric.Metric) bool {
					return metric.Type() == pmetric.MetricTypeSum
				})
			},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "drop gauge data point",
			conditions: MetricFilters{
				DataPointConditions: []string{
					`metric.type == METRIC_DATA_TYPE_GAUGE and value_double == 1.0`,
				},
			},
			want: func(md pmetric.Metrics) {
				md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Gauge().DataPoints().RemoveIf(func(point pmetric.NumberDataPoint) bool {
					return point.DoubleValue() == 1.0
				})
			},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "drop all gauge data points",
			conditions: MetricFilters{
				DataPointConditions: []string{
					`metric.type == METRIC_DATA_TYPE_GAUGE`,
				},
			},
			want: func(md pmetric.Metrics) {
				md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().RemoveIf(func(metric pmetric.Metric) bool {
					return metric.Type() == pmetric.MetricTypeGauge
				})
			},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "drop histogram data point",
			conditions: MetricFilters{
				DataPointConditions: []string{
					`metric.type == METRIC_DATA_TYPE_HISTOGRAM and count == 1`,
				},
			},
			want: func(md pmetric.Metrics) {
				md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().RemoveIf(func(point pmetric.HistogramDataPoint) bool {
					return point.Count() == 1
				})
			},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "drop all histogram data points",
			conditions: MetricFilters{
				DataPointConditions: []string{
					`metric.type == METRIC_DATA_TYPE_HISTOGRAM`,
				},
			},
			want: func(md pmetric.Metrics) {
				md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().RemoveIf(func(metric pmetric.Metric) bool {
					return metric.Type() == pmetric.MetricTypeHistogram
				})
			},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "drop exponential histogram data point",
			conditions: MetricFilters{
				DataPointConditions: []string{
					`metric.type == METRIC_DATA_TYPE_EXPONENTIAL_HISTOGRAM and count == 1`,
				},
			},
			want: func(md pmetric.Metrics) {
				md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().RemoveIf(func(point pmetric.ExponentialHistogramDataPoint) bool {
					return point.Count() == 1
				})
			},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "drop all exponential histogram data points",
			conditions: MetricFilters{
				DataPointConditions: []string{
					`metric.type == METRIC_DATA_TYPE_EXPONENTIAL_HISTOGRAM`,
				},
			},
			want: func(md pmetric.Metrics) {
				md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().RemoveIf(func(metric pmetric.Metric) bool {
					return metric.Type() == pmetric.MetricTypeExponentialHistogram
				})
			},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "drop summary data point",
			conditions: MetricFilters{
				DataPointConditions: []string{
					`metric.type == METRIC_DATA_TYPE_SUMMARY and sum == 43.21`,
				},
			},
			want: func(md pmetric.Metrics) {
				md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().RemoveIf(func(point pmetric.SummaryDataPoint) bool {
					return point.Sum() == 43.21
				})
			},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "drop all summary data points",
			conditions: MetricFilters{
				DataPointConditions: []string{
					`metric.type == METRIC_DATA_TYPE_SUMMARY`,
				},
			},
			want: func(md pmetric.Metrics) {
				md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().RemoveIf(func(metric pmetric.Metric) bool {
					return metric.Type() == pmetric.MetricTypeSummary
				})
			},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "multiple conditions",
			conditions: MetricFilters{
				MetricConditions: []string{
					`resource.attributes["not real"] == "unknown"`,
					`type != nil`,
				},
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "with error conditions",
			conditions: MetricFilters{
				MetricConditions: []string{
					`Substring("", 0, 100) == "test"`,
				},
			},
			want:      func(_ pmetric.Metrics) {},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "HasAttrOnDatapoint",
			conditions: MetricFilters{
				MetricConditions: []string{
					`HasAttrOnDatapoint("attr1", "test1")`,
				},
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "HasAttrKeyOnDatapoint",
			conditions: MetricFilters{
				MetricConditions: []string{
					`HasAttrKeyOnDatapoint("attr1")`,
				},
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor, err := newFilterMetricProcessor(processortest.NewNopSettings(metadata.Type), &Config{Metrics: tt.conditions, ErrorMode: tt.errorMode})
			assert.NoError(t, err)

			got, err := processor.processMetrics(context.Background(), constructMetrics())

			if tt.filterEverything {
				assert.Equal(t, processorhelper.ErrSkipProcessingData, err)
			} else {
				exTd := constructMetrics()
				tt.want(exTd)
				assert.Equal(t, exTd, got)
			}
		})
	}
}

func constructMetrics() pmetric.Metrics {
	td := pmetric.NewMetrics()
	rm0 := td.ResourceMetrics().AppendEmpty()
	rm0.Resource().Attributes().PutStr("host.name", "myhost")
	rm0ils0 := rm0.ScopeMetrics().AppendEmpty()
	rm0ils0.Scope().SetName("scope")
	fillMetricOne(rm0ils0.Metrics().AppendEmpty())
	fillMetricTwo(rm0ils0.Metrics().AppendEmpty())
	fillMetricThree(rm0ils0.Metrics().AppendEmpty())
	fillMetricFour(rm0ils0.Metrics().AppendEmpty())
	fillMetricFive(rm0ils0.Metrics().AppendEmpty())
	return td
}

func fillMetricOne(m pmetric.Metric) {
	m.SetName("operationA")
	m.SetDescription("operationA description")
	m.SetUnit("operationA unit")

	dataPoint0 := m.SetEmptySum().DataPoints().AppendEmpty()
	dataPoint0.SetStartTimestamp(dataPointStartTimestamp)
	dataPoint0.SetDoubleValue(1.0)
	dataPoint0.Attributes().PutStr("attr1", "test1")
	dataPoint0.Attributes().PutStr("attr2", "test2")
	dataPoint0.Attributes().PutStr("attr3", "test3")
	dataPoint0.Attributes().PutStr("flags", "A|B|C")

	dataPoint1 := m.Sum().DataPoints().AppendEmpty()
	dataPoint1.SetStartTimestamp(dataPointStartTimestamp)
	dataPoint1.SetDoubleValue(3.7)
	dataPoint1.Attributes().PutStr("attr1", "test1")
	dataPoint1.Attributes().PutStr("attr2", "test2")
	dataPoint1.Attributes().PutStr("attr3", "test3")
	dataPoint1.Attributes().PutStr("flags", "A|B|C")
}

func fillMetricTwo(m pmetric.Metric) {
	m.SetName("operationB")
	m.SetDescription("operationB description")
	m.SetUnit("operationB unit")

	dataPoint0 := m.SetEmptyHistogram().DataPoints().AppendEmpty()
	dataPoint0.SetStartTimestamp(dataPointStartTimestamp)
	dataPoint0.Attributes().PutStr("attr1", "test1")
	dataPoint0.Attributes().PutStr("attr2", "test2")
	dataPoint0.Attributes().PutStr("attr3", "test3")
	dataPoint0.Attributes().PutStr("flags", "C|D")
	dataPoint0.SetCount(1)

	dataPoint1 := m.Histogram().DataPoints().AppendEmpty()
	dataPoint1.SetStartTimestamp(dataPointStartTimestamp)
	dataPoint1.Attributes().PutStr("attr1", "test1")
	dataPoint1.Attributes().PutStr("attr2", "test2")
	dataPoint1.Attributes().PutStr("attr3", "test3")
	dataPoint1.Attributes().PutStr("flags", "C|D")
}

func fillMetricThree(m pmetric.Metric) {
	m.SetName("operationC")
	m.SetDescription("operationC description")
	m.SetUnit("operationC unit")

	dataPoint0 := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
	dataPoint0.SetStartTimestamp(dataPointStartTimestamp)
	dataPoint0.Attributes().PutStr("attr1", "test1")
	dataPoint0.Attributes().PutStr("attr2", "test2")
	dataPoint0.Attributes().PutStr("attr3", "test3")
	dataPoint0.SetCount(1)
	dataPoint0.SetScale(1)
	dataPoint0.SetZeroCount(1)
	dataPoint0.Positive().SetOffset(1)
	dataPoint0.Negative().SetOffset(1)

	dataPoint1 := m.ExponentialHistogram().DataPoints().AppendEmpty()
	dataPoint1.SetStartTimestamp(dataPointStartTimestamp)
	dataPoint1.Attributes().PutStr("attr1", "test1")
	dataPoint1.Attributes().PutStr("attr2", "test2")
	dataPoint1.Attributes().PutStr("attr3", "test3")
}

func fillMetricFour(m pmetric.Metric) {
	m.SetName("operationD")
	m.SetDescription("operationD description")
	m.SetUnit("operationD unit")

	dataPoint0 := m.SetEmptySummary().DataPoints().AppendEmpty()
	dataPoint0.SetStartTimestamp(dataPointStartTimestamp)
	dataPoint0.SetTimestamp(dataPointTestTimeStamp)
	dataPoint0.Attributes().PutStr("attr1", "test1")
	dataPoint0.Attributes().PutStr("attr2", "test2")
	dataPoint0.Attributes().PutStr("attr3", "test3")
	dataPoint0.SetCount(1234)
	dataPoint0.SetSum(12.34)

	quantileDataPoint0 := dataPoint0.QuantileValues().AppendEmpty()
	quantileDataPoint0.SetQuantile(.99)
	quantileDataPoint0.SetValue(123)

	quantileDataPoint1 := dataPoint0.QuantileValues().AppendEmpty()
	quantileDataPoint1.SetQuantile(.95)
	quantileDataPoint1.SetValue(321)

	dataPoint1 := m.Summary().DataPoints().AppendEmpty()
	dataPoint1.SetSum(43.21)
}

func fillMetricFive(m pmetric.Metric) {
	m.SetName("operationE")
	m.SetDescription("operationE description")
	m.SetUnit("operationE unit")

	dataPoint0 := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dataPoint0.SetStartTimestamp(dataPointStartTimestamp)
	dataPoint0.SetDoubleValue(1.0)
	dataPoint0.Attributes().PutStr("attr1", "test1")
	dataPoint0.Attributes().PutStr("attr2", "test2")
	dataPoint0.Attributes().PutStr("attr3", "test3")

	dataPoint1 := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dataPoint1.SetStartTimestamp(dataPointStartTimestamp)
	dataPoint1.SetDoubleValue(2.0)
	dataPoint1.Attributes().PutStr("attr1", "test1")
	dataPoint1.Attributes().PutStr("attr2", "test2")
	dataPoint1.Attributes().PutStr("attr3", "test3")
}

func Test_ResourceSkipExpr_With_Bridge(t *testing.T) {
	tests := []struct {
		name      string
		condition *filterconfig.MatchConfig
	}{
		// resource attributes
		{
			name: "single static resource attribute include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: "svcA",
						},
					},
				},
			},
		},
		{
			name: "multiple static resource attribute include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},

					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: "svc2",
						},
						{
							Key:   "service.version",
							Value: "v1",
						},
					},
				},
			},
		},
		{
			name: "single regex resource attribute include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: "svc.*",
						},
					},
				},
			},
		},
		{
			name: "multiple regex resource attribute include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: ".*2",
						},
						{
							Key:   "service.name",
							Value: ".*3",
						},
					},
				},
			},
		},
		{
			name: "single static resource attribute exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: "svcA",
						},
					},
				},
			},
		},
		{
			name: "multiple static resource attribute exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},

					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: "svc2",
						},
						{
							Key:   "service.version",
							Value: "v1",
						},
					},
				},
			},
		},
		{
			name: "single regex resource attribute exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: "svc.*",
						},
					},
				},
			},
		},
		{
			name: "multiple regex resource attribute exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: ".*2",
						},
						{
							Key:   "service.name",
							Value: ".*3",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := pcommon.NewResource()
			resource.Attributes().PutStr("test", "test")

			tCtx := ottlresource.NewTransformContext(resource, pmetric.NewResourceMetrics())

			includeMatchProperties, err := filterconfig.CreateMetricMatchPropertiesFromDefault(tt.condition.Include)
			assert.NoError(t, err)
			excludeMatchProperties, err := filterconfig.CreateMetricMatchPropertiesFromDefault(tt.condition.Exclude)
			assert.NoError(t, err)

			boolExpr, err := newSkipResExpr(includeMatchProperties, excludeMatchProperties)
			require.NoError(t, err)
			expectedResult, err := boolExpr.Eval(context.Background(), tCtx)
			assert.NoError(t, err)

			ottlBoolExpr, err := filterottl.NewResourceSkipExprBridge(tt.condition)

			assert.NoError(t, err)
			ottlResult, err := ottlBoolExpr.Eval(context.Background(), tCtx)
			assert.NoError(t, err)

			assert.Equal(t, expectedResult, ottlResult)
		})
	}
}
