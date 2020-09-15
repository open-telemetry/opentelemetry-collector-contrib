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

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticexporter/internal/translator/elastic"
	"go.opentelemetry.io/collector/consumer/pdata"
)

func TestEncodeMetrics(t *testing.T) {
	var w fastjson.Writer
	var recorder transporttest.RecorderTransport
	elastic.EncodeResourceMetadata(pdata.NewResource(), &w)

	instrumentationLibraryMetrics := pdata.NewInstrumentationLibraryMetrics()
	instrumentationLibraryMetrics.InitEmpty()
	metrics := instrumentationLibraryMetrics.Metrics()
	metrics.Resize(3)

	timestamp0 := time.Unix(123, 0).UTC()
	timestamp1 := time.Unix(456, 0).UTC()

	metric := metrics.At(0)
	metric.SetName("double_gauge_metric")
	metric.SetDataType(pdata.MetricDataTypeDoubleGauge)
	doubleGauge := metric.DoubleGauge()
	doubleGauge.InitEmpty()
	doubleGauge.DataPoints().Resize(4)
	doubleGauge.DataPoints().At(0).SetTimestamp(pdata.TimestampUnixNano(timestamp0.UnixNano()))
	doubleGauge.DataPoints().At(0).SetValue(123)
	doubleGauge.DataPoints().At(1).SetTimestamp(pdata.TimestampUnixNano(timestamp1.UnixNano()))
	doubleGauge.DataPoints().At(1).SetValue(456)
	doubleGauge.DataPoints().At(1).LabelsMap().InitFromMap(map[string]string{"k": "v"})
	doubleGauge.DataPoints().At(2).SetTimestamp(pdata.TimestampUnixNano(timestamp1.UnixNano()))
	doubleGauge.DataPoints().At(2).SetValue(789)
	doubleGauge.DataPoints().At(3).SetTimestamp(pdata.TimestampUnixNano(timestamp1.UnixNano()))
	doubleGauge.DataPoints().At(3).SetValue(101112)
	doubleGauge.DataPoints().At(3).LabelsMap().InitFromMap(map[string]string{"k": "v2"})

	metric = metrics.At(1)
	metric.SetName("int_sum_metric")
	metric.SetDataType(pdata.MetricDataTypeIntSum)
	intSum := metric.IntSum()
	intSum.InitEmpty()
	intSum.DataPoints().Resize(3)
	intSum.DataPoints().At(0).SetTimestamp(pdata.TimestampUnixNano(timestamp0.UnixNano()))
	intSum.DataPoints().At(0).SetValue(321)
	intSum.DataPoints().At(1).SetTimestamp(pdata.TimestampUnixNano(timestamp1.UnixNano()))
	intSum.DataPoints().At(1).SetValue(654)
	intSum.DataPoints().At(1).LabelsMap().InitFromMap(map[string]string{"k": "v"})
	intSum.DataPoints().At(2).SetTimestamp(pdata.TimestampUnixNano(timestamp1.UnixNano()))
	intSum.DataPoints().At(2).SetValue(101112)
	intSum.DataPoints().At(2).LabelsMap().InitFromMap(map[string]string{"k2": "v"})

	// Histograms are currently not supported, and will be ignored.
	metric = metrics.At(2)
	metric.SetName("double_histogram_metric")
	metric.SetDataType(pdata.MetricDataTypeDoubleHistogram)
	doubleHistogram := metric.DoubleHistogram()
	doubleHistogram.InitEmpty()
	doubleHistogram.DataPoints().Resize(1)

	dropped, err := elastic.EncodeMetrics(metrics, instrumentationLibraryMetrics.InstrumentationLibrary(), &w)
	require.NoError(t, err)
	assert.Equal(t, 1, dropped) // histogram dropped
	sendStream(t, &w, &recorder)

	payloads := recorder.Payloads()
	assert.Equal(t, []model.Metrics{{
		Timestamp: model.Time(timestamp0),
		Samples: map[string]model.Metric{
			"double_gauge_metric": {Value: 123},
			"int_sum_metric":      {Value: 321},
		},
	}, {
		Timestamp: model.Time(timestamp1),
		Samples: map[string]model.Metric{
			"double_gauge_metric": {Value: 789},
		},
	}, {
		Timestamp: model.Time(timestamp1),
		Labels:    model.StringMap{{Key: "k", Value: "v"}},
		Samples: map[string]model.Metric{
			"double_gauge_metric": {Value: 456},
			"int_sum_metric":      {Value: 654},
		},
	}, {
		Timestamp: model.Time(timestamp1),
		Labels:    model.StringMap{{Key: "k", Value: "v2"}},
		Samples: map[string]model.Metric{
			"double_gauge_metric": {Value: 101112},
		},
	}, {
		Timestamp: model.Time(timestamp1),
		Labels:    model.StringMap{{Key: "k2", Value: "v"}},
		Samples: map[string]model.Metric{
			"int_sum_metric": {Value: 101112},
		},
	}}, payloads.Metrics)

	assert.Empty(t, payloads.Errors)
}
