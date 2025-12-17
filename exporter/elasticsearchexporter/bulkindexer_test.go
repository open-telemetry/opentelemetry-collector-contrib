// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/elastic/go-docappender/v2"
	"github.com/elastic/go-elasticsearch/v8"
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
			esClient, err := elasticsearch.NewClient(elasticsearch.Config{Transport: &mockTransport{
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
			}})
			require.NoError(t, err)

			ct := componenttest.NewTelemetry()
			tb, err := metadata.NewTelemetryBuilder(
				metadatatest.NewSettings(ct).TelemetrySettings,
			)
			require.NoError(t, err)

			core, observed := observer.New(zap.NewAtomicLevelAt(zapcore.DebugLevel))
			bi := newSyncBulkIndexer(esClient, &cfg, false, tb, zap.New(core))

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
						attribute.Key("http.response.status_code").Int(http.StatusOK),
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
			}
			if tt.wantMessage != "" {
				attrs = []attribute.KeyValue{
					attribute.StringSlice("x-test", []string{"test"}),
					attribute.String("outcome", "failed_client"),
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

func TestQueryParamsParsedFromEndpoints(t *testing.T) {
	client, err := elasticsearch.NewDefaultClient()
	require.NoError(t, err)
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoints = []string{"http://localhost:9200?pipeline=test-pipeline"}

	bi := bulkIndexerConfig(client, cfg, true, zaptest.NewLogger(t))
	require.Equal(t, map[string][]string{
		"pipeline": {"test-pipeline"},
	}, bi.QueryParams)
}

func TestNewBulkIndexer(t *testing.T) {
	client, err := elasticsearch.NewDefaultClient()
	require.NoError(t, err)
	cfg := createDefaultConfig()

	bi := newBulkIndexer(client, cfg.(*Config), true, nil, nil)
	t.Cleanup(func() { bi.Close(t.Context()) })
}

func TestGetErrorHint(t *testing.T) {
	tests := []struct {
		name      string
		index     string
		errorType string
		want      string
	}{
		{
			name:      "version_conflict_engine_exception with .ds-metrics- prefix",
			index:     ".ds-metrics-foo",
			errorType: "version_conflict_engine_exception",
			want:      errorHintKnownIssues,
		},
		{
			name:      "illegal_argument_exception with .otel- in index (OTel mapping mode)",
			index:     "logs-generic.otel-default",
			errorType: "illegal_argument_exception",
			want:      errorHintOTelMappingMode,
		},
		{
			name:      "illegal_argument_exception with .otel- in metrics index",
			index:     "metrics-generic.otel-default",
			errorType: "illegal_argument_exception",
			want:      errorHintOTelMappingMode,
		},
		{
			name:      "illegal_argument_exception with .otel- in traces index",
			index:     "traces-generic.otel-default",
			errorType: "illegal_argument_exception",
			want:      errorHintOTelMappingMode,
		},
		{
			name:      "illegal_argument_exception without .otel- (not OTel mapping mode)",
			index:     "logs-generic-default",
			errorType: "illegal_argument_exception",
			want:      "",
		},
		{
			name:      "illegal_argument_exception with .otel but not as suffix (should not match)",
			index:     "logs-generic.oteldefault",
			errorType: "illegal_argument_exception",
			want:      "",
		},
		{
			name:      "other error type with .otel-",
			index:     "logs-generic.otel-default",
			errorType: "mapper_parsing_exception",
			want:      "",
		},
		{
			name:      "version_conflict_engine_exception without .ds-metrics- prefix",
			index:     "logs-foo",
			errorType: "version_conflict_engine_exception",
			want:      "",
		},
		{
			name:      "empty index and error type",
			index:     "",
			errorType: "",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getErrorHint(tt.index, tt.errorType)
			assert.Equal(t, tt.want, got)
		})
	}
}
