// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sematextexporter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/influxdata/influxdb-observability/common"
	"github.com/influxdata/line-protocol/v2/lineprotocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestSematextHTTPWriterBatchOptimizeTags(t *testing.T) {
	batch := &sematextHTTPWriterBatch{
		sematextHTTPWriter: &sematextHTTPWriter{
			logger:   common.NoopLogger{},
			token:    "test-token",
			hostname: "test-host",
		},
	}

	for _, testCase := range []struct {
		name         string
		m            map[string]string
		expectedTags []tag
	}{
		{
			name: "empty map",
			m:    map[string]string{},
			expectedTags: []tag{
				{"os.host", "test-host"},
				{"token", "test-token"},
			},
		},
		{
			name: "allowed tags only",
			m: map[string]string{
				"service.name": "test-service",
				"os.type":      "linux",
			},
			expectedTags: []tag{
				{"os.host", "test-host"},
				{"os.type", "linux"},
				{"service.name", "test-service"},
				{"token", "test-token"},
			},
		},
		{
			name: "mixed allowed and non-allowed tags",
			m: map[string]string{
				"service.name":    "test-service",
				"non.allowed.tag": "should-be-dropped",
				"os.type":         "linux",
				"random.tag":      "should-be-dropped-too",
			},
			expectedTags: []tag{
				{"os.host", "test-host"},
				{"os.type", "linux"},
				{"service.name", "test-service"},
				{"token", "test-token"},
			},
		},
		{
			name: "all allowed tags present",
			m: map[string]string{
				"service.name":              "test-service",
				"service.instance.id":       "instance-1",
				"process.pid":               "1234",
				"os.type":                   "linux",
				"http.response.status_code": "200",
				"network.protocol.version":  "1.1",
				"jvm.memory.type":           "heap",
				"http.request.method":       "GET",
				"jvm.gc.name":               "G1",
			},
			expectedTags: []tag{
				{"http.request.method", "GET"},
				{"http.response.status_code", "200"},
				{"jvm.gc.name", "G1"},
				{"jvm.memory.type", "heap"},
				{"network.protocol.version", "1.1"},
				{"os.host", "test-host"},
				{"os.type", "linux"},
				{"process.pid", "1234"},
				{"service.instance.id", "instance-1"},
				{"service.name", "test-service"},
				{"token", "test-token"},
			},
		},
		{
			name: "empty tag values",
			m: map[string]string{
				"service.name": "",
				"os.type":      "linux",
			},
			expectedTags: []tag{
				{"os.host", "test-host"},
				{"os.type", "linux"},
				{"token", "test-token"},
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			gotTags := batch.optimizeTags(testCase.m)
			assert.Equal(t, testCase.expectedTags, gotTags, "tags should match expected values")
		})
	}
}

func TestSematextHTTPWriterBatchMaxPayload(t *testing.T) {
	for _, testCase := range []struct {
		name                   string
		payloadMaxLines        int
		payloadMaxBytes        int
		expectMultipleRequests bool
	}{
		{
			name:                   "default",
			payloadMaxLines:        10_000,
			payloadMaxBytes:        10_000_000,
			expectMultipleRequests: false,
		},
		{
			name:                   "limit-lines",
			payloadMaxLines:        1,
			payloadMaxBytes:        10_000_000,
			expectMultipleRequests: true,
		},
		{
			name:                   "limit-bytes",
			payloadMaxLines:        10_000,
			payloadMaxBytes:        1,
			expectMultipleRequests: true,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var httpRequests []*http.Request

			mockHTTPService := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				httpRequests = append(httpRequests, r)
			}))
			t.Cleanup(mockHTTPService.Close)

			batch := &sematextHTTPWriterBatch{
				sematextHTTPWriter: &sematextHTTPWriter{
					encoderPool: sync.Pool{
						New: func() any {
							e := new(lineprotocol.Encoder)
							e.SetLax(false)
							e.SetPrecision(lineprotocol.Nanosecond)
							return e
						},
					},
					httpClient:      &http.Client{},
					writeURL:        mockHTTPService.URL,
					payloadMaxLines: testCase.payloadMaxLines,
					payloadMaxBytes: testCase.payloadMaxBytes,
					logger:          common.NoopLogger{},
					hostname:        "test-host",
					token:           "test-token",
				},
			}
			defer batch.httpClient.CloseIdleConnections()

			err := batch.EnqueuePoint(context.Background(), "m", map[string]string{"k": "v"}, map[string]any{"f": int64(1)}, time.Unix(1, 0), 0)
			require.NoError(t, err)
			err = batch.EnqueuePoint(context.Background(), "m", map[string]string{"k": "v"}, map[string]any{"f": int64(2)}, time.Unix(2, 0), 0)
			require.NoError(t, err)
			err = batch.WriteBatch(context.Background())
			require.NoError(t, err)

			if testCase.expectMultipleRequests {
				assert.Len(t, httpRequests, 2)
			} else {
				assert.Len(t, httpRequests, 1)
			}
		})
	}
}

func TestSematextHTTPWriterBatchEnqueuePointEmptyTagValue(t *testing.T) {
	var recordedRequest *http.Request
	var recordedRequestBody []byte
	noopHTTPServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if assert.Nil(t, recordedRequest) {
			var err error
			recordedRequest = r
			recordedRequestBody, err = io.ReadAll(r.Body)
			assert.NoError(t, err)
		}
	}))
	t.Cleanup(noopHTTPServer.Close)
	nowTime := time.Unix(1628605794, 318000000)

	sematextWriter, err := newSematextHTTPWriter(
		new(common.NoopLogger),
		&Config{
			MetricsConfig: MetricsConfig{
				MetricsEndpoint: noopHTTPServer.URL,
				AppToken:        "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			},
			Region: "US",
		},
		componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	sematextWriter.httpClient = noopHTTPServer.Client()
	sematextWriterBatch := sematextWriter.NewBatch()
	defer sematextWriter.httpClient.CloseIdleConnections()

	err = sematextWriterBatch.EnqueuePoint(
		context.Background(),
		"m",
		map[string]string{"k": "v", "empty": ""},
		map[string]any{"f": int64(1)},
		nowTime,
		common.InfluxMetricValueTypeUntyped)
	require.NoError(t, err)

	err = sematextWriterBatch.WriteBatch(context.Background())

	require.NoError(t, err)

	if assert.NotNil(t, recordedRequest) {
		expected := fmt.Sprintf("m,os.host=%s,token=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx f=1i 1628605794318000000", sematextWriter.hostname)
		assert.Equal(t, expected, strings.TrimSpace(string(recordedRequestBody)))
	}
}

func TestComposeWriteURLDoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		cfg := &Config{
			Region: "us",
			MetricsConfig: MetricsConfig{
				MetricsEndpoint: "http://localhost:8080",
				MetricsSchema:   "telegraf-prometheus-v2",
			},
		}
		_, err := composeWriteURL(cfg)
		assert.NoError(t, err)
	})

	assert.NotPanics(t, func() {
		cfg := &Config{
			Region: "eu",
			MetricsConfig: MetricsConfig{
				MetricsEndpoint: "http://localhost:8080",
				MetricsSchema:   "telegraf-prometheus-v2",
			},
		}
		_, err := composeWriteURL(cfg)
		assert.NoError(t, err)
	})
}
