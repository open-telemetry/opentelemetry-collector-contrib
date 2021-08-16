// Copyright 2019, OpenTelemetry Authors
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

package cloudfoundryreceiver

import (
	"testing"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestConvertCountEnvelope(t *testing.T) {
	now := time.Now()
	before := time.Now().Add(-time.Second)

	envelope := loggregator_v2.Envelope{
		Timestamp: now.UnixNano(),
		SourceId:  "uaa",
		Tags: map[string]string{
			"origin":     "gorouter",
			"deployment": "cf",
			"job":        "router",
			"index":      "bc276108-8282-48a5-bae7-c009c4392246",
			"ip":         "10.244.0.34",
		},
		Message: &loggregator_v2.Envelope_Counter{
			Counter: &loggregator_v2.Counter{
				Name:  "bad_gateways",
				Delta: 1,
				Total: 10,
			},
		},
	}

	metricSlice := pdata.NewMetricSlice()

	collectMetricsFromEnvelope(&envelope, metricSlice, before)
	assert.Equal(t, 1, metricSlice.Len())

	metric := metricSlice.At(0)
	assert.Equal(t, "gorouter.bad_gateways", metric.Name())
	assert.Equal(t, pdata.MetricDataTypeSum, metric.DataType())
	dataPoints := metric.Sum().DataPoints()
	assert.Equal(t, 1, dataPoints.Len())
	dataPoint := dataPoints.At(0)
	assert.Equal(t, pdata.TimestampFromTime(now), dataPoint.Timestamp())
	assert.Equal(t, pdata.TimestampFromTime(before), dataPoint.StartTimestamp())
	assert.Equal(t, 10.0, dataPoint.DoubleVal())

	assertLabels(t, dataPoint.LabelsMap(), map[string]string{
		"source_id":  "uaa",
		"origin":     "gorouter",
		"deployment": "cf",
		"job":        "router",
		"index":      "bc276108-8282-48a5-bae7-c009c4392246",
		"ip":         "10.244.0.34",
	})
}

func TestConvertGaugeEnvelope(t *testing.T) {
	now := time.Now()
	before := time.Now().Add(-time.Second)

	envelope := loggregator_v2.Envelope{
		Timestamp:  now.UnixNano(),
		SourceId:   "5db33854-09a4-4519-ba71-af33a878df6f",
		InstanceId: "0",
		Tags: map[string]string{
			"origin":              "rep",
			"deployment":          "cf",
			"process_type":        "web",
			"process_id":          "5db33854-09a4-4519-ba71-af33a878df6f",
			"process_instance_id": "78d4e6d9-ef14-4116-6dc8-9bd7",
			"job":                 "compute",
			"index":               "7505d2c9-beab-4aaa-afe3-41322ebcd13d",
			"ip":                  "10.0.4.8",
		},
		Message: &loggregator_v2.Envelope_Gauge{
			Gauge: &loggregator_v2.Gauge{
				Metrics: map[string]*loggregator_v2.GaugeValue{
					"memory": {
						Unit:  "bytes",
						Value: 17046641.0,
					},
					"disk": {
						Unit:  "bytes",
						Value: 10231808.0,
					},
				},
			},
		},
	}

	expectedLabels := map[string]string{
		"source_id":           "5db33854-09a4-4519-ba71-af33a878df6f",
		"instance_id":         "0",
		"origin":              "rep",
		"deployment":          "cf",
		"process_type":        "web",
		"process_id":          "5db33854-09a4-4519-ba71-af33a878df6f",
		"process_instance_id": "78d4e6d9-ef14-4116-6dc8-9bd7",
		"job":                 "compute",
		"index":               "7505d2c9-beab-4aaa-afe3-41322ebcd13d",
		"ip":                  "10.0.4.8",
	}

	metricSlice := pdata.NewMetricSlice()

	collectMetricsFromEnvelope(&envelope, metricSlice, before)
	assert.Equal(t, 2, metricSlice.Len())

	metric := metricSlice.At(0)
	assert.Equal(t, "rep.memory", metric.Name())
	assert.Equal(t, pdata.MetricDataTypeGauge, metric.DataType())
	assert.Equal(t, 1, metric.Gauge().DataPoints().Len())
	dataPoint := metric.Gauge().DataPoints().At(0)
	assert.Equal(t, pdata.TimestampFromTime(now), dataPoint.Timestamp())
	assert.Equal(t, pdata.TimestampFromTime(before), dataPoint.StartTimestamp())
	assert.Equal(t, 17046641.0, dataPoint.DoubleVal())
	assertLabels(t, dataPoint.LabelsMap(), expectedLabels)

	metric = metricSlice.At(1)
	assert.Equal(t, "rep.disk", metric.Name())
	assert.Equal(t, pdata.MetricDataTypeGauge, metric.DataType())
	assert.Equal(t, 1, metric.Gauge().DataPoints().Len())
	dataPoint = metric.Gauge().DataPoints().At(0)
	assert.Equal(t, pdata.TimestampFromTime(now), dataPoint.Timestamp())
	assert.Equal(t, pdata.TimestampFromTime(before), dataPoint.StartTimestamp())
	assert.Equal(t, 10231808.0, dataPoint.DoubleVal())
	assertLabels(t, dataPoint.LabelsMap(), expectedLabels)
}

func assertLabels(t *testing.T, labels pdata.StringMap, expected map[string]string) {
	assert.Equal(t, len(expected), labels.Len())

	for key, expectedValue := range expected {
		value, present := labels.Get(key)
		assert.True(t, present)
		assert.Equal(t, expectedValue, value)
	}
}
