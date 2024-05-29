// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudfoundryreceiver

import (
	"encoding/hex"
	"fmt"
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

func TestParseLogLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		id               string
		logLine          string
		expectedWordList []string
		expectedWordMap  map[string]string
	}{
		{
			id:               "good-rtr",
			logLine:          `www.example.com - [2024-05-21T15:40:13.892179798Z] "GET /articles/ssdfws HTTP/1.1" 200 0 110563 "-" "python-requests/2.26.0" "20.191.2.244:52238" "10.88.195.81:61222" x_forwarded_for:"18.21.57.150, 10.28.45.29, 35.16.25.46, 20.191.2.244" x_forwarded_proto:"https" vcap_request_id:"766afb19-1779-4bb9-65d4-f01306f9f912" response_time:0.191835 gorouter_time:0.000139 app_id:"e3267823-0938-43ce-85ff-003e3e3a5a23" app_index:"4" instance_id:"918dd283-a0ed-48be-7f0c-253b" x_cf_routererror:"-" x_forwarded_host:"www.example.com" x_b3_traceid:"766afb1917794bb965d4f01306f9f912" x_b3_spanid:"65d4f01306f9f912" x_b3_parentspanid:"-" b3:"766afb1917794bb965d4f01306f9f912-65d4f01306f9f912" traceparent:"00-766afb1917794bb965d4f01306f9f912-65d4f01306f9f912-01" tracestate:"gorouter=65d4f01306f9f912"`,
			expectedWordList: []string{"www.example.com", "-", "2024-05-21T15:40:13.892179798Z", "GET /articles/ssdfws HTTP/1.1", "200", "0", "110563", "-", "python-requests/2.26.0", "20.191.2.244:52238", "10.88.195.81:61222", `x_forwarded_for:"18.21.57.150, 10.28.45.29, 35.16.25.46, 20.191.2.244"`, `x_forwarded_proto:"https"`, `vcap_request_id:"766afb19-1779-4bb9-65d4-f01306f9f912"`, `response_time:0.191835`, `gorouter_time:0.000139`, `app_id:"e3267823-0938-43ce-85ff-003e3e3a5a23"`, `app_index:"4"`, `instance_id:"918dd283-a0ed-48be-7f0c-253b"`, `x_cf_routererror:"-"`, `x_forwarded_host:"www.example.com"`, `x_b3_traceid:"766afb1917794bb965d4f01306f9f912"`, `x_b3_spanid:"65d4f01306f9f912"`, `x_b3_parentspanid:"-"`, `b3:"766afb1917794bb965d4f01306f9f912-65d4f01306f9f912"`, `traceparent:"00-766afb1917794bb965d4f01306f9f912-65d4f01306f9f912-01"`, `tracestate:"gorouter=65d4f01306f9f912"`},
			expectedWordMap: map[string]string{
				"x_forwarded_for":   "18.21.57.150, 10.28.45.29, 35.16.25.46, 20.191.2.244",
				"x_forwarded_proto": "https",
				"vcap_request_id":   "766afb19-1779-4bb9-65d4-f01306f9f912",
				"response_time":     "0.191835",
				"gorouter_time":     "0.000139",
				"app_id":            "e3267823-0938-43ce-85ff-003e3e3a5a23",
				"app_index":         "4",
				"instance_id":       "918dd283-a0ed-48be-7f0c-253b",
				"x_cf_routererror":  "-",
				"x_forwarded_host":  "www.example.com",
				"x_b3_traceid":      "766afb1917794bb965d4f01306f9f912",
				"x_b3_spanid":       "65d4f01306f9f912",
				"x_b3_parentspanid": "-",
				"b3":                "766afb1917794bb965d4f01306f9f912-65d4f01306f9f912",
				"traceparent":       "00-766afb1917794bb965d4f01306f9f912-65d4f01306f9f912-01",
				"tracestate":        "gorouter=65d4f01306f9f912",
			},
		},
		{
			id:               "empty-wordmap-rtr",
			logLine:          `www.example.com - [2024-05-21T15:40:13.892179798Z] "GET /articles/ssdfws HTTP/1.1" 200 0 110563 "-" "python-requests/2.26.0" "20.191.2.244:52238" "10.88.195.81:61222"`,
			expectedWordList: []string{"www.example.com", "-", "2024-05-21T15:40:13.892179798Z", "GET /articles/ssdfws HTTP/1.1", "200", "0", "110563", "-", "python-requests/2.26.0", "20.191.2.244:52238", "10.88.195.81:61222"},
			expectedWordMap:  map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			wordList, wordMap := parseLogLine(tt.logLine)

			require.Equal(t, len(wordList), len(tt.expectedWordList))
			require.Equal(t, len(wordMap), len(tt.expectedWordMap))

			for wordExpectedIndex, wordExpected := range tt.expectedWordList {
				assert.Equal(t, wordExpected, wordList[wordExpectedIndex], "List Item %s value", wordList[wordExpectedIndex])
			}
			for wordExpectedKey, wordExpectedValue := range tt.expectedWordMap {
				value, present := wordMap[wordExpectedKey]
				assert.True(t, present, "Map Item %s presence", wordExpectedKey)
				assert.Equal(t, wordExpectedValue, value, "Map Item %s value", wordExpectedValue)
			}
		})
	}
}

