// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	"github.com/elastic/go-docappender/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
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

func TestSyncBulkIndexer(t *testing.T) {
	tests := []struct {
		name         string
		responseBody string
		wantMessage  string
		wantFields   []zap.Field
	}{
		{
			name:         "success",
			responseBody: successResp,
		},
		{
			name:         "document_error_with_metadata",
			responseBody: `{"items":[{"create":{"_index":"foo","status":400,"error":{"type":"version_conflict_engine_exception","reason":"document already exists"}}}]}`,
			wantMessage:  "failed to index document",
			wantFields:   []zap.Field{zap.Strings("x-test", []string{"test"})},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reqCnt atomic.Int64
			cfg := Config{
				QueueBatchConfig: configoptional.Default(exporterhelper.QueueBatchConfig{
					NumConsumers: 1,
				}),
				MetadataKeys: []string{"x-test"},
			}
			esClient, err := elastictransport.New(elastictransport.Config{
				URLs: []*url.URL{{Scheme: "http", Host: "localhost:9200"}},
				Transport: &mockTransport{
					RoundTripFunc: func(r *http.Request) (*http.Response, error) {
						if r.URL.Path == "/_bulk" {
							reqCnt.Add(1)
						}
						return &http.Response{
							Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
							Body:       io.NopCloser(strings.NewReader(tt.responseBody)),
							StatusCode: http.StatusOK,
						}, nil
					},
				},
			})
			require.NoError(t, err)

			ct := componenttest.NewTelemetry()
			tb, err := metadata.NewTelemetryBuilder(
				metadatatest.NewSettings(ct).TelemetrySettings,
			)
			require.NoError(t, err)

			core, observed := observer.New(zap.NewAtomicLevelAt(zapcore.DebugLevel))
			bi := newSyncBulkIndexer(esClient, &cfg, false, tb, zap.New(core), nil)

			info := client.Info{Metadata: client.NewMetadata(map[string][]string{"x-test": {"test"}})}
			ctx := client.NewContext(t.Context(), info)
			session := bi.StartSession(ctx)
			assert.NoError(t, session.Add(ctx, "foo", "", "", strings.NewReader(`{"foo": "bar"}`), nil, docappender.ActionCreate))
			assert.Equal(t, int64(0), reqCnt.Load()) // requests will not flush unless flush is called explicitly
			assert.NoError(t, session.Flush(ctx))
			assert.Equal(t, int64(1), reqCnt.Load())
			session.End()
			assert.NoError(t, bi.Close(ctx))

			metadatatest.AssertEqualElasticsearchBulkRequestsCount(t, ct, []metricdata.DataPoint[int64]{
				{
					Value: 1,
					Attributes: attribute.NewSet(
						attribute.String("outcome", "success"), // bulk request itself is successful
						attribute.StringSlice("x-test", []string{"test"}),
						attribute.Int("http.response.status_code", http.StatusOK),
					),
				},
			}, metricdatatest.IgnoreTimestamp())
			metadatatest.AssertEqualElasticsearchDocsReceived(t, ct, []metricdata.DataPoint[int64]{
				{
					Value: 1,
					Attributes: attribute.NewSet(
						attribute.StringSlice("x-test", []string{"test"}),
					),
				},
			}, metricdatatest.IgnoreTimestamp())

			// For failure cases, verify error.type attribute is present
			attrs := []attribute.KeyValue{
				attribute.StringSlice("x-test", []string{"test"}),
				attribute.String("outcome", "success"),
				attribute.Int("http.response.status_code", http.StatusOK),
			}
			if tt.wantMessage != "" {
				attrs = []attribute.KeyValue{
					attribute.StringSlice("x-test", []string{"test"}),
					attribute.String("outcome", "failed_client"),
					attribute.Int("http.response.status_code", http.StatusBadRequest),
					attribute.String("error.type", "version_conflict_engine_exception"),
				}
			}
			metadatatest.AssertEqualElasticsearchDocsProcessed(t, ct, []metricdata.DataPoint[int64]{
				{
					Value:      1,
					Attributes: attribute.NewSet(attrs...),
				},
			}, metricdatatest.IgnoreTimestamp())
			metadatatest.AssertEqualElasticsearchFlushedBytes(t, ct, []metricdata.DataPoint[int64]{
				{
					Value: 43, // hard-coding the flush bytes since the input is fixed
					Attributes: attribute.NewSet(
						attribute.StringSlice("x-test", []string{"test"}),
					),
				},
			}, metricdatatest.IgnoreTimestamp())
			metadatatest.AssertEqualElasticsearchFlushedUncompressedBytes(t, ct, []metricdata.DataPoint[int64]{
				{
					Value: 43, // hard-coding the flush bytes since the input is fixed
					Attributes: attribute.NewSet(
						attribute.StringSlice("x-test", []string{"test"}),
					),
				},
			}, metricdatatest.IgnoreTimestamp())

			// Assert logs
			if tt.wantMessage != "" {
				messages := observed.FilterMessage(tt.wantMessage)
				require.Equal(t, 1, messages.Len(), "message not found; observed.All()=%v", observed.All())
				for _, wantField := range tt.wantFields {
					assert.Equal(t, 1, messages.FilterField(wantField).Len(), "message with field not found; observed.All()=%v", observed.All())
				}
			}
		})
	}
}

