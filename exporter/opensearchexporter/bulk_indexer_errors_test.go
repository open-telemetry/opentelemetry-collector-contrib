// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter/internal/metadata"
)

const mapperParsingError = `{"type":"mapper_parsing_exception","reason":"failed to parse field [x]"}`

// bulkReply is how the test server answers one bulk request. If status is not
// 200 the whole request is rejected with body; otherwise itemStatus gives the
// status of each document, keyed by its log body.
type bulkReply struct {
	status     int
	body       string
	itemStatus map[string]int
}

func TestOpenSearchLogExporterRetriesRejectedRequests(t *testing.T) {
	all := []string{"doc-0", "doc-1", "doc-2", "doc-3"}
	tests := []struct {
		name          string
		replies       []bulkReply
		wantRequests  [][]string
		wantPermanent bool
	}{
		{
			name: "every item rejected with 429 is retried",
			replies: []bulkReply{
				{status: http.StatusOK, itemStatus: map[string]int{"doc-1": 429, "doc-2": 429, "doc-3": 503}},
				{status: http.StatusOK},
			},
			wantRequests: [][]string{all, {"doc-1", "doc-2", "doc-3"}},
		},
		{
			name: "429 items are retried when other items fail permanently",
			replies: []bulkReply{
				{status: http.StatusOK, itemStatus: map[string]int{"doc-1": 429, "doc-2": 400, "doc-3": 429}},
				{status: http.StatusOK},
			},
			wantRequests: [][]string{all, {"doc-1", "doc-3"}},
		},
		{
			name: "whole request rejected with 429 by OpenSearch",
			replies: []bulkReply{
				{status: http.StatusTooManyRequests, body: `{"error":{"type":"rejected_execution_exception","reason":"rejected execution"},"status":429}`},
				{status: http.StatusOK},
			},
			wantRequests: [][]string{all, all},
		},
		{
			name: "whole request rejected with 503 by a proxy",
			replies: []bulkReply{
				{status: http.StatusServiceUnavailable, body: "<html><body><h1>503 Service Temporarily Unavailable</h1></body></html>"},
				{status: http.StatusOK},
			},
			wantRequests: [][]string{all, all},
		},
		{
			name: "whole request rejected with 504 and no body",
			replies: []bulkReply{
				{status: http.StatusGatewayTimeout},
				{status: http.StatusOK},
			},
			wantRequests: [][]string{all, all},
		},
		{
			name: "whole request rejected with 400 is not retried",
			replies: []bulkReply{
				{status: http.StatusBadRequest, body: `{"error":{"type":"illegal_argument_exception","reason":"bad request"},"status":400}`},
			},
			wantRequests:  [][]string{all},
			wantPermanent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			var requests [][]string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				docs := readBulkDocs(t, r)
				mu.Lock()
				attempt := len(requests)
				requests = append(requests, docs)
				mu.Unlock()
				if !assert.Less(t, attempt, len(tt.replies), "unexpected bulk request %v", docs) {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				writeBulkReply(t, w, tt.replies[attempt], docs)
			}))
			defer ts.Close()

			cfg := withDefaultConfig(func(config *Config) {
				config.QueueConfig = configoptional.None[exporterhelper.QueueBatchConfig]()
				config.ClientConfig.Endpoint = ts.URL
				config.TimeoutSettings.Timeout = 0
				config.BackOffConfig.InitialInterval = 10 * time.Millisecond
				config.BackOffConfig.MaxInterval = 10 * time.Millisecond
				config.BackOffConfig.MaxElapsedTime = 10 * time.Second
			})
			exp, err := NewFactory().CreateLogs(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
			require.NoError(t, err)
			require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))
			defer func() { require.NoError(t, exp.Shutdown(t.Context())) }()

			logs := plog.NewLogs()
			records := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			for _, body := range all {
				records.AppendEmpty().Body().SetStr(body)
			}

			err = exp.ConsumeLogs(t.Context(), logs)
			if tt.wantPermanent {
				require.True(t, consumererror.IsPermanent(err), "want permanent error, got %v", err)
			} else {
				require.NoError(t, err)
			}
			mu.Lock()
			defer mu.Unlock()
			assert.Equal(t, tt.wantRequests, requests)
		})
	}
}

var docMarker = regexp.MustCompile(`doc-\d+`)

// readBulkDocs returns the doc-N marker of every document in a bulk request.
func readBulkDocs(t *testing.T, r *http.Request) []string {
	var docs []string
	scanner := bufio.NewScanner(r.Body)
	for i := 0; scanner.Scan(); i++ {
		if i%2 == 1 { // even lines are the action metadata
			docs = append(docs, docMarker.FindString(scanner.Text()))
		}
	}
	assert.NoError(t, scanner.Err())
	return docs
}

