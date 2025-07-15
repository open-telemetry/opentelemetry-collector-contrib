// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elastic/go-docappender/v2"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metadatatest"
)

var defaultRoundTripFunc = func(*http.Request) (*http.Response, error) {
	return &http.Response{
		Body: io.NopCloser(strings.NewReader("{}")),
	}, nil
}

type mockTransport struct {
	RoundTripFunc func(*http.Request) (*http.Response, error)
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.RoundTripFunc == nil {
		return defaultRoundTripFunc(req)
	}
	return t.RoundTripFunc(req)
}

const successResp = `{
  "took": 30,
  "errors": false,
  "items": [
    {
      "create": {
        "_index": "foo",
        "status": 201
      }
    }
  ]
}`

func TestAsyncBulkIndexer_flushOnClose(t *testing.T) {
	cfg := Config{NumWorkers: 1, Flush: FlushSettings{Interval: time.Hour, Bytes: 2 << 30}}
	client, err := elasticsearch.NewClient(elasticsearch.Config{Transport: &mockTransport{
		RoundTripFunc: func(*http.Request) (*http.Response, error) {
			return &http.Response{
				Header: http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
				Body:   io.NopCloser(strings.NewReader(successResp)),
			}, nil
		},
	}})
	require.NoError(t, err)

	runBulkIndexerOnce(t, &cfg, client)
}

func TestAsyncBulkIndexer_flush(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name:   "flush.bytes",
			config: Config{NumWorkers: 1, Flush: FlushSettings{Interval: time.Hour, Bytes: 1}},
		},
		{
			name:   "flush.interval",
			config: Config{NumWorkers: 1, Flush: FlushSettings{Interval: 50 * time.Millisecond, Bytes: 2 << 30}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client, err := elasticsearch.NewClient(elasticsearch.Config{Transport: &mockTransport{
				RoundTripFunc: func(*http.Request) (*http.Response, error) {
					return &http.Response{
						Header: http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
						Body:   io.NopCloser(strings.NewReader(successResp)),
					}, nil
				},
			}})
			require.NoError(t, err)

			ct := componenttest.NewTelemetry()
			tb, err := metadata.NewTelemetryBuilder(
				metadatatest.NewSettings(ct).TelemetrySettings,
			)
			require.NoError(t, err)
			bulkIndexer, err := newAsyncBulkIndexer(client, &tt.config, false, tb, zap.NewNop())
			require.NoError(t, err)

			session := bulkIndexer.StartSession(context.Background())
			assert.NoError(t, session.Add(context.Background(), "foo", "", "", strings.NewReader(`{"foo": "bar"}`), nil, docappender.ActionCreate))
			// should flush
			time.Sleep(100 * time.Millisecond)
			assert.NoError(t, session.Flush(context.Background()))
			session.End()
			assert.NoError(t, bulkIndexer.Close(context.Background()))
			// Assert internal telemetry metrics
			metadatatest.AssertEqualElasticsearchBulkRequestsCount(t, ct, []metricdata.DataPoint[int64]{
				{
					Value: 1,
					Attributes: attribute.NewSet(
						attribute.String("outcome", "success"),
					),
				},
			}, metricdatatest.IgnoreTimestamp())
			metadatatest.AssertEqualElasticsearchDocsReceived(t, ct, []metricdata.DataPoint[int64]{
				{Value: 1},
			}, metricdatatest.IgnoreTimestamp())
			metadatatest.AssertEqualElasticsearchDocsProcessed(t, ct, []metricdata.DataPoint[int64]{
				{
					Value: 1,
					Attributes: attribute.NewSet(
						attribute.String("outcome", "success"),
					),
				},
			}, metricdatatest.IgnoreTimestamp())
			metadatatest.AssertEqualElasticsearchFlushedUncompressedBytes(t, ct, []metricdata.DataPoint[int64]{
				{Value: 43}, // hard-coding the flush bytes since the input is fixed
			}, metricdatatest.IgnoreTimestamp())
			metadatatest.AssertEqualElasticsearchFlushedBytes(t, ct, []metricdata.DataPoint[int64]{
				{Value: 43}, // hard-coding the flush bytes since the input is fixed
			}, metricdatatest.IgnoreTimestamp())
		})
	}
}

