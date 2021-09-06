// Copyright  OpenTelemetry Authors
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

package timestampprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"
)

type metricTimestampTest struct {
	name               string
	roundToNearest     time.Duration
	inMetrics          pdata.Metrics
	expectedDataPoints []testDataPoint
}

type testDataPoint struct {
	Timestamp pdata.Timestamp
	Name      string
}

var (
	standardTests = []metricTimestampTest{
		{
			name:           "timestamps end up rounded to nearest second",
			roundToNearest: time.Second,
			inMetrics: testResourceMetrics([]testDataPoint{
				{1626298669697344000, "a"},
				{1626298669697390000, "b"},
				{1626298669697574000, "c"},
				{1626298669697627000, "d"},
			}),
			expectedDataPoints: []testDataPoint{
				// 1626298670000000000 - max's value
				{1626298669000000000, "a"},
				{1626298669000000000, "b"},
				{1626298669000000000, "c"},
				{1626298669000000000, "d"},
			},
		},
		{
			name:           "timestamps more than 1 second apart end up rounded to nearest second",
			roundToNearest: time.Second,
			inMetrics: testResourceMetrics([]testDataPoint{

				{1626298670000000000, "a"},
				{1626298672300000000, "b"},
				{1626298673100000000, "c"},
				{1626298672600000000, "d"},
				{1626298673900000000, "e"},
			}),
			expectedDataPoints: []testDataPoint{
				{1626298670000000000, "a"},
				{1626298672000000000, "b"},
				{1626298673000000000, "c"},
				{1626298672000000000, "d"},
				{1626298673000000000, "e"},
			},
		},
	}
)

func TestTimestampProcessor(t *testing.T) {
	for _, test := range standardTests {
		t.Run(test.name, func(t *testing.T) {
			// next stores the results of the filter metric processor
			next := new(consumertest.MetricsSink)
			cfg := &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewID(typeStr)),
				RoundToNearest:    &test.roundToNearest,
			}
			factory := NewFactory()
			fmp, err := factory.CreateMetricsProcessor(
				context.Background(),
				componenttest.NewNopProcessorCreateSettings(),
				cfg,
				next,
			)
			assert.NotNil(t, fmp)
			assert.Nil(t, err)

			caps := fmp.Capabilities()
			assert.True(t, caps.MutatesData)
			ctx := context.Background()
			assert.NoError(t, fmp.Start(ctx, nil))

			cErr := fmp.ConsumeMetrics(context.Background(), test.inMetrics)
			assert.Nil(t, cErr)

			gotDataPoints := getDatapointListFromMetrics(next.AllMetrics())

			assert.Len(t, gotDataPoints, len(test.expectedDataPoints))

			for i, gotDataPoint := range gotDataPoints {
				expectedDataPoint := test.expectedDataPoints[i]
				assert.Equal(t, gotDataPoint.Name, expectedDataPoint.Name)
				assert.Equal(t, gotDataPoint.Timestamp, expectedDataPoint.Timestamp)
			}

			assert.NoError(t, fmp.Shutdown(ctx))
		})
	}
}

func testResourceMetrics(dataPoints []testDataPoint) pdata.Metrics {
	md := pdata.NewMetrics()
	for _, namedDataPoint := range dataPoints {
		rm := md.ResourceMetrics().AppendEmpty()
		ms := rm.InstrumentationLibraryMetrics().AppendEmpty().Metrics()
		m := ms.AppendEmpty()
		m.SetName(namedDataPoint.Name)
		m.SetDataType(pdata.MetricDataTypeGauge)
		dp := m.Gauge().DataPoints().AppendEmpty()
		dp.SetTimestamp(namedDataPoint.Timestamp)
		dp.SetDoubleVal(123)
	}
	return md
}