func TestSyncBulkIndexerRequestRetriesMetric(t *testing.T) {
	tests := []struct {
		name               string
		retryOnStatus      []int
		responseStatusCode int
		docsCount          int
		retryCount         int
	}{
		{
			name:               "retry_on_429_should_increment_retried_count",
			retryOnStatus:      []int{429},
			responseStatusCode: http.StatusTooManyRequests,
			docsCount:          3,
			retryCount:         6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reqCnt atomic.Int64
			cfg := Config{
				QueueBatchConfig: configoptional.Default(exporterhelper.QueueBatchConfig{
					NumConsumers: 1,
				}),
				MetadataKeys: []string{"x-test"},
				Retry: RetrySettings{
					Enabled:       true,
					RetryOnStatus: tt.retryOnStatus,
					MaxRetries:    tt.retryCount,
				},
			}

			esClient, err := elastictransport.New(elastictransport.Config{
				EnableMetrics: true,
				URLs:          []*url.URL{{Scheme: "http", Host: "localhost:9200"}},
				RetryOnStatus: cfg.Retry.RetryOnStatus,
				MaxRetries:    cfg.Retry.MaxRetries,
				DisableRetry:  !cfg.Retry.Enabled,
				Interceptors: []elastictransport.InterceptorFunc{
					countRetriesInterceptor(),
				},
				Transport: &mockTransport{
					RoundTripFunc: func(r *http.Request) (*http.Response, error) {
						if r.URL.Path == "/_bulk" {
							reqCnt.Add(1)

							if reqCnt.Load() <= int64(tt.retryCount) {
								return &http.Response{
									Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
									Body:       io.NopCloser(strings.NewReader("{}")),
									StatusCode: tt.responseStatusCode,
								}, nil
							}
						}

						return &http.Response{
							Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
							Body:       io.NopCloser(strings.NewReader(successResp)),
							StatusCode: http.StatusOK,
						}, nil
					},
				},
			})
			require.NoError(t, err)

			expectedRequestCount := int64(tt.retryCount + 1) // retry attempts + success

			ct := componenttest.NewTelemetry()
			tb, err := metadata.NewTelemetryBuilder(
				metadatatest.NewSettings(ct).TelemetrySettings,
			)
			require.NoError(t, err)

			core, _ := observer.New(zap.NewAtomicLevelAt(zapcore.DebugLevel))
			bi := newSyncBulkIndexer(esClient, &cfg, false, tb, zap.New(core), nil)

			info := client.Info{Metadata: client.NewMetadata(map[string][]string{"x-test": {"test"}})}
			ctx := client.NewContext(t.Context(), info)
			session := bi.StartSession(ctx)

			// Add multiple documents to test batch retry count
			for i := 0; i < tt.docsCount; i++ {
				assert.NoError(t, session.Add(ctx, "foo", "", "", strings.NewReader(`{"foo": "bar"}`), nil, docappender.ActionCreate))
			}

			assert.Equal(t, int64(0), reqCnt.Load()) // requests will not flush unless flush is called explicitly
			assert.NoError(t, session.Flush(ctx))    // After retries, flush should succeed
			assert.Equal(t, expectedRequestCount, reqCnt.Load())
			session.End()
			assert.NoError(t, bi.Close(ctx))

			// Assert elasticsearch docs retried metric
			metadatatest.AssertEqualElasticsearchDocsRetriedHTTPRequest(t, ct, []metricdata.DataPoint[int64]{
				{
					Value: int64(tt.retryCount * tt.docsCount), // all docs in the batch are retried for each retry attempt
					Attributes: attribute.NewSet(
						attribute.StringSlice("x-test", []string{"test"}),
					),
				},
			}, metricdatatest.IgnoreTimestamp())
		})
	}
}

