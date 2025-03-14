// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

var (
	StartTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	StartTimestamp = pcommon.NewTimestampFromTime(StartTime)
	TestTime       = time.Date(2021, 3, 12, 21, 27, 13, 322, time.UTC)
	TestTimeStamp  = pcommon.NewTimestampFromTime(StartTime)
)

func Test_ProcessMetrics_ResourceContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td pmetric.Metrics)
	}{
		{
			statement: `set(attributes["test"], "pass")`,
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).Resource().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where attributes["host.name"] == "wrong"`,
			want: func(_ pmetric.Metrics) {
			},
		},
		{
			statement: `set(schema_url, "test_schema_url")`,
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).SetSchemaUrl("test_schema_url")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructMetrics()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "resource", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessMetrics(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructMetrics()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessMetrics_InferredResourceContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td pmetric.Metrics)
	}{
		{
			statement: `set(resource.attributes["test"], "pass")`,
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).Resource().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(resource.attributes["test"], "pass") where resource.attributes["host.name"] == "wrong"`,
			want: func(_ pmetric.Metrics) {
			},
		},
		{
			statement: `set(resource.schema_url, "test_schema_url")`,
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).SetSchemaUrl("test_schema_url")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructMetrics()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessMetrics(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructMetrics()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessMetrics_ScopeContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td pmetric.Metrics)
	}{
		{
			statement: `set(attributes["test"], "pass") where name == "scope"`,
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where version == 2`,
			want: func(_ pmetric.Metrics) {
			},
		},
		{
			statement: `set(schema_url, "test_schema_url")`,
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).SetSchemaUrl("test_schema_url")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructMetrics()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "scope", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessMetrics(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructMetrics()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessMetrics_InferredScopeContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td pmetric.Metrics)
	}{
		{
			statement: `set(scope.attributes["test"], "pass") where scope.name == "scope"`,
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(scope.attributes["test"], "pass") where scope.version == 2`,
			want: func(_ pmetric.Metrics) {
			},
		},
		{
			statement: `set(scope.schema_url, "test_schema_url")`,
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).SetSchemaUrl("test_schema_url")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructMetrics()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessMetrics(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructMetrics()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessMetrics_MetricContext(t *testing.T) {
	tests := []struct {
		statements []string
		want       func(pmetric.Metrics)
	}{
		{
			statements: []string{`extract_sum_metric(true) where name == "operationB"`},
			want: func(td pmetric.Metrics) {
				sumMetric := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				sumDp := sumMetric.SetEmptySum().DataPoints().AppendEmpty()

				histogramMetric := pmetric.NewMetric()
				fillMetricTwo(histogramMetric)
				histogramDp := histogramMetric.Histogram().DataPoints().At(0)

				sumMetric.SetDescription(histogramMetric.Description())
				sumMetric.SetName(histogramMetric.Name() + "_sum")
				sumMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				sumMetric.Sum().SetIsMonotonic(true)
				sumMetric.SetUnit(histogramMetric.Unit())

				histogramDp.Attributes().CopyTo(sumDp.Attributes())
				sumDp.SetDoubleValue(histogramDp.Sum())
				sumDp.SetStartTimestamp(StartTimestamp)

				// we have two histogram datapoints, but only one of them has the Sum set
				// so we should only have one Sum datapoint
			},
		},
		{ // this checks if subsequent statements apply to the newly created metric
			statements: []string{
				`extract_sum_metric(true) where name == "operationB"`,
				`set(name, "new_name") where name == "operationB_sum"`,
			},
			want: func(td pmetric.Metrics) {
				sumMetric := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				sumDp := sumMetric.SetEmptySum().DataPoints().AppendEmpty()

				histogramMetric := pmetric.NewMetric()
				fillMetricTwo(histogramMetric)
				histogramDp := histogramMetric.Histogram().DataPoints().At(0)

				sumMetric.SetDescription(histogramMetric.Description())
				sumMetric.SetName("new_name")
				sumMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				sumMetric.Sum().SetIsMonotonic(true)
				sumMetric.SetUnit(histogramMetric.Unit())

				histogramDp.Attributes().CopyTo(sumDp.Attributes())
				sumDp.SetDoubleValue(histogramDp.Sum())
				sumDp.SetStartTimestamp(StartTimestamp)

				// we have two histogram datapoints, but only one of them has the Sum set
				// so we should only have one Sum datapoint
			},
		},
		{
			statements: []string{`extract_count_metric(true) where name == "operationB"`},
			want: func(td pmetric.Metrics) {
				countMetric := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				countMetric.SetEmptySum()

				histogramMetric := pmetric.NewMetric()
				fillMetricTwo(histogramMetric)

				countMetric.SetDescription(histogramMetric.Description())
				countMetric.SetName(histogramMetric.Name() + "_count")
				countMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				countMetric.Sum().SetIsMonotonic(true)
				countMetric.SetUnit("1")

				histogramDp0 := histogramMetric.Histogram().DataPoints().At(0)
				countDp0 := countMetric.Sum().DataPoints().AppendEmpty()
				histogramDp0.Attributes().CopyTo(countDp0.Attributes())
				countDp0.SetIntValue(int64(histogramDp0.Count()))
				countDp0.SetStartTimestamp(StartTimestamp)

				// we have two histogram datapoints
				histogramDp1 := histogramMetric.Histogram().DataPoints().At(1)
				countDp1 := countMetric.Sum().DataPoints().AppendEmpty()
				histogramDp1.Attributes().CopyTo(countDp1.Attributes())
				countDp1.SetIntValue(int64(histogramDp1.Count()))
				countDp1.SetStartTimestamp(StartTimestamp)
			},
		},
		{
			statements: []string{`copy_metric(name="http.request.status_code", unit="s") where name == "operationA"`},
			want: func(td pmetric.Metrics) {
				newMetric := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).CopyTo(newMetric)
				newMetric.SetName("http.request.status_code")
				newMetric.SetUnit("s")
			},
		},
		{
			statements: []string{`scale_metric(10.0,"s") where name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).SetDoubleValue(10.0)
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).SetDoubleValue(37.0)
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetUnit("s")
			},
		},
		{
			statements: []string{`scale_metric(10.0) where name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).SetDoubleValue(10.0)
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).SetDoubleValue(37.0)
			},
		},
		{
			statements: []string{`aggregate_on_attributes("sum", ["attr1", "attr2"]) where name == "operationA"`},
			want: func(td pmetric.Metrics) {
				m := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)

				dataPoints := pmetric.NewNumberDataPointSlice()
				dataPoint1 := dataPoints.AppendEmpty()
				dataPoint1.SetStartTimestamp(StartTimestamp)
				dataPoint1.SetDoubleValue(4.7)
				dataPoint1.Attributes().PutStr("attr1", "test1")
				dataPoint1.Attributes().PutStr("attr2", "test2")

				dataPoints.CopyTo(m.Sum().DataPoints())
			},
		},
		{
			statements: []string{`aggregate_on_attributes("min") where name == "operationA"`},
			want: func(td pmetric.Metrics) {
				m := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)

				dataPoints := pmetric.NewNumberDataPointSlice()
				dataPoint1 := dataPoints.AppendEmpty()
				dataPoint1.SetStartTimestamp(StartTimestamp)
				dataPoint1.SetDoubleValue(1.0)
				dataPoint1.Attributes().PutStr("attr1", "test1")
				dataPoint1.Attributes().PutStr("attr2", "test2")
				dataPoint1.Attributes().PutStr("attr3", "test3")
				dataPoint1.Attributes().PutStr("flags", "A|B|C")
				dataPoint1.Attributes().PutStr("total.string", "123456789")

				dataPoints.CopyTo(m.Sum().DataPoints())
			},
		},
		{
			statements: []string{`aggregate_on_attribute_value("sum", "attr1", ["test1", "test2"], "test") where name == "operationE"`},
			want: func(td pmetric.Metrics) {
				m := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4)

				dataPoints := pmetric.NewNumberDataPointSlice()
				dataPoint1 := dataPoints.AppendEmpty()
				dataPoint1.SetStartTimestamp(StartTimestamp)
				dataPoint1.SetDoubleValue(4.7)
				dataPoint1.Attributes().PutStr("attr1", "test")

				dataPoints.CopyTo(m.Sum().DataPoints())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statements[0], func(t *testing.T) {
			td := constructMetrics()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "metric", Statements: tt.statements}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessMetrics(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructMetrics()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessMetrics_InferredMetricContext(t *testing.T) {
	tests := []struct {
		statements []string
		want       func(pmetric.Metrics)
	}{
		{
			statements: []string{`extract_sum_metric(true) where metric.name == "operationB"`},
			want: func(td pmetric.Metrics) {
				sumMetric := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				sumDp := sumMetric.SetEmptySum().DataPoints().AppendEmpty()

				histogramMetric := pmetric.NewMetric()
				fillMetricTwo(histogramMetric)
				histogramDp := histogramMetric.Histogram().DataPoints().At(0)

				sumMetric.SetDescription(histogramMetric.Description())
				sumMetric.SetName(histogramMetric.Name() + "_sum")
				sumMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				sumMetric.Sum().SetIsMonotonic(true)
				sumMetric.SetUnit(histogramMetric.Unit())

				histogramDp.Attributes().CopyTo(sumDp.Attributes())
				sumDp.SetDoubleValue(histogramDp.Sum())
				sumDp.SetStartTimestamp(StartTimestamp)

				// we have two histogram datapoints, but only one of them has the Sum set
				// so we should only have one Sum datapoint
			},
		},
		{ // this checks if subsequent statements apply to the newly created metric
			statements: []string{
				`extract_sum_metric(true) where metric.name == "operationB"`,
				`set(metric.name, "new_name") where metric.name == "operationB_sum"`,
			},
			want: func(td pmetric.Metrics) {
				sumMetric := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				sumDp := sumMetric.SetEmptySum().DataPoints().AppendEmpty()

				histogramMetric := pmetric.NewMetric()
				fillMetricTwo(histogramMetric)
				histogramDp := histogramMetric.Histogram().DataPoints().At(0)

				sumMetric.SetDescription(histogramMetric.Description())
				sumMetric.SetName("new_name")
				sumMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				sumMetric.Sum().SetIsMonotonic(true)
				sumMetric.SetUnit(histogramMetric.Unit())

				histogramDp.Attributes().CopyTo(sumDp.Attributes())
				sumDp.SetDoubleValue(histogramDp.Sum())
				sumDp.SetStartTimestamp(StartTimestamp)

				// we have two histogram datapoints, but only one of them has the Sum set
				// so we should only have one Sum datapoint
			},
		},
		{
			statements: []string{`extract_count_metric(true) where metric.name == "operationB"`},
			want: func(td pmetric.Metrics) {
				countMetric := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				countMetric.SetEmptySum()

				histogramMetric := pmetric.NewMetric()
				fillMetricTwo(histogramMetric)

				countMetric.SetDescription(histogramMetric.Description())
				countMetric.SetName(histogramMetric.Name() + "_count")
				countMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				countMetric.Sum().SetIsMonotonic(true)
				countMetric.SetUnit("1")

				histogramDp0 := histogramMetric.Histogram().DataPoints().At(0)
				countDp0 := countMetric.Sum().DataPoints().AppendEmpty()
				histogramDp0.Attributes().CopyTo(countDp0.Attributes())
				countDp0.SetIntValue(int64(histogramDp0.Count()))
				countDp0.SetStartTimestamp(StartTimestamp)

				// we have two histogram datapoints
				histogramDp1 := histogramMetric.Histogram().DataPoints().At(1)
				countDp1 := countMetric.Sum().DataPoints().AppendEmpty()
				histogramDp1.Attributes().CopyTo(countDp1.Attributes())
				countDp1.SetIntValue(int64(histogramDp1.Count()))
				countDp1.SetStartTimestamp(StartTimestamp)
			},
		},
		{
			statements: []string{`copy_metric(name="http.request.status_code", unit="s") where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				newMetric := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).CopyTo(newMetric)
				newMetric.SetName("http.request.status_code")
				newMetric.SetUnit("s")
			},
		},
		{
			statements: []string{`scale_metric(10.0,"s") where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).SetDoubleValue(10.0)
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).SetDoubleValue(37.0)
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetUnit("s")
			},
		},
		{
			statements: []string{`scale_metric(10.0) where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).SetDoubleValue(10.0)
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).SetDoubleValue(37.0)
			},
		},
		{
			statements: []string{`aggregate_on_attributes("sum", ["attr1", "attr2"]) where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				m := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)

				dataPoints := pmetric.NewNumberDataPointSlice()
				dataPoint1 := dataPoints.AppendEmpty()
				dataPoint1.SetStartTimestamp(StartTimestamp)
				dataPoint1.SetDoubleValue(4.7)
				dataPoint1.Attributes().PutStr("attr1", "test1")
				dataPoint1.Attributes().PutStr("attr2", "test2")

				dataPoints.CopyTo(m.Sum().DataPoints())
			},
		},
		{
			statements: []string{`aggregate_on_attributes("min") where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				m := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)

				dataPoints := pmetric.NewNumberDataPointSlice()
				dataPoint1 := dataPoints.AppendEmpty()
				dataPoint1.SetStartTimestamp(StartTimestamp)
				dataPoint1.SetDoubleValue(1.0)
				dataPoint1.Attributes().PutStr("attr1", "test1")
				dataPoint1.Attributes().PutStr("attr2", "test2")
				dataPoint1.Attributes().PutStr("attr3", "test3")
				dataPoint1.Attributes().PutStr("flags", "A|B|C")
				dataPoint1.Attributes().PutStr("total.string", "123456789")

				dataPoints.CopyTo(m.Sum().DataPoints())
			},
		},
		{
			statements: []string{`aggregate_on_attribute_value("sum", "attr1", ["test1", "test2"], "test") where metric.name == "operationE"`},
			want: func(td pmetric.Metrics) {
				m := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4)

				dataPoints := pmetric.NewNumberDataPointSlice()
				dataPoint1 := dataPoints.AppendEmpty()
				dataPoint1.SetStartTimestamp(StartTimestamp)
				dataPoint1.SetDoubleValue(4.7)
				dataPoint1.Attributes().PutStr("attr1", "test")

				dataPoints.CopyTo(m.Sum().DataPoints())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statements[0], func(t *testing.T) {
			var contextStatements []common.ContextStatements
			for _, statement := range tt.statements {
				contextStatements = append(contextStatements, common.ContextStatements{Context: "", Statements: []string{statement}})
			}

			td := constructMetrics()
			processor, err := NewProcessor(contextStatements, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessMetrics(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructMetrics()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessMetrics_DataPointContext(t *testing.T) {
	tests := []struct {
		statements []string
		want       func(pmetric.Metrics)
	}{
		{
			statements: []string{`set(attributes["test"], "pass") where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			statements: []string{`set(attributes["test"], "pass") where resource.attributes["host.name"] == "myhost"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(1).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(1).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Sum().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Sum().DataPoints().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			statements: []string{`set(attributes["int_value"], Int("2")) where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutInt("int_value", 2)
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutInt("int_value", 2)
			},
		},
		{
			statements: []string{`set(attributes["int_value"], Int(value_double)) where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutInt("int_value", 1)
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutInt("int_value", 3)
			},
		},
		{
			statements: []string{`keep_keys(attributes, ["attr2"]) where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("attr2", "test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("attr2", "test2")
			},
		},
		{
			statements: []string{`set(metric.description, "test") where attributes["attr1"] == "test1"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetDescription("test")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).SetDescription("test")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).SetDescription("test")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).SetDescription("test")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).SetDescription("test")
			},
		},
		{
			statements: []string{`set(metric.unit, "new unit")`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetUnit("new unit")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).SetUnit("new unit")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).SetUnit("new unit")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).SetUnit("new unit")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).SetUnit("new unit")
			},
		},
		{
			statements: []string{`set(metric.description, "Sum") where metric.type == METRIC_DATA_TYPE_SUM`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetDescription("Sum")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).SetDescription("Sum")
			},
		},
		{
			statements: []string{`set(metric.aggregation_temporality, AGGREGATION_TEMPORALITY_DELTA) where metric.aggregation_temporality == 0`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
			},
		},
		{
			statements: []string{`set(metric.is_monotonic, true) where metric.is_monotonic == false`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().SetIsMonotonic(true)
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Sum().SetIsMonotonic(true)
			},
		},
		{
			statements: []string{`set(attributes["test"], "pass") where count == 1`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statements: []string{`set(attributes["test"], "pass") where scale == 1`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statements: []string{`set(attributes["test"], "pass") where zero_count == 1`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statements: []string{`set(attributes["test"], "pass") where positive.offset == 1`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statements: []string{`set(attributes["test"], "pass") where negative.offset == 1`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statements: []string{`replace_pattern(attributes["attr1"], "test1", "pass")`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(1).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().At(0).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Sum().DataPoints().At(0).Attributes().PutStr("attr1", "pass")
			},
		},
		{
			statements: []string{`replace_all_patterns(attributes, "value", "test1", "pass")`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(1).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().At(0).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Sum().DataPoints().At(0).Attributes().PutStr("attr1", "pass")
			},
		},
		{
			statements: []string{`replace_all_patterns(attributes, "key", "attr3", "attr4")`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("attr2", "test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("attr4", "test3")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("flags", "A|B|C")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("total.string", "123456789")

				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("attr2", "test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("attr4", "test3")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("flags", "A|B|C")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("total.string", "123456789")

				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("attr2", "test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("attr4", "test3")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("flags", "C|D")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("total.string", "345678")

				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutStr("attr2", "test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutStr("attr4", "test3")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutStr("flags", "C|D")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutStr("total.string", "345678")

				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("attr2", "test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("attr4", "test3")

				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(1).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(1).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(1).Attributes().PutStr("attr2", "test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(1).Attributes().PutStr("attr4", "test3")

				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().At(0).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().At(0).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().At(0).Attributes().PutStr("attr2", "test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().At(0).Attributes().PutStr("attr4", "test3")
			},
		},
		{
			statements: []string{`convert_summary_count_val_to_sum("delta", true) where metric.name == "operationD"`},
			want: func(td pmetric.Metrics) {
				sumMetric := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				sumDp := sumMetric.SetEmptySum().DataPoints().AppendEmpty()

				summaryMetric := pmetric.NewMetric()
				fillMetricFour(summaryMetric)
				summaryDp := summaryMetric.Summary().DataPoints().At(0)

				sumMetric.SetDescription(summaryMetric.Description())
				sumMetric.SetName(summaryMetric.Name() + "_count")
				sumMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				sumMetric.Sum().SetIsMonotonic(true)
				sumMetric.SetUnit(summaryMetric.Unit())

				summaryDp.Attributes().CopyTo(sumDp.Attributes())
				sumDp.SetIntValue(int64(summaryDp.Count()))
				sumDp.SetStartTimestamp(StartTimestamp)
				sumDp.SetTimestamp(TestTimeStamp)
			},
		},
		{
			statements: []string{`convert_summary_sum_val_to_sum("delta", true) where metric.name == "operationD"`},
			want: func(td pmetric.Metrics) {
				sumMetric := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				sumDp := sumMetric.SetEmptySum().DataPoints().AppendEmpty()

				summaryMetric := pmetric.NewMetric()
				fillMetricFour(summaryMetric)
				summaryDp := summaryMetric.Summary().DataPoints().At(0)

				sumMetric.SetDescription(summaryMetric.Description())
				sumMetric.SetName(summaryMetric.Name() + "_sum")
				sumMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				sumMetric.Sum().SetIsMonotonic(true)
				sumMetric.SetUnit(summaryMetric.Unit())

				summaryDp.Attributes().CopyTo(sumDp.Attributes())
				sumDp.SetDoubleValue(summaryDp.Sum())
				sumDp.SetStartTimestamp(StartTimestamp)
				sumDp.SetTimestamp(TestTimeStamp)
			},
		},
		{
			statements: []string{
				`convert_summary_sum_val_to_sum("delta", true) where metric.name == "operationD"`,
				`set(metric.unit, "new unit")`,
			},
			want: func(td pmetric.Metrics) {
				sumMetric := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				sumDp := sumMetric.SetEmptySum().DataPoints().AppendEmpty()

				summaryMetric := pmetric.NewMetric()
				fillMetricFour(summaryMetric)
				summaryDp := summaryMetric.Summary().DataPoints().At(0)

				sumMetric.SetDescription(summaryMetric.Description())
				sumMetric.SetName(summaryMetric.Name() + "_sum")
				sumMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				sumMetric.Sum().SetIsMonotonic(true)
				sumMetric.SetUnit("new unit")

				summaryDp.Attributes().CopyTo(sumDp.Attributes())
				sumDp.SetDoubleValue(summaryDp.Sum())
				sumDp.SetStartTimestamp(StartTimestamp)
				sumDp.SetTimestamp(TestTimeStamp)

				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetUnit("new unit")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).SetUnit("new unit")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).SetUnit("new unit")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).SetUnit("new unit")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).SetUnit("new unit")
			},
		},
		{
			statements: []string{`set(attributes["test"], "pass") where IsMatch(metric.name, "operation[AC]")`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			statements: []string{`delete_key(attributes, "attr3") where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("attr2", "test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("total.string", "123456789")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("flags", "A|B|C")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("attr2", "test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("total.string", "123456789")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("flags", "A|B|C")
			},
		},
		{
			statements: []string{`delete_matching_keys(attributes, "[23]") where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("flags", "A|B|C")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("total.string", "123456789")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("flags", "A|B|C")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("total.string", "123456789")
			},
		},
		{
			statements: []string{`set(attributes["test"], Concat([attributes["attr1"], attributes["attr2"]], "-")) where metric.name == Concat(["operation", "A"], "")`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test", "test1-test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test", "test1-test2")
			},
		},
		{
			statements: []string{`set(attributes["test"], Split(attributes["flags"], "|"))`},
			want: func(td pmetric.Metrics) {
				v00 := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutEmptySlice("test")
				v00.AppendEmpty().SetStr("A")
				v00.AppendEmpty().SetStr("B")
				v00.AppendEmpty().SetStr("C")
				v01 := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutEmptySlice("test")
				v01.AppendEmpty().SetStr("A")
				v01.AppendEmpty().SetStr("B")
				v01.AppendEmpty().SetStr("C")
				v10 := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutEmptySlice("test")
				v10.AppendEmpty().SetStr("C")
				v10.AppendEmpty().SetStr("D")
				v11 := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutEmptySlice("test")
				v11.AppendEmpty().SetStr("C")
				v11.AppendEmpty().SetStr("D")
			},
		},
		{
			statements: []string{`set(attributes["test"], Split(attributes["flags"], "|")) where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				v00 := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutEmptySlice("test")
				v00.AppendEmpty().SetStr("A")
				v00.AppendEmpty().SetStr("B")
				v00.AppendEmpty().SetStr("C")
				v01 := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutEmptySlice("test")
				v01.AppendEmpty().SetStr("A")
				v01.AppendEmpty().SetStr("B")
				v01.AppendEmpty().SetStr("C")
			},
		},
		{
			statements: []string{`set(attributes["test"], Split(attributes["not_exist"], "|"))`},
			want:       func(_ pmetric.Metrics) {},
		},
		{
			statements: []string{`set(attributes["test"], Substring(attributes["total.string"], 3, 3))`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test", "456")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test", "456")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("test", "678")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutStr("test", "678")
			},
		},
		{
			statements: []string{`set(attributes["test"], Substring(attributes["total.string"], 3, 3)) where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test", "456")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test", "456")
			},
		},
		{
			statements: []string{`set(attributes["test"], Substring(attributes["not_exist"], 3, 3))`},
			want:       func(_ pmetric.Metrics) {},
		},
		{
			statements: []string{
				`set(attributes["test_lower"], ConvertCase(metric.name, "lower")) where metric.name == "operationA"`,
				`set(attributes["test_upper"], ConvertCase(metric.name, "upper")) where metric.name == "operationA"`,
				`set(attributes["test_snake"], ConvertCase(metric.name, "snake")) where metric.name == "operationA"`,
				`set(attributes["test_camel"], ConvertCase(metric.name, "camel")) where metric.name == "operationA"`,
			},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test_lower", "operationa")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test_lower", "operationa")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test_upper", "OPERATIONA")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test_upper", "OPERATIONA")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test_snake", "operation_a")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test_snake", "operation_a")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test_camel", "OperationA")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test_camel", "OperationA")
			},
		},
		{
			statements: []string{`set(attributes["test"], ["A", "B", "C"]) where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				v00 := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutEmptySlice("test")
				v00.AppendEmpty().SetStr("A")
				v00.AppendEmpty().SetStr("B")
				v00.AppendEmpty().SetStr("C")
				v01 := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutEmptySlice("test")
				v01.AppendEmpty().SetStr("A")
				v01.AppendEmpty().SetStr("B")
				v01.AppendEmpty().SetStr("C")
			},
		},
		{
			statements: []string{`merge_maps(attributes, ParseJSON("{\"json_test\":\"pass\"}"), "insert") where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("json_test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("json_test", "pass")
			},
		},
		{
			statements: []string{`limit(attributes, 0, []) where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().RemoveIf(func(_ string, _ pcommon.Value) bool { return true })
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().RemoveIf(func(_ string, _ pcommon.Value) bool { return true })
			},
		},
		{
			statements: []string{`set(attributes["test"], Log(1)) where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutDouble("test", 0.0)
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutDouble("test", 0.0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statements[0], func(t *testing.T) {
			td := constructMetrics()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "datapoint", Statements: tt.statements}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessMetrics(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructMetrics()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessMetrics_InferredDataPointContext(t *testing.T) {
	tests := []struct {
		statements []string
		want       func(pmetric.Metrics)
	}{
		{
			statements: []string{`set(datapoint.attributes["test"], "pass") where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			statements: []string{`set(datapoint.attributes["test"], "pass") where resource.attributes["host.name"] == "myhost"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(1).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(1).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Sum().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Sum().DataPoints().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			statements: []string{`set(datapoint.attributes["int_value"], Int("2")) where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutInt("int_value", 2)
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutInt("int_value", 2)
			},
		},
		{
			statements: []string{`set(datapoint.attributes["int_value"], Int(datapoint.value_double)) where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutInt("int_value", 1)
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutInt("int_value", 3)
			},
		},
		{
			statements: []string{`keep_keys(datapoint.attributes, ["attr2"]) where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("attr2", "test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("attr2", "test2")
			},
		},
		{
			statements: []string{`set(metric.description, "test") where datapoint.attributes["attr1"] == "test1"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetDescription("test")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).SetDescription("test")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).SetDescription("test")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).SetDescription("test")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).SetDescription("test")
			},
		},
		{
			statements: []string{`set(metric.unit, "new unit")`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetUnit("new unit")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).SetUnit("new unit")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).SetUnit("new unit")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).SetUnit("new unit")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).SetUnit("new unit")
			},
		},
		{
			statements: []string{`set(metric.description, "Sum") where metric.type == METRIC_DATA_TYPE_SUM`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetDescription("Sum")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).SetDescription("Sum")
			},
		},
		{
			statements: []string{`set(metric.aggregation_temporality, AGGREGATION_TEMPORALITY_DELTA) where metric.aggregation_temporality == 0`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
			},
		},
		{
			statements: []string{`set(metric.is_monotonic, true) where metric.is_monotonic == false`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().SetIsMonotonic(true)
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Sum().SetIsMonotonic(true)
			},
		},
		{
			statements: []string{`set(datapoint.attributes["test"], "pass") where datapoint.count == 1`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statements: []string{`set(datapoint.attributes["test"], "pass") where datapoint.scale == 1`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statements: []string{`set(datapoint.attributes["test"], "pass") where datapoint.zero_count == 1`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statements: []string{`set(datapoint.attributes["test"], "pass") where datapoint.positive.offset == 1`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statements: []string{`set(datapoint.attributes["test"], "pass") where datapoint.negative.offset == 1`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statements: []string{`replace_pattern(datapoint.attributes["attr1"], "test1", "pass")`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(1).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().At(0).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Sum().DataPoints().At(0).Attributes().PutStr("attr1", "pass")
			},
		},
		{
			statements: []string{`replace_all_patterns(datapoint.attributes, "value", "test1", "pass")`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(1).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().At(0).Attributes().PutStr("attr1", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Sum().DataPoints().At(0).Attributes().PutStr("attr1", "pass")
			},
		},
		{
			statements: []string{`replace_all_patterns(datapoint.attributes, "key", "attr3", "attr4")`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("attr2", "test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("attr4", "test3")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("flags", "A|B|C")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("total.string", "123456789")

				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("attr2", "test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("attr4", "test3")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("flags", "A|B|C")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("total.string", "123456789")

				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("attr2", "test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("attr4", "test3")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("flags", "C|D")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("total.string", "345678")

				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutStr("attr2", "test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutStr("attr4", "test3")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutStr("flags", "C|D")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutStr("total.string", "345678")

				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("attr2", "test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("attr4", "test3")

				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(1).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(1).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(1).Attributes().PutStr("attr2", "test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(1).Attributes().PutStr("attr4", "test3")

				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().At(0).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().At(0).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().At(0).Attributes().PutStr("attr2", "test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().At(0).Attributes().PutStr("attr4", "test3")
			},
		},
		{
			statements: []string{`convert_summary_count_val_to_sum("delta", true) where metric.name == "operationD"`},
			want: func(td pmetric.Metrics) {
				sumMetric := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				sumDp := sumMetric.SetEmptySum().DataPoints().AppendEmpty()

				summaryMetric := pmetric.NewMetric()
				fillMetricFour(summaryMetric)
				summaryDp := summaryMetric.Summary().DataPoints().At(0)

				sumMetric.SetDescription(summaryMetric.Description())
				sumMetric.SetName(summaryMetric.Name() + "_count")
				sumMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				sumMetric.Sum().SetIsMonotonic(true)
				sumMetric.SetUnit(summaryMetric.Unit())

				summaryDp.Attributes().CopyTo(sumDp.Attributes())
				sumDp.SetIntValue(int64(summaryDp.Count()))
				sumDp.SetStartTimestamp(StartTimestamp)
				sumDp.SetTimestamp(TestTimeStamp)
			},
		},
		{
			statements: []string{`convert_summary_sum_val_to_sum("delta", true) where metric.name == "operationD"`},
			want: func(td pmetric.Metrics) {
				sumMetric := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				sumDp := sumMetric.SetEmptySum().DataPoints().AppendEmpty()

				summaryMetric := pmetric.NewMetric()
				fillMetricFour(summaryMetric)
				summaryDp := summaryMetric.Summary().DataPoints().At(0)

				sumMetric.SetDescription(summaryMetric.Description())
				sumMetric.SetName(summaryMetric.Name() + "_sum")
				sumMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				sumMetric.Sum().SetIsMonotonic(true)
				sumMetric.SetUnit(summaryMetric.Unit())

				summaryDp.Attributes().CopyTo(sumDp.Attributes())
				sumDp.SetDoubleValue(summaryDp.Sum())
				sumDp.SetStartTimestamp(StartTimestamp)
				sumDp.SetTimestamp(TestTimeStamp)
			},
		},
		{
			statements: []string{
				`convert_summary_sum_val_to_sum("delta", true) where metric.name == "operationD"`,
				`set(metric.unit, "new unit")`,
			},
			want: func(td pmetric.Metrics) {
				sumMetric := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				sumDp := sumMetric.SetEmptySum().DataPoints().AppendEmpty()

				summaryMetric := pmetric.NewMetric()
				fillMetricFour(summaryMetric)
				summaryDp := summaryMetric.Summary().DataPoints().At(0)

				sumMetric.SetDescription(summaryMetric.Description())
				sumMetric.SetName(summaryMetric.Name() + "_sum")
				sumMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				sumMetric.Sum().SetIsMonotonic(true)
				sumMetric.SetUnit("new unit")

				summaryDp.Attributes().CopyTo(sumDp.Attributes())
				sumDp.SetDoubleValue(summaryDp.Sum())
				sumDp.SetStartTimestamp(StartTimestamp)
				sumDp.SetTimestamp(TestTimeStamp)

				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetUnit("new unit")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).SetUnit("new unit")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).SetUnit("new unit")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).SetUnit("new unit")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).SetUnit("new unit")
			},
		},
		{
			statements: []string{`set(datapoint.attributes["test"], "pass") where IsMatch(metric.name, "operation[AC]")`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			statements: []string{`delete_key(datapoint.attributes, "attr3") where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("attr2", "test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("total.string", "123456789")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("flags", "A|B|C")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("attr2", "test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("total.string", "123456789")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("flags", "A|B|C")
			},
		},
		{
			statements: []string{`delete_matching_keys(datapoint.attributes, "[23]") where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("flags", "A|B|C")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("total.string", "123456789")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().Clear()
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("attr1", "test1")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("flags", "A|B|C")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("total.string", "123456789")
			},
		},
		{
			statements: []string{`set(datapoint.attributes["test"], Concat([datapoint.attributes["attr1"], datapoint.attributes["attr2"]], "-")) where metric.name == Concat(["operation", "A"], "")`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test", "test1-test2")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test", "test1-test2")
			},
		},
		{
			statements: []string{`set(datapoint.attributes["test"], Split(datapoint.attributes["flags"], "|"))`},
			want: func(td pmetric.Metrics) {
				v00 := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutEmptySlice("test")
				v00.AppendEmpty().SetStr("A")
				v00.AppendEmpty().SetStr("B")
				v00.AppendEmpty().SetStr("C")
				v01 := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutEmptySlice("test")
				v01.AppendEmpty().SetStr("A")
				v01.AppendEmpty().SetStr("B")
				v01.AppendEmpty().SetStr("C")
				v10 := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutEmptySlice("test")
				v10.AppendEmpty().SetStr("C")
				v10.AppendEmpty().SetStr("D")
				v11 := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutEmptySlice("test")
				v11.AppendEmpty().SetStr("C")
				v11.AppendEmpty().SetStr("D")
			},
		},
		{
			statements: []string{`set(datapoint.attributes["test"], Split(datapoint.attributes["flags"], "|")) where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				v00 := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutEmptySlice("test")
				v00.AppendEmpty().SetStr("A")
				v00.AppendEmpty().SetStr("B")
				v00.AppendEmpty().SetStr("C")
				v01 := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutEmptySlice("test")
				v01.AppendEmpty().SetStr("A")
				v01.AppendEmpty().SetStr("B")
				v01.AppendEmpty().SetStr("C")
			},
		},
		{
			statements: []string{`set(datapoint.attributes["test"], Split(datapoint.attributes["not_exist"], "|"))`},
			want:       func(_ pmetric.Metrics) {},
		},
		{
			statements: []string{`set(datapoint.attributes["test"], Substring(datapoint.attributes["total.string"], 3, 3))`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test", "456")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test", "456")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("test", "678")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutStr("test", "678")
			},
		},
		{
			statements: []string{`set(datapoint.attributes["test"], Substring(datapoint.attributes["total.string"], 3, 3)) where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test", "456")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test", "456")
			},
		},
		{
			statements: []string{`set(datapoint.attributes["test"], Substring(datapoint.attributes["not_exist"], 3, 3))`},
			want:       func(_ pmetric.Metrics) {},
		},
		{
			statements: []string{
				`set(datapoint.attributes["test_lower"], ConvertCase(metric.name, "lower")) where metric.name == "operationA"`,
				`set(datapoint.attributes["test_upper"], ConvertCase(metric.name, "upper")) where metric.name == "operationA"`,
				`set(datapoint.attributes["test_snake"], ConvertCase(metric.name, "snake")) where metric.name == "operationA"`,
				`set(datapoint.attributes["test_camel"], ConvertCase(metric.name, "camel")) where metric.name == "operationA"`,
			},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test_lower", "operationa")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test_lower", "operationa")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test_upper", "OPERATIONA")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test_upper", "OPERATIONA")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test_snake", "operation_a")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test_snake", "operation_a")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test_camel", "OperationA")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test_camel", "OperationA")
			},
		},
		{
			statements: []string{`set(datapoint.attributes["test"], ["A", "B", "C"]) where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				v00 := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutEmptySlice("test")
				v00.AppendEmpty().SetStr("A")
				v00.AppendEmpty().SetStr("B")
				v00.AppendEmpty().SetStr("C")
				v01 := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutEmptySlice("test")
				v01.AppendEmpty().SetStr("A")
				v01.AppendEmpty().SetStr("B")
				v01.AppendEmpty().SetStr("C")
			},
		},
		{
			statements: []string{`merge_maps(datapoint.attributes, ParseJSON("{\"json_test\":\"pass\"}"), "insert") where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("json_test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("json_test", "pass")
			},
		},
		{
			statements: []string{`limit(datapoint.attributes, 0, []) where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().RemoveIf(func(_ string, _ pcommon.Value) bool { return true })
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().RemoveIf(func(_ string, _ pcommon.Value) bool { return true })
			},
		},
		{
			statements: []string{`set(datapoint.attributes["test"], Log(1)) where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutDouble("test", 0.0)
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutDouble("test", 0.0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statements[0], func(t *testing.T) {
			td := constructMetrics()
			var contextStatements []common.ContextStatements
			for _, statement := range tt.statements {
				contextStatements = append(contextStatements, common.ContextStatements{Context: "", Statements: []string{statement}})
			}

			processor, err := NewProcessor(contextStatements, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessMetrics(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructMetrics()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessMetrics_MixContext(t *testing.T) {
	tests := []struct {
		name              string
		contextStatements []common.ContextStatements
		want              func(td pmetric.Metrics)
	}{
		{
			name: "set resource and then use",
			contextStatements: []common.ContextStatements{
				{
					Context: "resource",
					Statements: []string{
						`set(attributes["test"], "pass")`,
					},
				},
				{
					Context: "datapoint",
					Statements: []string{
						`set(attributes["test"], "pass") where resource.attributes["test"] == "pass"`,
					},
				},
			},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).Resource().Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(1).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Sum().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Sum().DataPoints().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "set scope and then use",
			contextStatements: []common.ContextStatements{
				{
					Context: "scope",
					Statements: []string{
						`set(attributes["test"], "pass")`,
					},
				},
				{
					Context: "datapoint",
					Statements: []string{
						`set(attributes["test"], "pass") where instrumentation_scope.attributes["test"] == "pass"`,
					},
				},
			},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(1).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Sum().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Sum().DataPoints().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "order matters",
			contextStatements: []common.ContextStatements{
				{
					Context: "datapoint",
					Statements: []string{
						`set(attributes["test"], "pass") where instrumentation_scope.attributes["test"] == "pass"`,
					},
				},
				{
					Context: "scope",
					Statements: []string{
						`set(attributes["test"], "pass")`,
					},
				},
			},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "reuse context ",
			contextStatements: []common.ContextStatements{
				{
					Context: "scope",
					Statements: []string{
						`set(attributes["test"], "pass")`,
					},
				},
				{
					Context: "datapoint",
					Statements: []string{
						`set(attributes["test"], "pass") where instrumentation_scope.attributes["test"] == "pass"`,
					},
				},
				{
					Context: "scope",
					Statements: []string{
						`set(attributes["test"], "fail")`,
					},
				},
			},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes().PutStr("test", "fail")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Histogram().DataPoints().At(1).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).ExponentialHistogram().DataPoints().At(1).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Sum().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Sum().DataPoints().At(1).Attributes().PutStr("test", "pass")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := constructMetrics()
			processor, err := NewProcessor(tt.contextStatements, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessMetrics(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructMetrics()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessMetrics_ErrorMode(t *testing.T) {
	tests := []struct {
		statement string
		context   common.ContextID
	}{
		{
			statement: `set(attributes["test"], ParseJSON(1))`,
			context:   "resource",
		},
		{
			statement: `set(attributes["test"], ParseJSON(1))`,
			context:   "scope",
		},
		{
			statement: `set(name, ParseJSON(1))`,
			context:   "metric",
		},
		{
			statement: `set(attributes["test"], ParseJSON(1))`,
			context:   "datapoint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructMetrics()
			processor, err := NewProcessor([]common.ContextStatements{{Context: tt.context, Statements: []string{tt.statement}}}, ottl.PropagateError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessMetrics(context.Background(), td)
			assert.Error(t, err)
		})
	}
}

func Test_ProcessMetrics_StatementsErrorMode(t *testing.T) {
	tests := []struct {
		name          string
		errorMode     ottl.ErrorMode
		statements    []common.ContextStatements
		want          func(td pmetric.Metrics)
		wantErrorWith string
	}{
		{
			name:      "metric: statements group with error mode",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(metric.name, ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(metric.name, "pass") where metric.name == "operationA" `}},
			},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetName("pass")
			},
		},
		{
			name:      "metric: statements group error mode does not affect default",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(metric.name, ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(metric.name, ParseJSON(true))`}},
			},
			wantErrorWith: "expected string but got bool",
		},
		{
			name:      "datapoint: statements group with error mode",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(datapoint.attributes["test"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(datapoint.attributes["test"], "pass") where metric.name == "operationA" `}},
			},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			name:      "datapoint: statements group error mode does not affect default",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(datapoint.attributes["test"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(datapoint.attributes["test"], ParseJSON(true))`}},
			},
			wantErrorWith: "expected string but got bool",
		},
		{
			name:      "resource: statements group with error mode",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(resource.attributes["pass"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(resource.attributes["test"], "pass")`}},
			},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).Resource().Attributes().PutStr("test", "pass")
			},
		},
		{
			name:      "resource: statements group error mode does not affect default",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(resource.attributes["pass"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(resource.attributes["pass"], ParseJSON(true))`}},
			},
			wantErrorWith: "expected string but got bool",
		},
		{
			name:      "scope: statements group with error mode",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(scope.attributes["pass"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(scope.attributes["test"], "pass")`}},
			},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			name:      "scope: statements group error mode does not affect default",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(scope.attributes["pass"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(scope.attributes["pass"], ParseJSON(true))`}},
			},
			wantErrorWith: "expected string but got bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := constructMetrics()
			processor, err := NewProcessor(tt.statements, tt.errorMode, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)
			_, err = processor.ProcessMetrics(context.Background(), td)
			if tt.wantErrorWith != "" {
				if err == nil {
					t.Errorf("expected error containing '%s', got: <nil>", tt.wantErrorWith)
				}
				assert.Contains(t, err.Error(), tt.wantErrorWith)
				return
			}
			assert.NoError(t, err)
			exTd := constructMetrics()
			tt.want(exTd)
			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessMetrics_CacheAccess(t *testing.T) {
	tests := []struct {
		name       string
		statements []common.ContextStatements
		want       func(td pmetric.Metrics)
	}{
		{
			name: "resource:resource.cache",
			statements: []common.ContextStatements{
				{Statements: []string{`set(resource.cache["test"], "pass")`}, SharedCache: true},
				{Statements: []string{`set(resource.attributes["test"], resource.cache["test"])`}, SharedCache: true},
			},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).Resource().Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "resource:cache",
			statements: []common.ContextStatements{
				{
					Context: common.Resource,
					Statements: []string{
						`set(cache["test"], "pass")`,
						`set(attributes["test"], cache["test"])`,
					},
				},
			},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).Resource().Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "scope:scope.cache",
			statements: []common.ContextStatements{
				{Statements: []string{`set(scope.cache["test"], "pass")`}, SharedCache: true},
				{Statements: []string{`set(scope.attributes["test"], scope.cache["test"])`}, SharedCache: true},
			},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "scope:cache",
			statements: []common.ContextStatements{{
				Context: common.Scope,
				Statements: []string{
					`set(cache["test"], "pass")`,
					`set(attributes["test"], cache["test"])`,
				},
			}},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "metric:metric.cache",
			statements: []common.ContextStatements{
				{Statements: []string{`set(metric.cache["test"], "pass")`}, SharedCache: true},
				{Statements: []string{`set(metric.name, metric.cache["test"]) where metric.name == "operationB"`}, SharedCache: true},
			},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).SetName("pass")
			},
		},
		{
			name: "metric:cache",
			statements: []common.ContextStatements{{
				Context: common.Metric,
				Statements: []string{
					`set(cache["test"], "pass")`,
					`set(name, cache["test"]) where name == "operationB"`,
				},
			}},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).SetName("pass")
			},
		},
		{
			name: "datapoint:datapoint.cache",
			statements: []common.ContextStatements{
				{Statements: []string{`set(datapoint.cache["test"], "pass")`}, SharedCache: true},
				{Statements: []string{`set(datapoint.attributes["test"], datapoint.cache["test"]) where metric.name == "operationA"`}, SharedCache: true},
			},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "datapoint:cache",
			statements: []common.ContextStatements{{
				Context: common.DataPoint,
				Statements: []string{
					`set(cache["test"], "pass")`,
					`set(attributes["test"], cache["test"]) where metric.name == "operationA"`,
				},
			}},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "cache isolation",
			statements: []common.ContextStatements{
				{
					Statements:  []string{`set(datapoint.cache["shared"], "pass")`},
					SharedCache: true,
				},
				{
					Statements: []string{
						`set(datapoint.cache["test"], "fail")`,
						`set(datapoint.attributes["test"], datapoint.cache["test"])`,
						`set(datapoint.cache["shared"], "fail")`,
					},
					Conditions: []string{
						`metric.name == "operationA"`,
					},
				},
				{
					Context: common.DataPoint,
					Statements: []string{
						`set(attributes["extra"], cache["test"]) where cache["test"] != nil`,
						`set(cache["test"], "fail")`,
						`set(attributes["test"], cache["test"])`,
						`set(cache["shared"], "fail")`,
					},
					Conditions: []string{
						`metric.name == "operationA"`,
					},
				},
				{
					Statements:  []string{`set(datapoint.attributes["test"], "pass") where datapoint.cache["shared"] == "pass"`},
					SharedCache: true,
					Conditions: []string{
						`metric.name == "operationA"`,
					},
				},
			},
			want: func(td pmetric.Metrics) {
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("test", "pass")
				td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(1).Attributes().PutStr("test", "pass")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := constructMetrics()
			processor, err := NewProcessor(tt.statements, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessMetrics(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructMetrics()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_NewProcessor_ConditionsParse(t *testing.T) {
	type testCase struct {
		name          string
		statements    []common.ContextStatements
		wantErrorWith string
	}

	contextsTests := map[string][]testCase{"metric": nil, "datapoint": nil, "resource": nil, "scope": nil}
	for ctx := range contextsTests {
		contextsTests[ctx] = []testCase{
			{
				name: "inferred: condition with context",
				statements: []common.ContextStatements{
					{
						Statements: []string{fmt.Sprintf(`set(%s.cache["test"], "pass")`, ctx)},
						Conditions: []string{fmt.Sprintf(`%s.cache["test"] == ""`, ctx)},
					},
				},
			},
			{
				name: "inferred: condition without context",
				statements: []common.ContextStatements{
					{
						Statements: []string{fmt.Sprintf(`set(%s.cache["test"], "pass")`, ctx)},
						Conditions: []string{`cache["test"] == ""`},
					},
				},
				wantErrorWith: `missing context name for path "cache[test]"`,
			},
			{
				name: "context defined: condition without context",
				statements: []common.ContextStatements{
					{
						Context:    common.ContextID(ctx),
						Statements: []string{`set(cache["test"], "pass")`},
						Conditions: []string{`cache["test"] == ""`},
					},
				},
			},
			{
				name: "context defined: condition with context",
				statements: []common.ContextStatements{
					{
						Context:    common.ContextID(ctx),
						Statements: []string{`set(cache["test"], "pass")`},
						Conditions: []string{fmt.Sprintf(`%s.cache["test"] == ""`, ctx)},
					},
				},
			},
		}
	}

	for ctx, tests := range contextsTests {
		t.Run(ctx, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					_, err := NewProcessor(tt.statements, ottl.PropagateError, componenttest.NewNopTelemetrySettings())
					if tt.wantErrorWith != "" {
						if err == nil {
							t.Errorf("expected error containing '%s', got: <nil>", tt.wantErrorWith)
						}
						assert.Contains(t, err.Error(), tt.wantErrorWith)
						return
					}
					require.NoError(t, err)
				})
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

	dataPoint0 := m.SetEmptySum().DataPoints().AppendEmpty()
	dataPoint0.SetStartTimestamp(StartTimestamp)
	dataPoint0.SetDoubleValue(1.0)
	dataPoint0.Attributes().PutStr("attr1", "test1")

	dataPoint1 := m.Sum().DataPoints().AppendEmpty()
	dataPoint1.SetStartTimestamp(StartTimestamp)
	dataPoint1.SetDoubleValue(3.7)
	dataPoint1.Attributes().PutStr("attr1", "test2")
}