func writeBulkReply(t *testing.T, w http.ResponseWriter, reply bulkReply, docs []string) {
	if reply.status != http.StatusOK {
		w.WriteHeader(reply.status)
		_, err := w.Write([]byte(reply.body))
		assert.NoError(t, err)
		return
	}
	items := make([]string, len(docs))
	for i, doc := range docs {
		status := reply.itemStatus[doc]
		switch {
		case status == 0:
			items[i] = `{"create":{"_index":"logs","status":201,"result":"created"}}`
		case shouldRetryEvent(status):
			items[i] = fmt.Sprintf(`{"create":{"_index":"logs","status":%d,"error":{"type":"es_rejected_execution_exception","reason":"rejected"}}}`, status)
		default:
			items[i] = fmt.Sprintf(`{"create":{"_index":"logs","status":%d,"error":%s}}`, status, mapperParsingError)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_, err := fmt.Fprintf(w, `{"took":1,"errors":%t,"items":[%s]}`, len(reply.itemStatus) > 0, strings.Join(items, ","))
	assert.NoError(t, err)
}

// mappingFailure is a bulk response item for a document OpenSearch cannot index.
func mappingFailure(t *testing.T) opensearchapi.BulkRespItem {
	var item opensearchapi.BulkRespItem
	require.NoError(t, json.Unmarshal([]byte(`{"status":400,"error":`+mapperParsingError+`}`), &item))
	return item
}

func TestTraceBulkIndexerRetriesAllRejectedSpans(t *testing.T) {
	tbi := &traceBulkIndexer{}
	for range 3 {
		traces := ptrace.NewTraces()
		traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		tbi.processItemFailure(opensearchapi.BulkRespItem{Status: 429}, nil, traces)
	}
	tbi.processItemFailure(mappingFailure(t), nil, ptrace.NewTraces())

	err := tbi.joinedError()
	require.False(t, consumererror.IsPermanent(err))
	retry, ok := errors.AsType[consumererror.Traces](err)
	require.True(t, ok)
	assert.Equal(t, 3, retry.Data().SpanCount())
	assert.ErrorContains(t, err, "mapper_parsing_exception")
}

func TestMetricBulkIndexerRetriesAllRejectedDataPoints(t *testing.T) {
	mbi := &metricBulkIndexer{}
	for range 3 {
		metrics := pmetric.NewMetrics()
		metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetEmptyGauge().DataPoints().AppendEmpty()
		mbi.processItemFailure(opensearchapi.BulkRespItem{Status: 429}, nil, metrics)
	}
	mbi.processItemFailure(mappingFailure(t), nil, pmetric.NewMetrics())

	err := mbi.joinedError()
	require.False(t, consumererror.IsPermanent(err))
	retry, ok := errors.AsType[consumererror.Metrics](err)
	require.True(t, ok)
	assert.Equal(t, 3, retry.Data().DataPointCount())
	assert.ErrorContains(t, err, "mapper_parsing_exception")
}

func TestIsRetryableResponse(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"429 from OpenSearch", 429, `{"error":{"type":"rejected_execution_exception","reason":"rejected"},"status":429}`, true},
		{"500 from OpenSearch", 500, `{"error":{"type":"exception","reason":"boom"},"status":500}`, true},
		{"503 with string error", 503, `{"error":"unavailable","status":503}`, true},
		{"502 html page from a proxy", 502, "<html>Bad Gateway</html>", true},
		{"504 without body", 504, "", true},
		{"400 from OpenSearch", 400, `{"error":{"type":"illegal_argument_exception","reason":"bad"},"status":400}`, false},
		{"401 with string error", 401, `{"error":"unauthorized","status":401}`, false},
		{"413 json without status field", 413, `{"message":"too large"}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer ts.Close()
			client, err := newOpenSearchClient(ts.URL, http.DefaultClient, zap.NewNop())
			require.NoError(t, err)

			_, err = client.Bulk(t.Context(), opensearchapi.BulkReq{Body: strings.NewReader("{}\n")})
			require.Error(t, err)
			assert.Equal(t, tt.want, isRetryableResponse(fmt.Errorf("flush: %w", err)), "error: %v", err)
		})
	}
}

func TestIsRetryableResponseIgnoresTransportErrors(t *testing.T) {
	assert.False(t, isRetryableResponse(errors.New("json: unsupported value")))
	assert.False(t, isRetryableResponse(fmt.Errorf("flush: %w", json.Unmarshal([]byte("x"), &struct{}{}))))
}

func TestJoinBulkErrors(t *testing.T) {
	requestErr := fmt.Errorf("flush: %w", errors.New("status 429"))
	withData := func(err error) error { return consumererror.NewLogs(err, plog.NewLogs()) }

	t.Run("only permanent errors stay permanent", func(t *testing.T) {
		err := joinBulkErrors([]error{consumererror.NewPermanent(errors.New("bad doc"))}, nil, nil, withData)
		require.True(t, consumererror.IsPermanent(err))
	})
	t.Run("request error retries the whole push", func(t *testing.T) {
		// OnError reports the same failed request again as a permanent error.
		errs := []error{consumererror.NewPermanent(requestErr), consumererror.NewPermanent(errors.New("bad doc"))}
		err := joinBulkErrors(errs, requestErr, nil, withData)
		require.False(t, consumererror.IsPermanent(err))
		_, isLogs := errors.AsType[consumererror.Logs](err)
		require.False(t, isLogs, "whole push must be retried, not a subset")
		assert.Equal(t, 1, strings.Count(err.Error(), "status 429"))
		assert.ErrorContains(t, err, "bad doc")
	})
	t.Run("another failed request retries the whole push", func(t *testing.T) {
		err := joinBulkErrors([]error{errors.New("flush: EOF")}, nil, []error{errors.New("429")}, withData)
		require.False(t, consumererror.IsPermanent(err))
		_, isLogs := errors.AsType[consumererror.Logs](err)
		require.False(t, isLogs, "whole push must be retried, not a subset")
	})
}
