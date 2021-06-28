// Copyright 2020, OpenTelemetry Authors
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

package elastic_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.elastic.co/apm/model"
	"go.elastic.co/apm/transport/transporttest"
	"go.elastic.co/fastjson"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticexporter/internal/translator/elastic"
)

func TestEncodeMetrics(t *testing.T) {
	var w fastjson.Writer
	var recorder transporttest.RecorderTransport
	elastic.EncodeResourceMetadata(pdata.NewResource(), &w)

	instrumentationLibraryMetrics := pdata.NewInstrumentationLibraryMetrics()
	metrics := instrumentationLibraryMetrics.Metrics()
	appendMetric := func(name string, dataType pdata.MetricDataType) pdata.Metric {
		n := metrics.Len()
		metrics.Resize(n + 1)
		metric := metrics.At(n)
		metric.SetName(name)
		metric.SetDataType(dataType)
		return metric
	}

	timestamp0 := time.Unix(123, 0).UTC()
	timestamp1 := time.Unix(456, 0).UTC()

	var expectDropped int

	metric := appendMetric("int_gauge_metric", pdata.MetricDataTypeIntGauge)
	intGauge := metric.IntGauge()
	intGauge.DataPoints().Resize(4)
	intGauge.DataPoints().At(0).SetTimestamp(pdata.TimestampFromTime(timestamp0))
	intGauge.DataPoints().At(0).SetValue(1)
	intGauge.DataPoints().At(1).SetTimestamp(pdata.TimestampFromTime(timestamp1))
	intGauge.DataPoints().At(1).SetValue(2)
	intGauge.DataPoints().At(1).LabelsMap().InitFromMap(map[string]string{"k": "v"})
	intGauge.DataPoints().At(2).SetTimestamp(pdata.TimestampFromTime(timestamp1))
	intGauge.DataPoints().At(2).SetValue(3)
	intGauge.DataPoints().At(3).SetTimestamp(pdata.TimestampFromTime(timestamp1))
	intGauge.DataPoints().At(3).SetValue(4)
	intGauge.DataPoints().At(3).LabelsMap().InitFromMap(map[string]string{"k": "v2"})

	metric = appendMetric("double_gauge_metric", pdata.MetricDataTypeDoubleGauge)
	doubleGauge := metric.DoubleGauge()
	doubleGauge.DataPoints().Resize(4)
	doubleGauge.DataPoints().At(0).SetTimestamp(pdata.TimestampFromTime(timestamp0))
	doubleGauge.DataPoints().At(0).SetValue(5)
	doubleGauge.DataPoints().At(1).SetTimestamp(pdata.TimestampFromTime(timestamp1))
	doubleGauge.DataPoints().At(1).SetValue(6)
	doubleGauge.DataPoints().At(1).LabelsMap().InitFromMap(map[string]string{"k": "v"})
	doubleGauge.DataPoints().At(2).SetTimestamp(pdata.TimestampFromTime(timestamp1))
	doubleGauge.DataPoints().At(2).SetValue(7)
	doubleGauge.DataPoints().At(3).SetTimestamp(pdata.TimestampFromTime(timestamp1))
	doubleGauge.DataPoints().At(3).SetValue(8)
	doubleGauge.DataPoints().At(3).LabelsMap().InitFromMap(map[string]string{"k": "v2"})

	metric = appendMetric("int_sum_metric", pdata.MetricDataTypeIntSum)
	intSum := metric.IntSum()
	intSum.DataPoints().Resize(3)
	intSum.DataPoints().At(0).SetTimestamp(pdata.TimestampFromTime(timestamp0))
	intSum.DataPoints().At(0).SetValue(9)
	intSum.DataPoints().At(1).SetTimestamp(pdata.TimestampFromTime(timestamp1))
	intSum.DataPoints().At(1).SetValue(10)
	intSum.DataPoints().At(1).LabelsMap().InitFromMap(map[string]string{"k": "v"})
	intSum.DataPoints().At(2).SetTimestamp(pdata.TimestampFromTime(timestamp1))
	intSum.DataPoints().At(2).SetValue(11)
	intSum.DataPoints().At(2).LabelsMap().InitFromMap(map[string]string{"k2": "v"})

	metric = appendMetric("double_sum_metric", pdata.MetricDataTypeDoubleSum)
	doubleSum := metric.DoubleSum()
	doubleSum.DataPoints().Resize(3)
	doubleSum.DataPoints().At(0).SetTimestamp(pdata.TimestampFromTime(timestamp0))
	doubleSum.DataPoints().At(0).SetValue(12)
	doubleSum.DataPoints().At(1).SetTimestamp(pdata.TimestampFromTime(timestamp1))
	doubleSum.DataPoints().At(1).SetValue(13)
	doubleSum.DataPoints().At(1).LabelsMap().InitFromMap(map[string]string{"k": "v"})
	doubleSum.DataPoints().At(2).SetTimestamp(pdata.TimestampFromTime(timestamp1))
	doubleSum.DataPoints().At(2).SetValue(14)
	doubleSum.DataPoints().At(2).LabelsMap().InitFromMap(map[string]string{"k2": "v"})

	// Histograms are currently not supported, and will be ignored.
	metric = appendMetric("double_histogram_metric", pdata.MetricDataTypeHistogram)
	metric.Histogram().DataPoints().Resize(1)
	expectDropped++
	metric = appendMetric("int_histogram_metric", pdata.MetricDataTypeIntHistogram)
	metric.IntHistogram().DataPoints().Resize(1)
	expectDropped++

	dropped, err := elastic.EncodeMetrics(metrics, instrumentationLibraryMetrics.InstrumentationLibrary(), &w)
	require.NoError(t, err)
	assert.Equal(t, expectDropped, dropped)
	sendStream(t, &w, &recorder)

	payloads := recorder.Payloads()
	assert.Equal(t, []model.Metrics{{
		Timestamp: model.Time(timestamp0),
		Samples: map[string]model.Metric{
			"double_gauge_metric": {Value: 5},
			"double_sum_metric":   {Value: 12},
			"int_gauge_metric":    {Value: 1},
			"int_sum_metric":      {Value: 9},
		},
	}, {
		Timestamp: model.Time(timestamp1),
		Samples: map[string]model.Metric{
			"double_gauge_metric": {Value: 7},
			"int_gauge_metric":    {Value: 3},
		},
	}, {
		Timestamp: model.Time(timestamp1),
		Labels:    model.StringMap{{Key: "k", Value: "v"}},
		Samples: map[string]model.Metric{
			"double_gauge_metric": {Value: 6},
			"double_sum_metric":   {Value: 13},
			"int_gauge_metric":    {Value: 2},
			"int_sum_metric":      {Value: 10},
		},
	}, {
		Timestamp: model.Time(timestamp1),
		Labels:    model.StringMap{{Key: "k", Value: "v2"}},
		Samples: map[string]model.Metric{
			"double_gauge_metric": {Value: 8},
			"int_gauge_metric":    {Value: 4},
		},
	}, {
		Timestamp: model.Time(timestamp1),
		Labels:    model.StringMap{{Key: "k2", Value: "v"}},
		Samples: map[string]model.Metric{
			"double_sum_metric": {Value: 14},
			"int_sum_metric":    {Value: 11},
		},
	}}, payloads.Metrics)

	assert.Empty(t, payloads.Errors)
}
