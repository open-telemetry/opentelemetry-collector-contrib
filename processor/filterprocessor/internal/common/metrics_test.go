// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common_test // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/common_test"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/common"
)

var (
	StartTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	StartTimestamp = pcommon.NewTimestampFromTime(StartTime)
	TestTime       = time.Date(2021, 3, 12, 21, 27, 13, 322, time.UTC)
	TestTimeStamp  = pcommon.NewTimestampFromTime(StartTime)
)

func TestFilterMetricProcessorWithOTTL(t *testing.T) {
	tests := []struct {
		name             string
		conditions       []string
		filterEverything bool
		want             func(md pmetric.Metrics)
		wantErr          bool
		errorMode        ottl.ErrorMode
	}{
		{
			name: "drop metrics",
			conditions: []string{
				`metric.name == "operationA"`,
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
			conditions: []string{
				`IsMatch(metric.name, "operation.*")`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "drop sum data point",
			conditions: []string{
				`metric.type == METRIC_DATA_TYPE_SUM and datapoint.value_double == 1.0`,
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
			conditions: []string{
				`metric.type == METRIC_DATA_TYPE_SUM`,
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
			conditions: []string{
				`metric.type == METRIC_DATA_TYPE_GAUGE and datapoint.value_double == 1.0`,
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
			conditions: []string{
				`metric.type == METRIC_DATA_TYPE_GAUGE`,
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
			conditions: []string{
				`metric.type == METRIC_DATA_TYPE_HISTOGRAM and datapoint.count == 1`,
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
			conditions: []string{
				`metric.type == METRIC_DATA_TYPE_HISTOGRAM`,
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
			conditions: []string{
				`metric.type == METRIC_DATA_TYPE_EXPONENTIAL_HISTOGRAM and datapoint.count == 1`,
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
			conditions: []string{
				`metric.type == METRIC_DATA_TYPE_EXPONENTIAL_HISTOGRAM`,
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
			conditions: []string{
				`metric.type == METRIC_DATA_TYPE_SUMMARY and datapoint.sum == 43.21`,
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
			conditions: []string{
				`metric.type == METRIC_DATA_TYPE_SUMMARY`,
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
			conditions: []string{
				`resource.attributes["not real"] == "unknown"`,
				`metric.type != nil`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "with error conditions",
			conditions: []string{
				`Substring("", 0, 100) == "test"`,
			},
			want:      func(_ pmetric.Metrics) {},
			errorMode: ottl.IgnoreError,
			wantErr:   true,
		},
		{
			name: "HasAttrOnDatapoint",
			conditions: []string{
				`metric.type != nil and HasAttrOnDatapoint("attr1", "test1")`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "HasAttrKeyOnDatapoint",
			conditions: []string{
				`metric.type != nil and HasAttrKeyOnDatapoint("attr1")`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "filters resource",
			conditions: []string{
				`resource.schema_url == "test_schema_url"`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collection, err := common.NewMetricParserCollection(componenttest.NewNopTelemetrySettings(), common.WithMetricParser(filterottl.StandardMetricFuncs()), common.WithDataPointParser(filterottl.StandardDataPointFuncs()))
			assert.NoError(t, err)
			got, err := collection.ParseContextConditions(common.ContextConditions{Conditions: tt.conditions, ErrorMode: tt.errorMode})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err, "error parsing conditions")
			finalMetrics := constructMetrics()
			consumeErr := got.ConsumeMetrics(context.Background(), finalMetrics)
			switch {
			case tt.filterEverything && !tt.wantErr:
				assert.Equal(t, processorhelper.ErrSkipProcessingData, consumeErr)
			case tt.wantErr:
				assert.Error(t, consumeErr)
			default:
				assert.NoError(t, consumeErr)
				exTd := constructMetrics()
				tt.want(exTd)
				assert.Equal(t, exTd, finalMetrics)
			}
		})
	}
}

func Test_ProcessMetrics_ConditionsErrorMode(t *testing.T) {
	tests := []struct {
		name          string
		errorMode     ottl.ErrorMode
		conditions    []common.ContextConditions
		want          func(md pmetric.Metrics)
		wantErr       bool
		wantErrorWith string
	}{
		{
			name:      "metric: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`metric.name == ParseJSON(1)`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`not IsMatch(metric.name, ".*")`}},
			},
			want: func(md pmetric.Metrics) {
				md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().RemoveIf(func(metric pmetric.Metric) bool {
					return len(metric.Name()) == 0
				})
			},
		},
		{
			name:      "metric: conditions group error mode does not affect default",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`metric.name == ParseJSON(1)`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`metric.name == ParseJSON(true)`}},
			},
			wantErrorWith: "expected string but got bool",
		},
		{
			name:      "resource: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`resource.attributes["pass"] == ParseJSON(1)`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`not IsMatch(resource.attributes["host.name"], ".*")`}},
			},
			want: func(md pmetric.Metrics) {
				md.ResourceMetrics().RemoveIf(func(rm pmetric.ResourceMetrics) bool {
					v, _ := rm.Resource().Attributes().Get("host.name")
					return len(v.AsString()) == 0
				})
			},
		},
		{
			name:      "resource: conditions group error mode does not affect default",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`resource.attributes["pass"] == ParseJSON(1)`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`resource.attributes["pass"] == ParseJSON(true)`}},
			},
			wantErrorWith: "expected string but got bool",
		},
		{
			name:      "scope: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`scope.attributes["pass"] == ParseJSON(1)`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`scope.schema_url != "test_schema_url"`}},
			},
			want: func(md pmetric.Metrics) {
				md.ResourceMetrics().At(0).ScopeMetrics().RemoveIf(func(sm pmetric.ScopeMetrics) bool {
					return sm.SchemaUrl() != "test_schema_url"
				})
			},
		},
		{
			name:      "scope: conditions group error mode does not affect default",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`scope.attributes["pass"] == ParseJSON(1)`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`scope.attributes["pass"] == ParseJSON(true)`}},
			},
			wantErrorWith: "expected string but got bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			collection, err := common.NewMetricParserCollection(componenttest.NewNopTelemetrySettings(), common.WithMetricParser(filterottl.StandardMetricFuncs()), common.WithDataPointParser(filterottl.StandardDataPointFuncs()), common.WithMetricErrorMode(tt.errorMode))
			assert.NoError(t, err)

			var consumers []common.MetricsConsumer
			for _, condition := range tt.conditions {
				consumer, err := collection.ParseContextConditions(condition)
				if tt.wantErr {
					require.Error(t, err)
					return
				}
				require.NoError(t, err, "error parsing conditions")
				consumers = append(consumers, consumer)
			}

			finalMetrics := constructMetrics()
			var consumeErr error

			// Apply each consumer sequentially
			for _, consumer := range consumers {
				if err := consumer.ConsumeMetrics(context.Background(), finalMetrics); err != nil {
					if err == processorhelper.ErrSkipProcessingData {
						consumeErr = err
						break
					}
					consumeErr = err
					break
				}
			}

			if tt.wantErrorWith != "" {
				if consumeErr == nil {
					t.Errorf("expected error containing '%s', got: <nil>", tt.wantErrorWith)
				} else {
					assert.Contains(t, consumeErr.Error(), tt.wantErrorWith)
				}
				return
			}

			if consumeErr != nil && consumeErr != processorhelper.ErrSkipProcessingData {
				assert.NoError(t, consumeErr)
				return
			}

			exTd := constructMetrics()
			tt.want(exTd)
			assert.Equal(t, exTd, finalMetrics)
		})
	}
}

func Test_ProcessMetrics_InferredResourceContext(t *testing.T) {
	tests := []struct {
		condition          string
		filteredEverything bool
		want               func(md pmetric.Metrics)
	}{
		{
			condition:          `resource.attributes["host.name"] == "myhost"`,
			filteredEverything: true,
			want: func(_ pmetric.Metrics) {
				// Everything should be filtered out
			},
		},
		{
			condition:          `resource.attributes["host.name"] == "wrong"`,
			filteredEverything: false,
			want: func(md pmetric.Metrics) {
				// Nothing should be filtered, original data remains
			},
		},
		{
			condition:          `resource.schema_url == "test_schema_url"`,
			filteredEverything: true,
			want: func(_ pmetric.Metrics) {
				// Everything should be filtered out since schema_url matches
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.condition, func(t *testing.T) {
			md := constructMetrics()

			collection, err := common.NewMetricParserCollection(componenttest.NewNopTelemetrySettings(), common.WithMetricParser(filterottl.StandardMetricFuncs()), common.WithDataPointParser(filterottl.StandardDataPointFuncs()))
			assert.NoError(t, err)

			consumer, err := collection.ParseContextConditions(common.ContextConditions{Conditions: []string{tt.condition}})
			assert.NoError(t, err)

			err = consumer.ConsumeMetrics(context.Background(), md)

			if tt.filteredEverything {
				assert.Equal(t, processorhelper.ErrSkipProcessingData, err)
			} else {
				assert.NoError(t, err)
				exTd := constructMetrics()
				tt.want(exTd)
				assert.Equal(t, exTd, md)
			}
		})
	}
}

func Test_ProcessMetrics_InferredScopeContext(t *testing.T) {
	tests := []struct {
		condition          string
		filteredEverything bool
		want               func(md pmetric.Metrics)
	}{
		{
			condition:          `scope.name == "scope"`,
			filteredEverything: true,
			want: func(_ pmetric.Metrics) {
				// Everything should be filtered out since scope name matches
			},
		},
		{
			condition:          `scope.version == "2"`,
			filteredEverything: false,
			want: func(md pmetric.Metrics) {
				// Nothing should be filtered, original data remains
			},
		},
		{
			condition:          `scope.schema_url == "test_schema_url"`,
			filteredEverything: true,
			want: func(_ pmetric.Metrics) {
				// Everything should be filtered out since schema_url matches
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.condition, func(t *testing.T) {
			md := constructMetrics()

			collection, err := common.NewMetricParserCollection(componenttest.NewNopTelemetrySettings(), common.WithMetricParser(filterottl.StandardMetricFuncs()), common.WithDataPointParser(filterottl.StandardDataPointFuncs()))
			assert.NoError(t, err)

			consumer, err := collection.ParseContextConditions(common.ContextConditions{Conditions: []string{tt.condition}})
			assert.NoError(t, err)

			err = consumer.ConsumeMetrics(context.Background(), md)

			if tt.filteredEverything {
				assert.Equal(t, processorhelper.ErrSkipProcessingData, err)
			} else {
				assert.NoError(t, err)
				exTd := constructMetrics()
				tt.want(exTd)
				assert.Equal(t, exTd, md)
			}
		})
	}
}

func constructMetrics() pmetric.Metrics {
	td := pmetric.NewMetrics()
	rm0 := td.ResourceMetrics().AppendEmpty()
	rm0.SetSchemaUrl("test_schema_url")
	rm0.Resource().Attributes().PutStr("host.name", "myhost")
	rm0ils0 := rm0.ScopeMetrics().AppendEmpty()
	rm0ils0.SetSchemaUrl("test_schema_url")
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
	dataPoint0.SetStartTimestamp(StartTimestamp)
	dataPoint0.SetDoubleValue(1.0)
	dataPoint0.Attributes().PutStr("attr1", "test1")
	dataPoint0.Attributes().PutStr("attr2", "test2")
	dataPoint0.Attributes().PutStr("attr3", "test3")
	dataPoint0.Attributes().PutStr("flags", "A|B|C")
	dataPoint0.Attributes().PutStr("total.string", "123456789")

	dataPoint1 := m.Sum().DataPoints().AppendEmpty()
	dataPoint1.SetStartTimestamp(StartTimestamp)
	dataPoint1.SetDoubleValue(3.7)
	dataPoint1.Attributes().PutStr("attr1", "test1")
	dataPoint1.Attributes().PutStr("attr2", "test2")
	dataPoint1.Attributes().PutStr("attr3", "test3")
	dataPoint1.Attributes().PutStr("flags", "A|B|C")
	dataPoint1.Attributes().PutStr("total.string", "123456789")
}

func fillMetricTwo(m pmetric.Metric) {
	m.SetName("operationB")
	m.SetDescription("operationB description")
	m.SetUnit("operationB unit")
	m.SetEmptyHistogram()
	m.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	dataPoint0 := m.Histogram().DataPoints().AppendEmpty()
	dataPoint0.SetStartTimestamp(StartTimestamp)
	dataPoint0.Attributes().PutStr("attr1", "test1")
	dataPoint0.Attributes().PutStr("attr2", "test2")
	dataPoint0.Attributes().PutStr("attr3", "test3")
	dataPoint0.Attributes().PutStr("flags", "C|D")
	dataPoint0.Attributes().PutStr("total.string", "345678")
	dataPoint0.SetCount(1)
	dataPoint0.SetSum(5)

	dataPoint1 := m.Histogram().DataPoints().AppendEmpty()
	dataPoint1.SetStartTimestamp(StartTimestamp)
	dataPoint1.Attributes().PutStr("attr1", "test1")
	dataPoint1.Attributes().PutStr("attr2", "test2")
	dataPoint1.Attributes().PutStr("attr3", "test3")
	dataPoint1.Attributes().PutStr("flags", "C|D")
	dataPoint1.Attributes().PutStr("total.string", "345678")
	dataPoint1.SetCount(3)
}

func fillMetricThree(m pmetric.Metric) {
	m.SetName("operationC")
	m.SetDescription("operationC description")
	m.SetUnit("operationC unit")

	dataPoint0 := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
	dataPoint0.SetStartTimestamp(StartTimestamp)
	dataPoint0.Attributes().PutStr("attr1", "test1")
	dataPoint0.Attributes().PutStr("attr2", "test2")
	dataPoint0.Attributes().PutStr("attr3", "test3")
	dataPoint0.SetCount(1)
	dataPoint0.SetScale(1)
	dataPoint0.SetZeroCount(1)
	dataPoint0.Positive().SetOffset(1)
	dataPoint0.Negative().SetOffset(1)

	dataPoint1 := m.ExponentialHistogram().DataPoints().AppendEmpty()
	dataPoint1.SetStartTimestamp(StartTimestamp)
	dataPoint1.Attributes().PutStr("attr1", "test1")
	dataPoint1.Attributes().PutStr("attr2", "test2")
	dataPoint1.Attributes().PutStr("attr3", "test3")
}

func fillMetricFour(m pmetric.Metric) {
	m.SetName("operationD")
	m.SetDescription("operationD description")
	m.SetUnit("operationD unit")

	dataPoint0 := m.SetEmptySummary().DataPoints().AppendEmpty()
	dataPoint0.SetStartTimestamp(StartTimestamp)
	dataPoint0.SetTimestamp(TestTimeStamp)
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
}

func fillMetricFive(m pmetric.Metric) {
	m.SetName("operationE")
	m.SetDescription("operationE description")
	m.SetUnit("operationE unit")

	dataPoint0 := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dataPoint0.SetStartTimestamp(StartTimestamp)
	dataPoint0.SetDoubleValue(1.0)
	dataPoint0.Attributes().PutStr("attr1", "test1")

	dataPoint1 := m.Gauge().DataPoints().AppendEmpty()
	dataPoint1.SetStartTimestamp(StartTimestamp)
	dataPoint1.SetDoubleValue(3.7)
	dataPoint1.Attributes().PutStr("attr1", "test1")
}
