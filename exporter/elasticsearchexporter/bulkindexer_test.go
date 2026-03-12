// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
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
		"pipeline": {"test-pipeline"},
		"pretty":   {"false"},
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
// omitting it.
func BenchmarkRequireDataStreamPayloadOverhead(b *testing.B) {
	for _, tc := range []struct {
		name              string
		requireDataStream bool
		numItems          int
	}{
		{name: "without_rds/10_items", requireDataStream: false, numItems: 10},
		{name: "with_rds/10_items", requireDataStream: true, numItems: 10},
		{name: "without_rds/100_items", requireDataStream: false, numItems: 100},
		{name: "with_rds/100_items", requireDataStream: true, numItems: 100},
		{name: "without_rds/1000_items", requireDataStream: false, numItems: 1000},
		{name: "with_rds/1000_items", requireDataStream: true, numItems: 1000},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				biCfg := newBenchBulkIndexerConfig(b)
				bi, err := docappender.NewBulkIndexer(biCfg)
				require.NoError(b, err)
				for i := 0; i < tc.numItems; i++ {
					doc := strings.NewReader(`{"@timestamp":"2024-01-01T00:00:00Z","message":"test log message with some realistic content"}`)
					err := bi.Add(docappender.BulkIndexerItem{
						Index:             "logs-generic-default",
						Body:              doc,
						Action:            "create",
						RequireDataStream: tc.requireDataStream,
					})
					require.NoError(b, err)
				}
				b.ReportMetric(float64(bi.Len()), "payload_bytes")
			}
		})
	}
}

// TestRequireDataStreamPayloadSizeImpact measures the byte overhead of
// require_data_stream encoded on the action line.
func TestRequireDataStreamPayloadSizeImpact(t *testing.T) {
	buildPayload := func(requireDataStream bool, numItems int) int {
		biCfg := newBenchBulkIndexerConfig(t)
		bi, err := docappender.NewBulkIndexer(biCfg)
		require.NoError(t, err)
		for i := 0; i < numItems; i++ {
			doc := strings.NewReader(`{"@timestamp":"2024-01-01T00:00:00Z","message":"test log message"}`)
			err := bi.Add(docappender.BulkIndexerItem{
				Index:             "logs-generic-default",
				Body:              doc,
				Action:            "create",
				RequireDataStream: requireDataStream,
			})
			require.NoError(t, err)
		}
		return bi.Len()
	}

	for _, numItems := range []int{1, 10, 100, 1000} {
		withoutRDS := buildPayload(false, numItems)
		withRDS := buildPayload(true, numItems)

		overhead := withRDS - withoutRDS
		overheadPercent := float64(overhead) / float64(withoutRDS) * 100

		t.Logf("Items: %4d | Without RDS: %6d bytes | With RDS: %6d bytes | Overhead: %4d bytes (%.2f%%)",
			numItems, withoutRDS, withRDS, overhead, overheadPercent)

		assert.Less(t, overheadPercent, 30.0,
			"require_data_stream overhead should be less than 30%% for %d items", numItems)
	}
}

func newBenchBulkIndexerConfig(tb testing.TB) docappender.BulkIndexerConfig {
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
	return docappender.BulkIndexerConfig{Client: client}
}