func TestBulkIndexerLogsStatusCode(t *testing.T) {
	responseBody := `{"errors": true, "items":[
		{"create":{"_index":"foo-200","status":200}},
		{"create":{"_index":"foo-400","status":400,"error":{"type":"error_400","reason":"status 400"}}},
		{"create":{"_index":"foo-401","status":401,"error":{"type":"error_401","reason":"status 401"}}},
		{"create":{"_index":"foo-429","status":429,"error":{"type":"error_429","reason":"status 429"}}},
		{"create":{"_index":"foo-500","status":500,"error":{"type":"error_500","reason":"status 500"}}}
	]}`
	statuses := []int{400, 401, 429, 500}

	cfg := Config{
		QueueBatchConfig: configoptional.Default(exporterhelper.QueueBatchConfig{
			NumConsumers: 1,
		}),
	}
	esClient, err := elastictransport.New(elastictransport.Config{
		URLs: []*url.URL{{Scheme: "http", Host: "localhost:9200"}},
		Transport: &mockTransport{
			RoundTripFunc: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
					Body:       io.NopCloser(strings.NewReader(responseBody)),
					StatusCode: http.StatusOK,
				}, nil
			},
		},
	})
	require.NoError(t, err)

	ct := componenttest.NewTelemetry()
	tb, err := metadata.NewTelemetryBuilder(
		metadatatest.NewSettings(ct).TelemetrySettings,
	)
	require.NoError(t, err)

	core, observed := observer.New(zap.NewAtomicLevelAt(zapcore.DebugLevel))
	bi := newSyncBulkIndexer(esClient, &cfg, false, tb, zap.New(core), nil)

	ctx := t.Context()
	session := bi.StartSession(ctx)
	// Add initial document to ensure we have at least one document to process.
	require.NoError(t, session.Add(ctx, "foo", "", "", strings.NewReader(`{"foo": "bar"}`), nil, docappender.ActionCreate))
	for range statuses {
		require.NoError(t, session.Add(ctx, "foo", "", "", strings.NewReader(`{"foo": "bar"}`), nil, docappender.ActionCreate))
	}
	require.NoError(t, session.Flush(ctx))
	session.End()
	assert.NoError(t, bi.Close(ctx))

	messages := observed.FilterMessage("failed to index document").FilterFieldKey("http.response.status_code")
	require.Equal(t, len(statuses), messages.Len(), "message not found; observed.All()=%v", observed.All())
	for i, status := range statuses {
		if i >= messages.Len() {
			t.Errorf("expected at least %d log messages, got %d", i+1, messages.Len())
			continue
		}
		msg := messages.All()[i]
		statusCode, ok := msg.ContextMap()["http.response.status_code"]
		if !ok {
			t.Errorf("http.response.status_code missing in log at index %d; msg: %+v", i, msg.ContextMap())
			continue
		}
		assert.Equal(t, int64(status), statusCode, "http.response.status_code does not match at index %d; msg: %s", i, msg.Message)
	}
}