func TestSetTraceIDs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		id       string
		logLine  string
		expected map[string]interface{}
	}{
		{
			id:      "w3c-zipkin",
			logLine: `www.example.com - [2024-05-21T15:40:13.892179798Z] "GET /articles/ssdfws HTTP/1.1" 200 0 110563 "-" "python-requests/2.26.0" "20.191.2.244:52238" "10.88.195.81:61222" x_forwarded_for:"18.21.57.150, 10.28.45.29, 35.16.25.46, 20.191.2.244" x_forwarded_proto:"https" vcap_request_id:"766afb19-1779-4bb9-65d4-f01306f9f912" response_time:0.191835 gorouter_time:0.000139 app_id:"e3267823-0938-43ce-85ff-003e3e3a5a23" app_index:"4" instance_id:"918dd283-a0ed-48be-7f0c-253b" x_cf_routererror:"-" x_forwarded_host:"www.example.com" x_b3_traceid:"766afb1917794bb965d4f01306f9f912" x_b3_spanid:"65d4f01306f9f912" x_b3_parentspanid:"-" b3:"766afb1917794bb965d4f01306f9f912-65d4f01306f9f912" traceparent:"00-766afb1917794bb965d4f01306f9f912-65d4f01306f9f912-01" tracestate:"gorouter=65d4f01306f9f912"`,
			expected: map[string]interface{}{
				"err":     nil,
				"traceID": "766afb1917794bb965d4f01306f9f912",
				"spanID":  "65d4f01306f9f912",
			},
		},
		{
			id:      "zipkin",
			logLine: `www.example.com - [2024-05-21T15:40:13.892179798Z] "GET /articles/ssdfws HTTP/1.1" 200 0 110563 "-" "python-requests/2.26.0" "20.191.2.244:52238" "10.88.195.81:61222" x_forwarded_for:"18.21.57.150, 10.28.45.29, 35.16.25.46, 20.191.2.244" x_forwarded_proto:"https" vcap_request_id:"766afb19-1779-4bb9-65d4-f01306f9f912" response_time:0.191835 gorouter_time:0.000139 app_id:"e3267823-0938-43ce-85ff-003e3e3a5a23" app_index:"4" instance_id:"918dd283-a0ed-48be-7f0c-253b" x_cf_routererror:"-" x_forwarded_host:"www.example.com" x_b3_traceid:"766afb1917794bb965d4f01306f9f912" x_b3_spanid:"65d4f01306f9f912" x_b3_parentspanid:"-" b3:"766afb1917794bb965d4f01306f9f912-65d4f01306f9f912"`,
			expected: map[string]interface{}{
				"err":     nil,
				"traceID": "766afb1917794bb965d4f01306f9f912",
				"spanID":  "65d4f01306f9f912",
			},
		},
		{
			id:      "w3c",
			logLine: `www.example.com - [2024-05-21T15:40:13.892179798Z] "GET /articles/ssdfws HTTP/1.1" 200 0 110563 "-" "python-requests/2.26.0" "20.191.2.244:52238" "10.88.195.81:61222" x_forwarded_for:"18.21.57.150, 10.28.45.29, 35.16.25.46, 20.191.2.244" x_forwarded_proto:"https" vcap_request_id:"766afb19-1779-4bb9-65d4-f01306f9f912" response_time:0.191835 gorouter_time:0.000139 app_id:"e3267823-0938-43ce-85ff-003e3e3a5a23" app_index:"4" instance_id:"918dd283-a0ed-48be-7f0c-253b" x_cf_routererror:"-" x_forwarded_host:"www.example.com" traceparent:"00-766afb1917794bb965d4f01306f9f912-65d4f01306f9f912-01" tracestate:"gorouter=65d4f01306f9f912"`,
			expected: map[string]interface{}{
				"err":     nil,
				"traceID": "766afb1917794bb965d4f01306f9f912",
				"spanID":  "65d4f01306f9f912",
			},
		},
		{
			id:      "not-found",
			logLine: `www.example.com - [2024-05-21T15:40:13.892179798Z] "GET /articles/ssdfws HTTP/1.1" 200 0 110563 "-" "python-requests/2.26.0" "20.191.2.244:52238" "10.88.195.81:61222" x_forwarded_for:"18.21.57.150, 10.28.45.29, 35.16.25.46, 20.191.2.244" x_forwarded_proto:"https" vcap_request_id:"766afb19-1779-4bb9-65d4-f01306f9f912" response_time:0.191835 gorouter_time:0.000139 app_id:"e3267823-0938-43ce-85ff-003e3e3a5a23" app_index:"4" instance_id:"918dd283-a0ed-48be-7f0c-253b" x_cf_routererror:"-" x_forwarded_host:"www.example.com"`,
			expected: map[string]interface{}{
				"err":     nil,
				"traceID": "00000000000000000000000000000000",
				"spanID":  "0000000000000000",
			},
		},
		{
			id:      "error-w3c",
			logLine: `www.example.com - [2024-05-21T15:40:13.892179798Z] "GET /articles/ssdfws HTTP/1.1" 200 0 110563 "-" "python-requests/2.26.0" "20.191.2.244:52238" "10.88.195.81:61222" x_forwarded_for:"18.21.57.150, 10.28.45.29, 35.16.25.46, 20.191.2.244" x_forwarded_proto:"https" vcap_request_id:"766afb19-1779-4bb9-65d4-f01306f9f912" response_time:0.191835 gorouter_time:0.000139 app_id:"e3267823-0938-43ce-85ff-003e3e3a5a23" app_index:"4" instance_id:"918dd283-a0ed-48be-7f0c-253b" x_cf_routererror:"-" x_forwarded_host:"www.example.com" traceparent:"00-766afb1917794bb965d4f01306f9f912-65d4f01306f9f9122-01" tracestate:"gorouter=65d4f01306f9f912"`, expected: map[string]interface{}{
				"err":     fmt.Errorf("encoding/hex: odd length hex string"),
				"traceID": "00000000000000000000000000000000",
				"spanID":  "0000000000000000",
			},
		},
		{
			id:      "error-w3c-format",
			logLine: `www.example.com - [2024-05-21T15:40:13.892179798Z] "GET /articles/ssdfws HTTP/1.1" 200 0 110563 "-" "python-requests/2.26.0" "20.191.2.244:52238" "10.88.195.81:61222" x_forwarded_for:"18.21.57.150, 10.28.45.29, 35.16.25.46, 20.191.2.244" x_forwarded_proto:"https" vcap_request_id:"766afb19-1779-4bb9-65d4-f01306f9f912" response_time:0.191835 gorouter_time:0.000139 app_id:"e3267823-0938-43ce-85ff-003e3e3a5a23" app_index:"4" instance_id:"918dd283-a0ed-48be-7f0c-253b" x_cf_routererror:"-" x_forwarded_host:"www.example.com" traceparent:"00-766afb1917794bb965d4f01306f9f912-65d4f01306f9f912" tracestate:"gorouter=65d4f01306f9f912"`, expected: map[string]interface{}{
				"err":     fmt.Errorf("traceId W3C key traceparent with format 00 not valid in log"),
				"traceID": "00000000000000000000000000000000",
				"spanID":  "0000000000000000",
			},
		},
		{
			id:      "error-zipkin-format",
			logLine: `www.example.com - [2024-05-21T15:40:13.892179798Z] "GET /articles/ssdfws HTTP/1.1" 200 0 110563 "-" "python-requests/2.26.0" "20.191.2.244:52238" "10.88.195.81:61222" x_forwarded_for:"18.21.57.150, 10.28.45.29, 35.16.25.46, 20.191.2.244" x_forwarded_proto:"https" vcap_request_id:"766afb19-1779-4bb9-65d4-f01306f9f912" response_time:0.191835 gorouter_time:0.000139 app_id:"e3267823-0938-43ce-85ff-003e3e3a5a23" app_index:"4" instance_id:"918dd283-a0ed-48be-7f0c-253b" x_cf_routererror:"-" x_forwarded_host:"www.example.com" x_b3_traceid:"766afb1917794bb965d4f01306f9f912" x_b3_spanid:"65d4f01306f9f912" x_b3_parentspanid:"-" b3:"766afb1917794bb965d4f01306f9f912-65d4f01306f9f912-01"`,
			expected: map[string]interface{}{
				"err":     fmt.Errorf("traceId Zipkin key b3 not valid in log"),
				"traceID": "00000000000000000000000000000000",
				"spanID":  "0000000000000000",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			traceID, spanID, err := getTracingIDs(tt.logLine)

			require.Equal(t, tt.expected["err"], err)
			assert.Equal(t, tt.expected["traceID"].(string), hex.EncodeToString(traceID[:]))
			assert.Equal(t, tt.expected["spanID"].(string), hex.EncodeToString(spanID[:]))

		})
	}
}