func UnwrapMetricsList(wrappedMetricsList []pdata.Metrics) (metricObjects []pdata.Metric) {
	for _, wrappedMetrics := range wrappedMetricsList {

		resourceMetrics := wrappedMetrics.ResourceMetrics()
		for i := 0; i < resourceMetrics.Len(); i++ {

			resourceMetric := resourceMetrics.At(i)
			instrumentationLibraryMetrics := resourceMetric.InstrumentationLibraryMetrics()
			for j := 0; j < instrumentationLibraryMetrics.Len(); j++ {

				instrumentationLibraryMetric := instrumentationLibraryMetrics.At(j)
				metrics := instrumentationLibraryMetric.Metrics()
				for k := 0; k < metrics.Len(); k++ {
					metricObjects = append(metricObjects, instrumentationLibraryMetric.Metrics().At(k))
				}
			}
		}
	}
	return
}

func getDatapointListFromMetrics(metricsList []pdata.Metrics) (dataPointsToReturn []testDataPoint) {
	for _, metric := range UnwrapMetricsList(metricsList) {
		switch metric.DataType() {
		case pdata.MetricDataTypeGauge:
			dataPoints := metric.Gauge().DataPoints()
			for l := 0; l < dataPoints.Len(); l++ {
				gotDataPoint := dataPoints.At(l)
				dataPointsToReturn = append(dataPointsToReturn, testDataPoint{gotDataPoint.Timestamp(), metric.Name()})
			}
		case pdata.MetricDataTypeSum:
			dataPoints := metric.Sum().DataPoints()
			for l := 0; l < dataPoints.Len(); l++ {
				gotDataPoint := dataPoints.At(l)
				dataPointsToReturn = append(dataPointsToReturn, testDataPoint{gotDataPoint.Timestamp(), metric.Name()})
			}
		case pdata.MetricDataTypeHistogram:
			dataPoints := metric.Histogram().DataPoints()
			for l := 0; l < dataPoints.Len(); l++ {
				gotDataPoint := dataPoints.At(l)
				dataPointsToReturn = append(dataPointsToReturn, testDataPoint{gotDataPoint.Timestamp(), metric.Name()})
			}
		case pdata.MetricDataTypeSummary:
			dataPoints := metric.Summary().DataPoints()
			for l := 0; l < dataPoints.Len(); l++ {
				gotDataPoint := dataPoints.At(l)
				dataPointsToReturn = append(dataPointsToReturn, testDataPoint{gotDataPoint.Timestamp(), metric.Name()})
			}
		}
	}
	return
}

func TestNilResourceMetrics(t *testing.T) {
	metrics := pdata.NewMetrics()
	rms := metrics.ResourceMetrics()
	rms.AppendEmpty()
	requireNotPanics(t, metrics)
}

func TestNilILM(t *testing.T) {
	metrics := pdata.NewMetrics()
	rms := metrics.ResourceMetrics()
	rm := rms.AppendEmpty()
	ilms := rm.InstrumentationLibraryMetrics()
	ilms.AppendEmpty()
	requireNotPanics(t, metrics)
}

func TestNilMetric(t *testing.T) {
	metrics := pdata.NewMetrics()
	rms := metrics.ResourceMetrics()
	rm := rms.AppendEmpty()
	ilms := rm.InstrumentationLibraryMetrics()
	ilm := ilms.AppendEmpty()
	ms := ilm.Metrics()
	ms.AppendEmpty()
	requireNotPanics(t, metrics)
}

func requireNotPanics(t *testing.T, metrics pdata.Metrics) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	pcfg := cfg.(*Config)
	oneSecond := time.Second
	pcfg.RoundToNearest = &oneSecond
	ctx := context.Background()
	proc, _ := factory.CreateMetricsProcessor(
		ctx,
		componenttest.NewNopProcessorCreateSettings(),
		cfg,
		consumertest.NewNop(),
	)
	require.NotPanics(t, func() {
		_ = proc.ConsumeMetrics(ctx, metrics)
	})
}