func TestQueryParamsParsedFromEndpoints(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoints = []string{"http://localhost:9200?pipeline=test-pipeline&pretty=false&require_data_stream=true"}

	client, err := newElasticsearchClient(t.Context(), cfg, componenttest.NewNopHost(), componenttest.NewTelemetry().NewTelemetrySettings(), "")
	require.NoError(t, err)

	bi := bulkIndexerConfig(client, cfg, true, zaptest.NewLogger(t))
	require.Equal(t, map[string][]string{
		"pipeline":            {"test-pipeline"},
		"pretty":              {"false"},
		"require_data_stream": {"true"},
	}, bi.QueryParams)
}

func TestNewBulkIndexer(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoints = []string{"http://localhost:9200"}

	client, err := newElasticsearchClient(t.Context(), cfg, componenttest.NewNopHost(), componenttest.NewTelemetry().NewTelemetrySettings(), "")
	require.NoError(t, err)

	bi := newBulkIndexer(client, cfg, true, nil, nil, nil)
	t.Cleanup(func() { bi.Close(t.Context()) })
}

func TestGetErrorHint(t *testing.T) {
	tests := []struct {
		name      string
		mode      MappingMode
		index     string
		errorType string
		want      string
	}{
		{
			name:      "version_conflict_engine_exception with .ds-metrics- prefix",
			mode:      MappingNone,
			index:     ".ds-metrics-foo",
			errorType: "version_conflict_engine_exception",
			want:      errorHintKnownIssues,
		},
		{
			name:      "illegal_argument_exception in OTel mode",
			mode:      MappingOTel,
			index:     "logs-generic.otel-default",
			errorType: "illegal_argument_exception",
			want:      errorHintOTelMappingMode,
		},
		{
			name:      "other error type in OTel mode",
			mode:      MappingOTel,
			index:     "logs-generic.otel-default",
			errorType: "mapper_parsing_exception",
			want:      "",
		},
		{
			name:      "version_conflict_engine_exception without .ds-metrics- prefix",
			mode:      MappingNone,
			index:     "logs-foo",
			errorType: "version_conflict_engine_exception",
			want:      "",
		},
		{
			name:      "empty index and error type",
			mode:      MappingNone,
			index:     "",
			errorType: "",
			want:      "",
		},
		{
			name:      "illegal_argument_exception in ECS mode",
			mode:      MappingECS,
			index:     "metrics.apm.app-default",
			errorType: "illegal_argument_exception",
			want:      errorHintECSMappingMode,
		},
		{
			name:      "illegal_argument_exception in none mode",
			mode:      MappingNone,
			index:     "logs-generic-default",
			errorType: "illegal_argument_exception",
			want:      "",
		},
		{
			name:      "illegal_argument_exception in raw mode",
			mode:      MappingRaw,
			index:     "logs-generic-default",
			errorType: "illegal_argument_exception",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getErrorHint(tt.mode, tt.index, tt.errorType)
			assert.Equal(t, tt.want, got)
		})
	}
}