func TestAsyncBulkIndexer_flush_error(t *testing.T) {
	tests := []struct {
		name                string
		roundTripFunc       func(*http.Request) (*http.Response, error)
		logFailedDocsInput  bool
		retrySettings       RetrySettings
		wantMessage         string
		wantFields          []zap.Field
		wantESBulkReqs      *metricdata.DataPoint[int64]
		wantESDocsProcessed *metricdata.DataPoint[int64]
		wantESDocsRetried   *metricdata.DataPoint[int64]
	}{
		{
			name: "500",
			roundTripFunc: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
					Body:       io.NopCloser(strings.NewReader("error")),
				}, nil
			},
			wantMessage: "bulk indexer flush error",
			wantESBulkReqs: &metricdata.DataPoint[int64]{
				Value: 1,
				Attributes: attribute.NewSet(
					attribute.String("outcome", "failed_server"),
					semconv.HTTPResponseStatusCode(500),
				),
			},
			wantESDocsProcessed: &metricdata.DataPoint[int64]{
				Value: 1,
				Attributes: attribute.NewSet(
					attribute.String("outcome", "failed_server"),
					semconv.HTTPResponseStatusCode(500),
				),
			},
		},
		{
			name: "429",
			roundTripFunc: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
					Body:       io.NopCloser(strings.NewReader("error")),
				}, nil
			},
			wantMessage: "bulk indexer flush error",
			wantESBulkReqs: &metricdata.DataPoint[int64]{
				Value: 1,
				Attributes: attribute.NewSet(
					attribute.String("outcome", "too_many"),
					semconv.HTTPResponseStatusCode(429),
				),
			},
			wantESDocsProcessed: &metricdata.DataPoint[int64]{
				Value: 1,
				Attributes: attribute.NewSet(
					attribute.String("outcome", "too_many"),
					semconv.HTTPResponseStatusCode(429),
				),
			},
		},
		{
			name: "429/with_retry",
			roundTripFunc: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
					Body: io.NopCloser(strings.NewReader(
						`{"items":[{"create":{"_index":"test","status":429}}]}`)),
				}, nil
			},
			retrySettings: RetrySettings{Enabled: true, MaxRetries: 5, RetryOnStatus: []int{429}},
			wantESBulkReqs: &metricdata.DataPoint[int64]{
				Value: 1,
				Attributes: attribute.NewSet(
					attribute.String("outcome", "success"),
				),
			},
			wantESDocsRetried: &metricdata.DataPoint[int64]{Value: 1},
		},
		{
			name: "500/doc_level",
			roundTripFunc: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
					Body: io.NopCloser(strings.NewReader(
						`{"items":[{"create":{"_index":"test","status":500,"error":{"type":"internal_server_error","reason":""}}}]}`)),
				}, nil
			},
			wantESBulkReqs: &metricdata.DataPoint[int64]{
				Value: 1,
				Attributes: attribute.NewSet(
					attribute.String("outcome", "success"),
				),
			},
			wantESDocsProcessed: &metricdata.DataPoint[int64]{
				Value: 1,
				Attributes: attribute.NewSet(
					attribute.String("outcome", "failed_server"),
				),
			},
		},
		{
			name: "transport error",
			roundTripFunc: func(*http.Request) (*http.Response, error) {
				return nil, errors.New("transport error")
			},
			wantMessage: "bulk indexer flush error",
			wantESBulkReqs: &metricdata.DataPoint[int64]{
				Value: 1,
				Attributes: attribute.NewSet(
					attribute.String("outcome", "internal_server_error"),
				),
			},
			wantESDocsProcessed: &metricdata.DataPoint[int64]{
				Value: 1,
				Attributes: attribute.NewSet(
					attribute.String("outcome", "internal_server_error"),
				),
			},
		},
		{
			name: "known version conflict error",
			roundTripFunc: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
					Body: io.NopCloser(strings.NewReader(
						`{"items":[{"create":{"_index":".ds-metrics-generic.otel-default","status":400,"error":{"type":"version_conflict_engine_exception","reason":""}}}]}`)),
				}, nil
			},
			wantMessage: "failed to index document",
			wantFields:  []zap.Field{zap.String("hint", "check the \"Known issues\" section of Elasticsearch Exporter docs")},
			wantESBulkReqs: &metricdata.DataPoint[int64]{
				Value: 1,
				Attributes: attribute.NewSet(
					attribute.String("outcome", "success"),
				),
			},
			wantESDocsProcessed: &metricdata.DataPoint[int64]{
				Value: 1,
				Attributes: attribute.NewSet(
					attribute.String("outcome", "failed_client"),
				),
			},
		},
		{
			name: "known version conflict error with logFailedDocsInput",
			roundTripFunc: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
					Body: io.NopCloser(strings.NewReader(
						`{"items":[{"create":{"_index":".ds-metrics-generic.otel-default","status":400,"error":{"type":"version_conflict_engine_exception","reason":""}}}]}`)),
				}, nil
			},
			logFailedDocsInput: true,
			wantMessage:        "failed to index document; input may contain sensitive data",
			wantFields: []zap.Field{
				zap.String("hint", "check the \"Known issues\" section of Elasticsearch Exporter docs"),
				zap.String("input", `{"create":{"_index":"foo"}}
{"foo": "bar"}
`),
			},
			wantESBulkReqs: &metricdata.DataPoint[int64]{
				Value: 1,
				Attributes: attribute.NewSet(
					attribute.String("outcome", "success"),
				),
			},
			wantESDocsProcessed: &metricdata.DataPoint[int64]{
				Value: 1,
				Attributes: attribute.NewSet(
					attribute.String("outcome", "failed_client"),
				),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := Config{
				NumWorkers: 1,
				Flush:      FlushSettings{Interval: time.Hour, Bytes: 1},
				Retry:      tt.retrySettings,
			}
			if tt.logFailedDocsInput {
				cfg.LogFailedDocsInput = true
			}
			client, err := elasticsearch.NewClient(elasticsearch.Config{Transport: &mockTransport{
				RoundTripFunc: tt.roundTripFunc,
			}})
			require.NoError(t, err)
			core, observed := observer.New(zap.NewAtomicLevelAt(zapcore.DebugLevel))

			ct := componenttest.NewTelemetry()
			tb, err := metadata.NewTelemetryBuilder(
				metadatatest.NewSettings(ct).TelemetrySettings,
			)
			require.NoError(t, err)
			bulkIndexer, err := newAsyncBulkIndexer(client, &cfg, false, tb, zap.New(core))
			require.NoError(t, err)
			defer bulkIndexer.Close(context.Background())

			session := bulkIndexer.StartSession(context.Background())
			assert.NoError(t, session.Add(context.Background(), "foo", "", "", strings.NewReader(`{"foo": "bar"}`), nil, docappender.ActionCreate))
			// should flush
			time.Sleep(100 * time.Millisecond)
			if tt.wantMessage != "" {
				messages := observed.FilterMessage(tt.wantMessage)
				require.Equal(t, 1, messages.Len(), "message not found; observed.All()=%v", observed.All())
				for _, wantField := range tt.wantFields {
					assert.Equal(t, 1, messages.FilterField(wantField).Len(), "message with field not found; observed.All()=%v", observed.All())
				}
			}
			assert.NoError(t, session.Flush(context.Background()))
			session.End()
			// Assert internal telemetry metrics
			if tt.wantESBulkReqs != nil {
				metadatatest.AssertEqualElasticsearchBulkRequestsCount(
					t, ct,
					[]metricdata.DataPoint[int64]{*tt.wantESBulkReqs},
					metricdatatest.IgnoreTimestamp(),
				)
			}
			if tt.wantESDocsProcessed != nil {
				metadatatest.AssertEqualElasticsearchDocsProcessed(
					t, ct,
					[]metricdata.DataPoint[int64]{*tt.wantESDocsProcessed},
					metricdatatest.IgnoreTimestamp(),
				)
			}
			if tt.wantESDocsRetried != nil {
				metadatatest.AssertEqualElasticsearchDocsRetried(
					t, ct,
					[]metricdata.DataPoint[int64]{*tt.wantESDocsRetried},
					metricdatatest.IgnoreTimestamp(),
				)
			}
		})
	}
}

