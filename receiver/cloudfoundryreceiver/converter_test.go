// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudfoundryreceiver

import (
	"testing"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
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

	assertAttributes(t, map[string]string{
		"org.cloudfoundry.source_id":  "uaa",
		"org.cloudfoundry.origin":     "gorouter",
		"org.cloudfoundry.deployment": "cf",
		"org.cloudfoundry.job":        "router",
		"org.cloudfoundry.index":      "bc276108-8282-48a5-bae7-c009c4392246",
		"org.cloudfoundry.ip":         "10.244.0.34",
	}, dataPoint.Attributes())
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
	assertAttributes(t, expectedAttributes, dataPoint.Attributes())

	metric = metricSlice.At(1 - memoryMetricPosition)
	assert.Equal(t, "rep.disk", metric.Name())
	assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())
	assert.Equal(t, 1, metric.Gauge().DataPoints().Len())
	dataPoint = metric.Gauge().DataPoints().At(0)
	assert.Equal(t, pcommon.NewTimestampFromTime(now), dataPoint.Timestamp())
	assert.Equal(t, pcommon.NewTimestampFromTime(before), dataPoint.StartTimestamp())
	assert.Equal(t, 10231808.0, dataPoint.DoubleValue())
	assertAttributes(t, expectedAttributes, dataPoint.Attributes())
}

func TestConvertLogsEnvelope(t *testing.T) {
	now := time.Now()
	before := time.Now().Add(-time.Second)
	t.Parallel()
	tests := []struct {
		id       string
		envelope loggregator_v2.Envelope
		expected map[string]any
	}{
		{
			id: "normal-without-sourcetype-tag",
			envelope: loggregator_v2.Envelope{
				Timestamp: before.UnixNano(),
				SourceId:  "744e75bb-69d1-4cf4-b037-76875368097b",
				Tags:      map[string]string{},
				Message: &loggregator_v2.Envelope_Log{
					Log: &loggregator_v2.Log{
						Payload: []byte(`test-app. Says Hello. on index: 0`),
						Type:    loggregator_v2.Log_OUT,
					},
				},
			},
			expected: map[string]any{
				"Timestamp": before,
				"Attributes": map[string]string{
					"org.cloudfoundry.source_id": "744e75bb-69d1-4cf4-b037-76875368097b",
				},
				"Body":           `test-app. Says Hello. on index: 0`,
				"SeverityNumber": plog.SeverityNumberInfo,
				"SeverityText":   plog.SeverityNumberInfo.String(),
			},
		},
		{
			id: "json-log-with-sourcetype-error",
			envelope: loggregator_v2.Envelope{
				Timestamp: before.UnixNano(),
				SourceId:  "df75aec8-b937-4dc8-9b4d-c336e36e3895",
				Tags: map[string]string{
					"source_type": "APP/PROC/WEB",
					"origin":      "rep",
					"deployment":  "cf",
					"job":         "diego-cell",
					"index":       "bc276108-8282-48a5-bae7-c009c4392246",
					"ip":          "10.80.0.2",
				},
				Message: &loggregator_v2.Envelope_Log{
					Log: &loggregator_v2.Log{
						Payload: []byte(`{"timestamp":"2024-05-29T16:16:28.063062903Z","level":"info","source":"guardian","message":"guardian.api.garden-server.get-properties.got-properties","data":{"handle":"e885e8be-c6a7-43b1-5066-a821","session":"2.1.209666"}}`),
						Type:    loggregator_v2.Log_ERR,
					},
				},
			},
			expected: map[string]any{
				"Timestamp": before,
				"Attributes": map[string]string{
					"org.cloudfoundry.source_id":   "df75aec8-b937-4dc8-9b4d-c336e36e3895",
					"org.cloudfoundry.source_type": "APP/PROC/WEB",
					"org.cloudfoundry.origin":      "rep",
					"org.cloudfoundry.deployment":  "cf",
					"org.cloudfoundry.job":         "diego-cell",
					"org.cloudfoundry.index":       "bc276108-8282-48a5-bae7-c009c4392246",
					"org.cloudfoundry.ip":          "10.80.0.2",
				},
				"Body":           `{"timestamp":"2024-05-29T16:16:28.063062903Z","level":"info","source":"guardian","message":"guardian.api.garden-server.get-properties.got-properties","data":{"handle":"e885e8be-c6a7-43b1-5066-a821","session":"2.1.209666"}}`,
				"SeverityNumber": plog.SeverityNumberError,
				"SeverityText":   plog.SeverityNumberError.String(),
			},
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.id, func(t *testing.T) {
			logSlice := plog.NewLogRecordSlice()
			e := convertEnvelopeToLogs(&tt.envelope, logSlice, now)
			require.NoError(t, e)
			require.Equal(t, 1, logSlice.Len())
			log := logSlice.At(0)
			assert.Equal(t, tt.expected["Body"], log.Body().AsString())
			assert.Equal(t, tt.expected["SeverityText"], log.SeverityText())
			assert.Equal(t, pcommon.NewTimestampFromTime(tt.expected["Timestamp"].(time.Time)), log.Timestamp())
			assert.Equal(t, pcommon.NewTimestampFromTime(now), log.ObservedTimestamp())
			assertAttributes(t, tt.expected["Attributes"].(map[string]string), log.Attributes())
		})
	}
}

func assertAttributes(t *testing.T, expected map[string]string, attributes pcommon.Map) {
	assert.Equal(t, len(expected), attributes.Len())

	for key, expectedValue := range expected {
		value, present := attributes.Get(key)
		assert.True(t, present, "Attribute %s presence", key)
		assert.Equal(t, expectedValue, value.Str(), "Attribute %s value", key)
	}
}