// BenchmarkRequireDataStreamPayloadOverhead benchmarks the payload size
// overhead of encoding require_data_stream at the document level vs
// omitting it, and reports compression impact on payload size.
func BenchmarkRequireDataStreamPayloadOverhead(b *testing.B) {
	for _, tc := range []struct {
		name              string
		requireDataStream bool
		numItems          int
		compressionLevel  int
	}{
		{name: "no_compression/without_rds/10_items", requireDataStream: false, numItems: 10},
		{name: "no_compression/with_rds/10_items", requireDataStream: true, numItems: 10},
		{name: "no_compression/without_rds/100_items", requireDataStream: false, numItems: 100},
		{name: "no_compression/with_rds/100_items", requireDataStream: true, numItems: 100},
		{name: "no_compression/without_rds/1000_items", requireDataStream: false, numItems: 1000},
		{name: "no_compression/with_rds/1000_items", requireDataStream: true, numItems: 1000},
		{name: "gzip_best_speed/without_rds/1000_items", requireDataStream: false, numItems: 1000, compressionLevel: gzip.BestSpeed},
		{name: "gzip_best_speed/with_rds/1000_items", requireDataStream: true, numItems: 1000, compressionLevel: gzip.BestSpeed},
		{name: "gzip_best_compression/without_rds/1000_items", requireDataStream: false, numItems: 1000, compressionLevel: gzip.BestCompression},
		{name: "gzip_best_compression/with_rds/1000_items", requireDataStream: true, numItems: 1000, compressionLevel: gzip.BestCompression},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				biCfg := newBenchBulkIndexerConfig(b, tc.compressionLevel)
				bi, err := docappender.NewBulkIndexer(biCfg)
				require.NoError(b, err)
				for range tc.numItems {
					doc := strings.NewReader(`{"@timestamp":"2024-01-01T00:00:00Z","message":"test log message with some realistic content"}`)
					err := bi.Add(docappender.BulkIndexerItem{
						Index:             "logs-generic-default",
						Body:              doc,
						Action:            "create",
						RequireDataStream: tc.requireDataStream,
					})
					require.NoError(b, err)
				}
				b.ReportMetric(float64(bi.UncompressedLen()), "uncompressed_payload_bytes")
				b.ReportMetric(float64(bi.Len()), "payload_bytes")
			}
		})
	}
}

func newBenchBulkIndexerConfig(tb testing.TB, compressionLevel int) docappender.BulkIndexerConfig {
	tb.Helper()
	client, err := elastictransport.New(elastictransport.Config{
		URLs: []*url.URL{{Scheme: "http", Host: "localhost:9200"}},
		Transport: &mockTransport{
			RoundTripFunc: func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
					Body:       io.NopCloser(strings.NewReader(`{"items":[]}`)),
					StatusCode: http.StatusOK,
				}, nil
			},
		},
	})
	require.NoError(tb, err)
	return docappender.BulkIndexerConfig{
		Client:           client,
		CompressionLevel: compressionLevel,
	}
}

func TestSyncBulkIndexer_SuppressConflictErrors(t *testing.T) {
	responseBody := `{"items":[{"create":{"_index":"foo","status":409,"error":{"type":"version_conflict_engine_exception","reason":"document already exists"}}}]}`

	cfg := Config{
		QueueBatchConfig: configoptional.Default(exporterhelper.QueueBatchConfig{
			NumConsumers: 1,
		}),
		SuppressConflictErrors: true,
	}

	esClient, err := elastictransport.New(elastictransport.Config{
		URLs: []*url.URL{{Scheme: "http", Host: "localhost:9200"}},
		Transport: &mockTransport{
			RoundTripFunc: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
					Body:       io.NopCloser(strings.NewReader(responseBody)),
					StatusCode: http.StatusOK,
				}, nil
			},
		},
	})
	require.NoError(t, err)

	ct := componenttest.NewTelemetry()
	tb, err := metadata.NewTelemetryBuilder(
		metadatatest.NewSettings(ct).TelemetrySettings,
	)
	require.NoError(t, err)

	core, observed := observer.New(zap.NewAtomicLevelAt(zapcore.DebugLevel))
	bi := newSyncBulkIndexer(esClient, &cfg, false, tb, zap.New(core), nil)

	ctx := t.Context()
	session := bi.StartSession(ctx)
	require.NoError(t, session.Add(ctx, "foo", "", "", strings.NewReader(`{"foo": "bar"}`), nil, docappender.ActionCreate))

	require.NoError(t, session.Flush(ctx))
	session.End()
	require.NoError(t, bi.Close(ctx))

	messages := observed.FilterMessage("failed to index document")
	assert.Equal(t, 0, messages.Len(), "expected no error logs for version_conflict_engine_exception when SuppressConflictErrors is true")
}