func TestAsyncBulkIndexer_logRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "compression none",
			config: Config{
				NumWorkers:   1,
				ClientConfig: confighttp.ClientConfig{Compression: "none"},
				Flush:        FlushSettings{Interval: time.Hour, Bytes: 1e+8},
			},
		},
		{
			name: "compression gzip",
			config: Config{
				NumWorkers:   1,
				ClientConfig: confighttp.ClientConfig{Compression: "gzip"},
				Flush:        FlushSettings{Interval: time.Hour, Bytes: 1e+8},
			},
		},
		{
			name: "compression gzip - level 5",
			config: Config{
				NumWorkers:   1,
				ClientConfig: confighttp.ClientConfig{Compression: "gzip", CompressionParams: configcompression.CompressionParams{Level: 5}},
				Flush:        FlushSettings{Interval: time.Hour, Bytes: 1e+8},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			loggerCore, logObserver := observer.New(zap.DebugLevel)

			esLogger := clientLogger{
				Logger:          zap.New(loggerCore),
				logRequestBody:  true,
				logResponseBody: true,
			}

			client, err := elasticsearch.NewClient(elasticsearch.Config{
				Transport: &mockTransport{
					RoundTripFunc: func(*http.Request) (*http.Response, error) {
						return &http.Response{
							Header: http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
							Body:   io.NopCloser(strings.NewReader(successResp)),
						}, nil
					},
				},
				Logger: &esLogger,
			})
			require.NoError(t, err)

			runBulkIndexerOnce(t, &tt.config, client)

			records := logObserver.AllUntimed()
			require.Len(t, records, 1)

			assert.Equal(t, "/_bulk", records[0].ContextMap()["path"])
			assert.Equal(t, "{\"create\":{\"_index\":\"foo\"}}\n{\"foo\": \"bar\"}\n", records[0].ContextMap()["request_body"])
			assert.JSONEq(t, successResp, records[0].ContextMap()["response_body"].(string))
		})
	}
}

