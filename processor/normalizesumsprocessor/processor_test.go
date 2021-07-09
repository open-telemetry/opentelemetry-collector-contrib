// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package normalizesumsprocessor

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
)

type testCase struct {
	name       string
	inputs     []pdata.Metrics
	expected   pdata.Metrics
	transforms []Transform
}

func TestNormalizeSumsProcessor(t *testing.T) {
	testStart := time.Now().Unix()
	tests := []testCase{
		{
			name:       "simple-case",
			inputs:     generateSimpleInput(testStart),
			expected:   generateSimpleInput(testStart)[0],
			transforms: make([]Transform, 0),
		},
		{
			name:     "removed-metric-case",
			inputs:   generateRemoveInput(testStart),
			expected: generateRemoveOutput(testStart),
			transforms: []Transform{
				{
					MetricName: "m1",
				},
			},
		},
		{
			name:     "one-metric-happy-case",
			inputs:   generateSimpleInput(testStart),
			expected: generateOneMetricHappyCaseOutput(testStart),
			transforms: []Transform{
				{
					MetricName: "m1",
				},
			},
		},
		{
			name:       "transform-all-happy-case",
			inputs:     generateLabelledInput(testStart),
			expected:   generateLabelledOutput(testStart),
			transforms: nil,
		},
		{
			name:       "transform-all-label-separated-case",
			inputs:     generateSeparatedLabelledInput(testStart),
			expected:   generateSeparatedLabelledOutput(testStart),
			transforms: nil,
		},
		{
			name:     "more-complex-case",
			inputs:   generateComplexInput(testStart),
			expected: generateComplexOutput(testStart),
			transforms: []Transform{
				{
					MetricName: "m1",
				},
				{
					MetricName: "m2",
					NewName:    "newM2",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nsp := newNormalizeSumsProcessor(zap.NewExample(), tt.transforms)

			tmn := &consumertest.MetricsSink{}
			id := config.NewID(typeStr)
			settings := config.NewProcessorSettings(id)
			rmp, err := processorhelper.NewMetricsProcessor(
				&Config{
					ProcessorSettings: &settings,
					Transforms:        tt.transforms,
				},
				tmn,
				nsp,
				processorhelper.WithCapabilities(processorCapabilities))
			require.NoError(t, err)

			assert.True(t, rmp.Capabilities().MutatesData)

			require.NoError(t, rmp.Start(context.Background(), componenttest.NewNopHost()))
			defer func() { assert.NoError(t, rmp.Shutdown(context.Background())) }()

			for _, input := range tt.inputs {
				err = rmp.ConsumeMetrics(context.Background(), input)
				require.NoError(t, err)
			}

			assertEqual(t, tt.expected, tmn.AllMetrics()[0])
		})
	}
}

func generateSimpleInput(startTime int64) []pdata.Metrics {
	input := pdata.NewMetrics()

	rmb := newResourceMetricsBuilder()
	b := rmb.addResourceMetrics(nil)

	mb1 := b.addMetric("m1", pdata.MetricDataTypeIntSum, true)
	mb1.addIntDataPoint(1, map[string]string{}, startTime, 0)
	mb1.addIntDataPoint(2, map[string]string{}, startTime+1000, 0)

	mb2 := b.addMetric("m2", pdata.MetricDataTypeDoubleSum, true)
	mb2.addDoubleDataPoint(3, map[string]string{}, startTime, 0)
	mb2.addDoubleDataPoint(4, map[string]string{}, startTime+1000, 0)

	mb3 := b.addMetric("m3", pdata.MetricDataTypeDoubleGauge, false)
	mb3.addDoubleDataPoint(5, map[string]string{}, startTime, 0)
	mb3.addDoubleDataPoint(6, map[string]string{}, startTime+1000, 0)

	rmb.Build().CopyTo(input.ResourceMetrics())
	return []pdata.Metrics{input}
}

func generateLabelledInput(startTime int64) []pdata.Metrics {
	input := pdata.NewMetrics()

	rmb := newResourceMetricsBuilder()
	b := rmb.addResourceMetrics(nil)

	mb1 := b.addMetric("m1", pdata.MetricDataTypeIntSum, true)
	mb1.addIntDataPoint(0, map[string]string{"label": "val1"}, startTime, 0)
	mb1.addIntDataPoint(3, map[string]string{"label": "val2"}, startTime, 0)
	mb1.addIntDataPoint(12, map[string]string{"label": "val1"}, startTime+1000, 0)
	mb1.addIntDataPoint(5, map[string]string{"label": "val2"}, startTime+1000, 0)
	mb1.addIntDataPoint(15, map[string]string{"label": "val1"}, startTime+2000, 0)
	mb1.addIntDataPoint(1, map[string]string{"label": "val2"}, startTime+2000, 0)
	mb1.addIntDataPoint(22, map[string]string{"label": "val1"}, startTime+3000, 0)
	mb1.addIntDataPoint(11, map[string]string{"label": "val2"}, startTime+3000, 0)

	rmb.Build().CopyTo(input.ResourceMetrics())
	return []pdata.Metrics{input}
}

func generateLabelledOutput(startTime int64) pdata.Metrics {
	output := pdata.NewMetrics()

	rmb := newResourceMetricsBuilder()
	b := rmb.addResourceMetrics(nil)

	mb1 := b.addMetric("m1", pdata.MetricDataTypeIntSum, true)
	// mb1.addIntDataPoint(1, map[string]string{"label": "val1"}, startTime, 0)
	// mb1.addIntDataPoint(1, map[string]string{"label": "val2"}, startTime, 0)
	mb1.addIntDataPoint(12, map[string]string{"label": "val1"}, startTime+1000, startTime)
	mb1.addIntDataPoint(2, map[string]string{"label": "val2"}, startTime+1000, startTime)
	mb1.addIntDataPoint(15, map[string]string{"label": "val1"}, startTime+2000, startTime)
	//mb1.addIntDataPoint(1, map[string]string{"label": "val2"}, startTime+2000, 1)
	mb1.addIntDataPoint(22, map[string]string{"label": "val1"}, startTime+3000, startTime)
	mb1.addIntDataPoint(10, map[string]string{"label": "val2"}, startTime+3000, startTime+2000)

	rmb.Build().CopyTo(output.ResourceMetrics())
	return output
}

func generateSeparatedLabelledInput(startTime int64) []pdata.Metrics {
	input := pdata.NewMetrics()

	rmb := newResourceMetricsBuilder()
	b := rmb.addResourceMetrics(nil)

	mb1 := b.addMetric("m1", pdata.MetricDataTypeIntSum, true)
	mb1.addIntDataPoint(0, map[string]string{"label": "val1"}, startTime, 0)
	mb1.addIntDataPoint(12, map[string]string{"label": "val1"}, startTime+1000, 0)
	mb1.addIntDataPoint(15, map[string]string{"label": "val1"}, startTime+2000, 0)
	mb1.addIntDataPoint(22, map[string]string{"label": "val1"}, startTime+3000, 0)

	mb2 := b.addMetric("m1", pdata.MetricDataTypeIntSum, true)
	mb2.addIntDataPoint(3, map[string]string{"label": "val2"}, startTime, 0)
	mb2.addIntDataPoint(5, map[string]string{"label": "val2"}, startTime+1000, 0)
	mb2.addIntDataPoint(1, map[string]string{"label": "val2"}, startTime+2000, 0)
	mb2.addIntDataPoint(11, map[string]string{"label": "val2"}, startTime+3000, 0)

	rmb.Build().CopyTo(input.ResourceMetrics())
	return []pdata.Metrics{input}
}

func generateSeparatedLabelledOutput(startTime int64) pdata.Metrics {
	output := pdata.NewMetrics()

	rmb := newResourceMetricsBuilder()
	b := rmb.addResourceMetrics(nil)

	mb1 := b.addMetric("m1", pdata.MetricDataTypeIntSum, true)
	// mb1.addIntDataPoint(1, map[string]string{"label": "val1"}, startTime, 0)
	mb1.addIntDataPoint(12, map[string]string{"label": "val1"}, startTime+1000, startTime)
	mb1.addIntDataPoint(15, map[string]string{"label": "val1"}, startTime+2000, startTime)
	mb1.addIntDataPoint(22, map[string]string{"label": "val1"}, startTime+3000, startTime)

	mb2 := b.addMetric("m1", pdata.MetricDataTypeIntSum, true)
	// mb2.addIntDataPoint(1, map[string]string{"label": "val2"}, startTime, 0)
	mb2.addIntDataPoint(2, map[string]string{"label": "val2"}, startTime+1000, startTime)
	//mb2.addIntDataPoint(1, map[string]string{"label": "val2"}, startTime+2000, 1)
	mb2.addIntDataPoint(10, map[string]string{"label": "val2"}, startTime+3000, startTime+2000)

	rmb.Build().CopyTo(output.ResourceMetrics())
	return output
}

func generateOneMetricHappyCaseOutput(startTime int64) pdata.Metrics {
	output := pdata.NewMetrics()

	rmb := newResourceMetricsBuilder()
	b := rmb.addResourceMetrics(nil)

	mb1 := b.addMetric("m1", pdata.MetricDataTypeIntSum, true)
	// mb1.addIntDataPoint(1, map[string]string{"label1": "value1"}, startTime)
	mb1.addIntDataPoint(1, map[string]string{}, startTime+1000, startTime)

	mb2 := b.addMetric("m2", pdata.MetricDataTypeDoubleSum, true)
	mb2.addDoubleDataPoint(3, map[string]string{}, startTime, 0)
	mb2.addDoubleDataPoint(4, map[string]string{}, startTime+1000, 0)

	mb3 := b.addMetric("m3", pdata.MetricDataTypeDoubleGauge, false)
	mb3.addDoubleDataPoint(5, map[string]string{}, startTime, 0)
	mb3.addDoubleDataPoint(6, map[string]string{}, startTime+1000, 0)

	rmb.Build().CopyTo(output.ResourceMetrics())
	return output
}

func generateTransformAllCaseOutput(startTime int64) pdata.Metrics {
	output := pdata.NewMetrics()

	rmb := newResourceMetricsBuilder()
	b := rmb.addResourceMetrics(nil)

	mb1 := b.addMetric("m1", pdata.MetricDataTypeIntSum, true)
	// mb1.addIntDataPoint(1, map[string]string{"label1": "value1"}, startTime)
	mb1.addIntDataPoint(1, map[string]string{}, startTime+1000, startTime)

	mb2 := b.addMetric("m2", pdata.MetricDataTypeDoubleSum, true)
	//mb2.addDoubleDataPoint(3, map[string]string{}, startTime, 0)
	mb2.addDoubleDataPoint(1, map[string]string{}, startTime+1000, startTime)

	mb3 := b.addMetric("m3", pdata.MetricDataTypeDoubleGauge, false)
	mb3.addDoubleDataPoint(5, map[string]string{}, startTime, 0)
	mb3.addDoubleDataPoint(6, map[string]string{}, startTime+1000, 0)

	rmb.Build().CopyTo(output.ResourceMetrics())
	return output
}

func generateRemoveInput(startTime int64) []pdata.Metrics {
	input := pdata.NewMetrics()

	rmb := newResourceMetricsBuilder()
	b := rmb.addResourceMetrics(nil)

	mb1 := b.addMetric("m1", pdata.MetricDataTypeIntSum, true)
	mb1.addIntDataPoint(1, map[string]string{}, startTime, 0)

	mb2 := b.addMetric("m2", pdata.MetricDataTypeDoubleSum, true)
	mb2.addDoubleDataPoint(3, map[string]string{}, startTime, 0)
	mb2.addDoubleDataPoint(4, map[string]string{}, startTime+1000, 0)

	mb3 := b.addMetric("m3", pdata.MetricDataTypeDoubleGauge, false)
	mb3.addDoubleDataPoint(5, map[string]string{}, startTime, 0)
	mb3.addDoubleDataPoint(6, map[string]string{}, startTime+1000, 0)

	rmb.Build().CopyTo(input.ResourceMetrics())
	return []pdata.Metrics{input}
}

func generateRemoveOutput(startTime int64) pdata.Metrics {
	output := pdata.NewMetrics()

	rmb := newResourceMetricsBuilder()
	b := rmb.addResourceMetrics(nil)

	mb2 := b.addMetric("m2", pdata.MetricDataTypeDoubleSum, true)
	mb2.addDoubleDataPoint(3, map[string]string{}, startTime, 0)
	mb2.addDoubleDataPoint(4, map[string]string{}, startTime+1000, 0)

	mb3 := b.addMetric("m3", pdata.MetricDataTypeDoubleGauge, false)
	mb3.addDoubleDataPoint(5, map[string]string{}, startTime, 0)
	mb3.addDoubleDataPoint(6, map[string]string{}, startTime+1000, 0)

	rmb.Build().CopyTo(output.ResourceMetrics())
	return output
}

func generateComplexInput(startTime int64) []pdata.Metrics {
	list := []pdata.Metrics{}
	input := pdata.NewMetrics()

	rmb := newResourceMetricsBuilder()
	b := rmb.addResourceMetrics(nil)

	mb1 := b.addMetric("m1", pdata.MetricDataTypeIntSum, true)
	mb1.addIntDataPoint(1, map[string]string{}, startTime, 0)
	mb1.addIntDataPoint(2, map[string]string{}, startTime+1000, 0)
	mb1.addIntDataPoint(2, map[string]string{}, startTime+2000, 0)
	mb1.addIntDataPoint(5, map[string]string{}, startTime+3000, 0)
	mb1.addIntDataPoint(2, map[string]string{}, startTime+4000, 0)
	mb1.addIntDataPoint(4, map[string]string{}, startTime+5000, 0)

	mb2 := b.addMetric("m2", pdata.MetricDataTypeDoubleSum, true)
	mb2.addDoubleDataPoint(3, map[string]string{}, startTime, 0)
	mb2.addDoubleDataPoint(4, map[string]string{}, startTime+1000, 0)
	mb2.addDoubleDataPoint(5, map[string]string{}, startTime+2000, 0)
	mb2.addDoubleDataPoint(2, map[string]string{}, startTime, 0)
	mb2.addDoubleDataPoint(8, map[string]string{}, startTime+3000, 0)
	mb2.addDoubleDataPoint(2, map[string]string{}, startTime+10000, 0)
	mb2.addDoubleDataPoint(6, map[string]string{}, startTime+120000, 0)

	mb3 := b.addMetric("m3", pdata.MetricDataTypeDoubleGauge, false)
	mb3.addDoubleDataPoint(5, map[string]string{}, startTime, 0)
	mb3.addDoubleDataPoint(6, map[string]string{}, startTime+1000, 0)

	rmb.Build().CopyTo(input.ResourceMetrics())
	list = append(list, input)

	rmb = newResourceMetricsBuilder()
	b = rmb.addResourceMetrics(nil)

	mb1 = b.addMetric("m1", pdata.MetricDataTypeIntSum, true)
	mb1.addIntDataPoint(7, map[string]string{}, startTime+6000, 0)
	mb1.addIntDataPoint(9, map[string]string{}, startTime+7000, 0)

	return list
}

func generateComplexOutput(startTime int64) pdata.Metrics {
	output := pdata.NewMetrics()

	rmb := newResourceMetricsBuilder()
	b := rmb.addResourceMetrics(nil)

	mb1 := b.addMetric("m1", pdata.MetricDataTypeIntSum, true)
	// mb1.addIntDataPoint(1, map[string]string{}, startTime, 0)
	mb1.addIntDataPoint(1, map[string]string{}, startTime+1000, startTime)
	mb1.addIntDataPoint(1, map[string]string{}, startTime+2000, startTime)
	mb1.addIntDataPoint(4, map[string]string{}, startTime+3000, startTime)
	// mb1.addIntDataPoint(2, map[string]string{}, startTime+4000, 0)
	mb1.addIntDataPoint(2, map[string]string{}, startTime+5000, startTime+4000)

	mb1.addIntDataPoint(5, map[string]string{}, startTime+6000, startTime+4000)
	mb1.addIntDataPoint(7, map[string]string{}, startTime+7000, startTime+4000)

	mb2 := b.addMetric("newM2", pdata.MetricDataTypeDoubleSum, true)
	// mb2.addDoubleDataPoint(3, map[string]string{}, startTime, 0)
	mb2.addDoubleDataPoint(1, map[string]string{}, startTime+1000, startTime)
	mb2.addDoubleDataPoint(2, map[string]string{}, startTime+2000, startTime)
	// mb2.addDoubleDataPoint(2, map[string]string{}, startTime, 0)
	mb2.addDoubleDataPoint(5, map[string]string{}, startTime+3000, startTime)
	// mb2.addDoubleDataPoint(2, map[string]string{}, startTime+10000, 0)
	mb2.addDoubleDataPoint(4, map[string]string{}, startTime+120000, startTime+10000)

	mb3 := b.addMetric("m3", pdata.MetricDataTypeDoubleGauge, false)
	mb3.addDoubleDataPoint(5, map[string]string{}, startTime, 0)
	mb3.addDoubleDataPoint(6, map[string]string{}, startTime+1000, 0)

	rmb.Build().CopyTo(output.ResourceMetrics())
	return output
}

// builders to generate test metrics

type resourceMetricsBuilder struct {
	rms pdata.ResourceMetricsSlice
}

func newResourceMetricsBuilder() resourceMetricsBuilder {
	return resourceMetricsBuilder{rms: pdata.NewResourceMetricsSlice()}
}

func (rmsb resourceMetricsBuilder) addResourceMetrics(resourceAttributes map[string]pdata.AttributeValue) metricsBuilder {
	rm := rmsb.rms.AppendEmpty()

	if resourceAttributes != nil {
		rm.Resource().Attributes().InitFromMap(resourceAttributes)
	}

	rm.InstrumentationLibraryMetrics().Resize(1)
	ilm := rm.InstrumentationLibraryMetrics().At(0)

	return metricsBuilder{metrics: ilm.Metrics()}
}

func (rmsb resourceMetricsBuilder) Build() pdata.ResourceMetricsSlice {
	return rmsb.rms
}

type metricsBuilder struct {
	metrics pdata.MetricSlice
}

func (msb metricsBuilder) addMetric(name string, t pdata.MetricDataType, isMonotonic bool) metricBuilder {
	metric := msb.metrics.AppendEmpty()
	metric.SetName(name)
	metric.SetDataType(t)

	switch t {
	case pdata.MetricDataTypeIntSum:
		sum := metric.IntSum()
		sum.SetIsMonotonic(isMonotonic)
		sum.SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
	case pdata.MetricDataTypeDoubleSum:
		sum := metric.DoubleSum()
		sum.SetIsMonotonic(isMonotonic)
		sum.SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
	}

	return metricBuilder{metric: metric}
}

type metricBuilder struct {
	metric pdata.Metric
}

func (mb metricBuilder) addIntDataPoint(value int64, labels map[string]string, timestamp int64, startTimestamp int64) metricBuilder {
	switch mb.metric.DataType() {
	case pdata.MetricDataTypeIntSum:
		idp := mb.metric.IntSum().DataPoints().AppendEmpty()
		idp.LabelsMap().InitFromMap(labels)
		idp.SetValue(value)
		idp.SetTimestamp(pdata.TimestampFromTime(time.Unix(timestamp, 0)))
		if startTimestamp > 0 {
			idp.SetStartTimestamp(pdata.TimestampFromTime(time.Unix(startTimestamp, 0)))
		}
	case pdata.MetricDataTypeIntGauge:
		idp := mb.metric.IntGauge().DataPoints().AppendEmpty()
		idp.LabelsMap().InitFromMap(labels)
		idp.SetValue(value)
		idp.SetTimestamp(pdata.TimestampFromTime(time.Unix(timestamp, 0)))
		if startTimestamp > 0 {
			idp.SetStartTimestamp(pdata.TimestampFromTime(time.Unix(startTimestamp, 0)))
		}
	}

	return mb
}

func (mb metricBuilder) addDoubleDataPoint(value float64, labels map[string]string, timestamp int64, startTimestamp int64) metricBuilder {
	switch mb.metric.DataType() {
	case pdata.MetricDataTypeDoubleSum:
		ddp := mb.metric.DoubleSum().DataPoints().AppendEmpty()
		ddp.LabelsMap().InitFromMap(labels)
		ddp.SetValue(value)
		ddp.SetTimestamp(pdata.TimestampFromTime(time.Unix(timestamp, 0)))
		if startTimestamp > 0 {
			ddp.SetStartTimestamp(pdata.TimestampFromTime(time.Unix(startTimestamp, 0)))
		}
	case pdata.MetricDataTypeDoubleGauge:
		ddp := mb.metric.DoubleGauge().DataPoints().AppendEmpty()
		ddp.LabelsMap().InitFromMap(labels)
		ddp.SetValue(value)
		ddp.SetTimestamp(pdata.TimestampFromTime(time.Unix(timestamp, 0)))
		if startTimestamp > 0 {
			ddp.SetStartTimestamp(pdata.TimestampFromTime(time.Unix(startTimestamp, 0)))
		}
	}

	return mb
}

// assertEqual is required because Attribute & Label Maps are not sorted by default
// and we don't provide any guarantees on the order of transformed metrics
func assertEqual(t *testing.T, expected, actual pdata.Metrics) {
	rmsAct := actual.ResourceMetrics()
	rmsExp := expected.ResourceMetrics()
	require.Equal(t, rmsExp.Len(), rmsAct.Len())
	for i := 0; i < rmsAct.Len(); i++ {
		rmAct := rmsAct.At(i)
		rmExp := rmsExp.At(i)

		// assert equality of resource attributes
		assert.Equal(t, rmExp.Resource().Attributes().Sort(), rmAct.Resource().Attributes().Sort())

		// assert equality of IL metrics
		ilmsAct := rmAct.InstrumentationLibraryMetrics()
		ilmsExp := rmExp.InstrumentationLibraryMetrics()
		require.Equal(t, ilmsExp.Len(), ilmsAct.Len())
		for j := 0; j < ilmsAct.Len(); j++ {
			ilmAct := ilmsAct.At(j)
			ilmExp := ilmsExp.At(j)

			// assert equality of metrics
			metricsAct := ilmAct.Metrics()
			metricsExp := ilmExp.Metrics()
			require.Equal(t, metricsExp.Len(), metricsAct.Len())

			// build a map of expected metrics
			metricsExpMap := make(map[string]pdata.Metric, metricsExp.Len())
			for k := 0; k < metricsExp.Len(); k++ {
				metricsExpMap[metricsExp.At(k).Name()] = metricsExp.At(k)
			}

			for k := 0; k < metricsAct.Len(); k++ {
				metricAct := metricsAct.At(k)
				metricExp := metricsExp.At(k)

				// assert equality of descriptors
				assert.Equal(t, metricExp.Name(), metricAct.Name())
				assert.Equalf(t, metricExp.Description(), metricAct.Description(), "Metric %s", metricAct.Name())
				assert.Equalf(t, metricExp.Unit(), metricAct.Unit(), "Metric %s", metricAct.Name())
				assert.Equalf(t, metricExp.DataType(), metricAct.DataType(), "Metric %s", metricAct.Name())

				// assert equality of aggregation info & data points
				switch ty := metricAct.DataType(); ty {
				case pdata.MetricDataTypeIntSum:
					assert.Equal(t, metricAct.IntSum().AggregationTemporality(), metricExp.IntSum().AggregationTemporality(), "Metric %s", metricAct.Name())
					assert.Equal(t, metricAct.IntSum().IsMonotonic(), metricExp.IntSum().IsMonotonic(), "Metric %s", metricAct.Name())
					assertEqualIntDataPointSlice(t, metricAct.Name(), metricAct.IntSum().DataPoints(), metricExp.IntSum().DataPoints())
				case pdata.MetricDataTypeDoubleSum:
					assert.Equal(t, metricAct.DoubleSum().AggregationTemporality(), metricExp.DoubleSum().AggregationTemporality(), "Metric %s", metricAct.Name())
					assert.Equal(t, metricAct.DoubleSum().IsMonotonic(), metricExp.DoubleSum().IsMonotonic(), "Metric %s", metricAct.Name())
					assertEqualDoubleDataPointSlice(t, metricAct.Name(), metricAct.DoubleSum().DataPoints(), metricExp.DoubleSum().DataPoints())
				case pdata.MetricDataTypeIntGauge:
					assertEqualIntDataPointSlice(t, metricAct.Name(), metricAct.IntGauge().DataPoints(), metricExp.IntGauge().DataPoints())
				case pdata.MetricDataTypeDoubleGauge:
					assertEqualDoubleDataPointSlice(t, metricAct.Name(), metricAct.DoubleGauge().DataPoints(), metricExp.DoubleGauge().DataPoints())
				default:
					assert.Fail(t, "unexpected metric type", t)
				}
			}
		}
	}
}

func assertEqualIntDataPointSlice(t *testing.T, metricName string, idpsAct, idpsExp pdata.IntDataPointSlice) {
	// require.Equalf(t, idpsExp.Len(), idpsAct.Len(), "Metric %s", metricName)

	// build a map of expected data points
	idpsExpMap := make(map[string]pdata.IntDataPoint, idpsExp.Len())
	for k := 0; k < idpsExp.Len(); k++ {
		idpsExpMap[intDataPointKey(metricName, idpsExp.At(k))] = idpsExp.At(k)
	}

	for l := 0; l < idpsAct.Len(); l++ {
		idpAct := idpsAct.At(l)

		idpExp, ok := idpsExpMap[intDataPointKey(metricName, idpAct)]
		if !ok {
			require.Failf(t, fmt.Sprintf("no data point for %s", intDataPointKey(metricName, idpAct)), "Metric %s", metricName)
		}

		assert.Equalf(t, idpExp.LabelsMap().Sort(), idpAct.LabelsMap().Sort(), "Metric %s", metricName)
		assert.Equalf(t, idpExp.StartTimestamp(), idpAct.StartTimestamp(), "Metric %s", metricName)
		assert.Equalf(t, idpExp.Timestamp(), idpAct.Timestamp(), "Metric %s", metricName)
		assert.Equalf(t, idpExp.Value(), idpAct.Value(), "Metric %s", metricName)
	}
}

func assertEqualDoubleDataPointSlice(t *testing.T, metricName string, ddpsAct, ddpsExp pdata.DoubleDataPointSlice) {
	require.Equalf(t, ddpsExp.Len(), ddpsAct.Len(), "Metric %s", metricName)

	// build a map of expected data points
	ddpsExpMap := make(map[string]pdata.DoubleDataPoint, ddpsExp.Len())
	for k := 0; k < ddpsExp.Len(); k++ {
		ddpsExpMap[doubleDataPointKey(metricName, ddpsExp.At(k))] = ddpsExp.At(k)
	}

	for l := 0; l < ddpsAct.Len(); l++ {
		ddpAct := ddpsAct.At(l)

		ddpExp, ok := ddpsExpMap[doubleDataPointKey(metricName, ddpAct)]
		if !ok {
			require.Failf(t, fmt.Sprintf("no data point for %s", doubleDataPointKey(metricName, ddpAct)), "Metric %s", metricName)
		}

		assert.Equalf(t, ddpExp.LabelsMap().Sort(), ddpAct.LabelsMap().Sort(), "Metric %s", metricName)
		assert.Equalf(t, ddpExp.StartTimestamp(), ddpAct.StartTimestamp(), "Metric %s", metricName)
		assert.Equalf(t, ddpExp.Timestamp(), ddpAct.Timestamp(), "Metric %s", metricName)
		assert.InDeltaf(t, ddpExp.Value(), ddpAct.Value(), 0.00000001, "Metric %s", metricName)
	}
}

// doubleDataPointKey returns a key representing the data point
func doubleDataPointKey(metricName string, dataPoint pdata.DoubleDataPoint) string {
	otherLabelsLen := dataPoint.LabelsMap().Len()

	idx, otherLabels := 0, make([]string, otherLabelsLen)
	dataPoint.LabelsMap().Range(func(k string, v string) bool {
		otherLabels[idx] = k + "=" + v
		idx++
		return true
	})
	// sort the slice so that we consider labelsets
	// the same regardless of order
	sort.StringSlice(otherLabels).Sort()
	return metricName + "/" + dataPoint.StartTimestamp().String() + "-" + dataPoint.Timestamp().String() + "/" + strings.Join(otherLabels, ";")
}

// intDataPointKey returns a key representing the data point
func intDataPointKey(metricName string, dataPoint pdata.IntDataPoint) string {
	otherLabelsLen := dataPoint.LabelsMap().Len()

	idx, otherLabels := 0, make([]string, otherLabelsLen)
	dataPoint.LabelsMap().Range(func(k string, v string) bool {
		otherLabels[idx] = k + "=" + v
		idx++
		return true
	})
	// sort the slice so that we consider labelsets
	// the same regardless of order
	sort.StringSlice(otherLabels).Sort()
	return metricName + "/" + dataPoint.StartTimestamp().String() + "-" + dataPoint.Timestamp().String() + "/" + strings.Join(otherLabels, ";")
}