func TestConvertLogsEnvelope(t *testing.T) {
	now := time.Now()
	before := time.Now().Add(-time.Second)

	envelope := loggregator_v2.Envelope{
		Timestamp: before.UnixNano(),
		SourceId:  "uaa",
		Tags: map[string]string{
			"origin":     "gorouter",
			"deployment": "cf",
			"job":        "router",
			"index":      "bc276108-8282-48a5-bae7-c009c4392246",
			"ip":         "10.244.0.34",
		},
		Message: &loggregator_v2.Envelope_Log{
			Log: &loggregator_v2.Log{
				Payload: []byte("log message payload"),
				Type:    loggregator_v2.Log_OUT,
			},
		},
	}

	logSlice := plog.NewLogRecordSlice()
	e := convertEnvelopeToLogs(&envelope, logSlice, now)
	require.Equal(t, nil, e)
	require.Equal(t, 1, logSlice.Len())
	log := logSlice.At(0)
	assert.Equal(t, "log message payload", log.Body().AsString())
	assert.Equal(t, plog.SeverityNumberInfo.String(), log.SeverityText())
	assert.Equal(t, pcommon.NewTimestampFromTime(before), log.Timestamp())
	assert.Equal(t, pcommon.NewTimestampFromTime(now), log.ObservedTimestamp())
	assertAttributes(t, log.Attributes(), map[string]string{
		"org.cloudfoundry.source_id":  "uaa",
		"org.cloudfoundry.origin":     "gorouter",
		"org.cloudfoundry.deployment": "cf",
		"org.cloudfoundry.job":        "router",
		"org.cloudfoundry.index":      "bc276108-8282-48a5-bae7-c009c4392246",
		"org.cloudfoundry.ip":         "10.244.0.34",
	})
}

func assertAttributes(t *testing.T, attributes pcommon.Map, expected map[string]string) {
	assert.Equal(t, len(expected), attributes.Len())

	for key, expectedValue := range expected {
		value, present := attributes.Get(key)
		assert.True(t, present, "Attribute %s presence", key)
		assert.Equal(t, expectedValue, value.Str(), "Attribute %s value", key)
	}
}