func runBulkIndexerOnce(t *testing.T, config *Config, client *elasticsearch.Client) *asyncBulkIndexer {
	ct := componenttest.NewTelemetry()
	tb, err := metadata.NewTelemetryBuilder(
		metadatatest.NewSettings(ct).TelemetrySettings,
	)
	require.NoError(t, err)
	bulkIndexer, err := newAsyncBulkIndexer(client, config, false, tb, zap.NewNop())
	require.NoError(t, err)

	session := bulkIndexer.StartSession(context.Background())
	assert.NoError(t, session.Add(context.Background(), "foo", "", "", strings.NewReader(`{"foo": "bar"}`), nil, docappender.ActionCreate))
	assert.NoError(t, session.Flush(context.Background()))
	session.End()
	assert.NoError(t, bulkIndexer.Close(context.Background()))
	// Assert internal telemetry metrics
	metadatatest.AssertEqualElasticsearchBulkRequestsCount(t, ct, []metricdata.DataPoint[int64]{
		{
			Value: 1,
			Attributes: attribute.NewSet(
				attribute.String("outcome", "success"),
			),
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualElasticsearchDocsReceived(t, ct, []metricdata.DataPoint[int64]{
		{Value: 1},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualElasticsearchDocsProcessed(t, ct, []metricdata.DataPoint[int64]{
		{
			Value: 1,
			Attributes: attribute.NewSet(
				attribute.String("outcome", "success"),
			),
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualElasticsearchFlushedUncompressedBytes(t, ct, []metricdata.DataPoint[int64]{
		{Value: 43}, // hard-coding the flush bytes since the input is fixed
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualElasticsearchFlushedBytes(
		t, ct,
		[]metricdata.DataPoint[int64]{{}},
		metricdatatest.IgnoreTimestamp(),
		metricdatatest.IgnoreValue(), // compression can change in test, ignore value
	)

	return bulkIndexer
}

func TestSyncBulkIndexer_flushBytes(t *testing.T) {
	var reqCnt atomic.Int64
	cfg := Config{NumWorkers: 1, Flush: FlushSettings{Interval: time.Hour, Bytes: 1}}
	client, err := elasticsearch.NewClient(elasticsearch.Config{Transport: &mockTransport{
		RoundTripFunc: func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/_bulk" {
				reqCnt.Add(1)
			}
			return &http.Response{
				Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
				Body:       io.NopCloser(strings.NewReader(successResp)),
				StatusCode: http.StatusOK,
			}, nil
		},
	}})
	require.NoError(t, err)

	ct := componenttest.NewTelemetry()
	tb, err := metadata.NewTelemetryBuilder(
		metadatatest.NewSettings(ct).TelemetrySettings,
	)
	require.NoError(t, err)
	bi := newSyncBulkIndexer(client, &cfg, false, tb, zap.NewNop())

	session := bi.StartSession(context.Background())
	assert.NoError(t, session.Add(context.Background(), "foo", "", "", strings.NewReader(`{"foo": "bar"}`), nil, docappender.ActionCreate))
	assert.Equal(t, int64(1), reqCnt.Load()) // flush due to flush::bytes
	assert.NoError(t, session.Flush(context.Background()))
	session.End()
	assert.NoError(t, bi.Close(context.Background()))
	// Assert internal telemetry metrics
	metadatatest.AssertEqualElasticsearchBulkRequestsCount(t, ct, []metricdata.DataPoint[int64]{
		{
			Value: 1, // empty session flush should be a no-op
			Attributes: attribute.NewSet(
				attribute.String("outcome", "success"),
			),
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualElasticsearchDocsReceived(t, ct, []metricdata.DataPoint[int64]{
		{Value: 1},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualElasticsearchDocsProcessed(t, ct, []metricdata.DataPoint[int64]{
		{
			Value: 1,
			Attributes: attribute.NewSet(
				attribute.String("outcome", "success"),
			),
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualElasticsearchFlushedBytes(t, ct, []metricdata.DataPoint[int64]{
		{Value: 43}, // hard-coding the flush bytes since the input is fixed
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualElasticsearchFlushedUncompressedBytes(t, ct, []metricdata.DataPoint[int64]{
		{Value: 43}, // hard-coding the flush bytes since the input is fixed
	}, metricdatatest.IgnoreTimestamp())
}
