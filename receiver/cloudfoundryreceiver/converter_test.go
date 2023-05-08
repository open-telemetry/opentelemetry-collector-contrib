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

package cloudfoundryreceiver

import (
	"testing"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
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

	metricSlice := pmetric.NewMetricSlice()

	convertEnvelopeToMetrics(&envelope, metricSlice, before)

	require.Equal(t, 1, metricSlice.Len())

	metric := metricSlice.At(0)
	assert.Equal(t, "gorouter.bad_gateways", metric.Name())
	assert.Equal(t, pmetric.MetricTypeSum, metric.Type())
	dataPoints := metric.Sum().DataPoints()
	assert.Equal(t, 1, dataPoints.Len())
	dataPoint := dataPoints.At(0)
	assert.Equal(t, pcommon.NewTimestampFromTime(now), dataPoint.Timestamp())
	assert.Equal(t, pcommon.NewTimestampFromTime(before), dataPoint.StartTimestamp())
	assert.Equal(t, 10.0, dataPoint.DoubleValue())

	assertAttributes(t, dataPoint.Attributes(), map[string]string{
		"org.cloudfoundry.source_id":  "uaa",
		"org.cloudfoundry.origin":     "gorouter",
		"org.cloudfoundry.deployment": "cf",
		"org.cloudfoundry.job":        "router",
		"org.cloudfoundry.index":      "bc276108-8282-48a5-bae7-c009c4392246",
		"org.cloudfoundry.ip":         "10.244.0.34",
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

	expectedAttributes := map[string]string{
		"org.cloudfoundry.source_id":           "5db33854-09a4-4519-ba71-af33a878df6f",
		"org.cloudfoundry.instance_id":         "0",
		"org.cloudfoundry.origin":              "rep",
		"org.cloudfoundry.deployment":          "cf",
		"org.cloudfoundry.process_type":        "web",
		"org.cloudfoundry.process_id":          "5db33854-09a4-4519-ba71-af33a878df6f",
		"org.cloudfoundry.process_instance_id": "78d4e6d9-ef14-4116-6dc8-9bd7",
		"org.cloudfoundry.job":                 "compute",
		"org.cloudfoundry.index":               "7505d2c9-beab-4aaa-afe3-41322ebcd13d",
		"org.cloudfoundry.ip":                  "10.0.4.8",
	}

	metricSlice := pmetric.NewMetricSlice()

	convertEnvelopeToMetrics(&envelope, metricSlice, before)

	require.Equal(t, 2, metricSlice.Len())
	memoryMetricPosition := 0

	if metricSlice.At(1).Name() == "rep.memory" {
		memoryMetricPosition = 1
	}

	metric := metricSlice.At(memoryMetricPosition)
	assert.Equal(t, "rep.memory", metric.Name())
	assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())
	assert.Equal(t, 1, metric.Gauge().DataPoints().Len())
	dataPoint := metric.Gauge().DataPoints().At(0)
	assert.Equal(t, pcommon.NewTimestampFromTime(now), dataPoint.Timestamp())
	assert.Equal(t, pcommon.NewTimestampFromTime(before), dataPoint.StartTimestamp())
	assert.Equal(t, 17046641.0, dataPoint.DoubleValue())
	assertAttributes(t, dataPoint.Attributes(), expectedAttributes)

	metric = metricSlice.At(1 - memoryMetricPosition)
	assert.Equal(t, "rep.disk", metric.Name())
	assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())
	assert.Equal(t, 1, metric.Gauge().DataPoints().Len())
	dataPoint = metric.Gauge().DataPoints().At(0)
	assert.Equal(t, pcommon.NewTimestampFromTime(now), dataPoint.Timestamp())
	assert.Equal(t, pcommon.NewTimestampFromTime(before), dataPoint.StartTimestamp())
	assert.Equal(t, 10231808.0, dataPoint.DoubleValue())
	assertAttributes(t, dataPoint.Attributes(), expectedAttributes)
}

func assertAttributes(t *testing.T, attributes pcommon.Map, expected map[string]string) {
	assert.Equal(t, len(expected), attributes.Len())

	for key, expectedValue := range expected {
		value, present := attributes.Get(key)
		assert.True(t, present, "Attribute %s presence", key)
		assert.Equal(t, expectedValue, value.Str(), "Attribute %s value", key)
	}
}
