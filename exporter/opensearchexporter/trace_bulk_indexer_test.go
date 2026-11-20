// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func TestTraceJoinedError(t *testing.T) {
	tests := []struct {
		name     string
		errs     []error
		hasError bool
	}{
		{"no errors", nil, false},
		{"single error", []error{errors.New("test")}, true},
		{"multiple errors", []error{errors.New("err1"), errors.New("err2")}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tbi := &traceBulkIndexer{errs: tt.errs}
			err := tbi.joinedError()
			if (err != nil) != tt.hasError {
				t.Errorf("joinedError() = %v, expected error: %v", err, tt.hasError)
			}
		})
	}
}

func TestTraceProcessItemFailure(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		itemErr       error
		wantPermanent bool
	}{
		{"retry status", 500, nil, false},
		{"permanent status", 400, nil, true},
		{"no status", 0, errors.New("encoding error"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tbi := &traceBulkIndexer{}
			resp := opensearchapi.BulkRespItem{Status: tt.status}
			traces := ptrace.NewTraces()
			tbi.processItemFailure(resp, tt.itemErr, traces)
			err := tbi.joinedError()
			require.Error(t, err)
			require.Equal(t, tt.wantPermanent, consumererror.IsPermanent(err))
		})
	}
}

func TestNewTraceBulkIndexerWithPipeline(t *testing.T) {
	tests := []struct {
		name     string
		pipeline string
	}{
		{"empty pipeline", ""},
		{"with pipeline", "my-pipeline"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tbi := newTraceBulkIndexer("create", nil, tt.pipeline)
			if tbi.pipeline != tt.pipeline {
				t.Errorf("expected pipeline %q, got %q", tt.pipeline, tbi.pipeline)
			}
			if tbi.bulkAction != "create" {
				t.Errorf("expected bulkAction 'create', got %s", tbi.bulkAction)
			}
		})
	}
}

func TestTraceNewBulkIndexerItem(t *testing.T) {
	tbi := &traceBulkIndexer{bulkAction: "create"}
	payload := []byte(`{"test": "data"}`)
	indexName := "test-index"
	item := tbi.newBulkIndexerItem(payload, indexName)

	if item.Action != "create" {
		t.Errorf("expected action 'create', got %s", item.Action)
	}
	if item.Index != indexName {
		t.Errorf("expected index %s, got %s", indexName, item.Index)
	}
	if item.Body == nil {
		t.Error("expected body to be set")
	}
}

func TestMakeTrace(t *testing.T) {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("service.name", "test-service")
	scope := pcommon.NewInstrumentationScope()
	scope.SetName("test-scope")
	span := ptrace.NewSpan()
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	traces := makeTrace(resource, "resource-schema", scope, "scope-schema", span)

	if traces.ResourceSpans().Len() != 1 {
		t.Error("expected 1 resource span")
	}
	rs := traces.ResourceSpans().At(0)
	if rs.SchemaUrl() != "resource-schema" {
		t.Errorf("expected schema 'resource-schema', got %s", rs.SchemaUrl())
	}
	if rs.ScopeSpans().Len() != 1 {
		t.Error("expected 1 scope span")
	}
	ss := rs.ScopeSpans().At(0)
	if ss.SchemaUrl() != "scope-schema" {
		t.Errorf("expected schema 'scope-schema', got %s", ss.SchemaUrl())
	}
	if ss.Spans().Len() != 1 {
		t.Error("expected 1 span")
	}
}

func bulkRespItem(index string, status int) map[string]map[string]any {
	entry := map[string]any{
		"_index": index,
		"status": status,
	}
	switch status {
	case http.StatusCreated:
		entry["result"] = "created"
	case http.StatusBadRequest:
		entry["error"] = map[string]any{
			"type":   "mapper_parsing_exception",
			"reason": "failed to parse field",
		}
	default:
		entry["error"] = map[string]any{
			"type":   "cluster_block_exception",
			"reason": "too many requests",
		}
	}
	return map[string]map[string]any{"create": entry}
}

func newMockBulkServer(t *testing.T, index string, statuses ...int) *httptest.Server {
	t.Helper()
	items := make([]map[string]map[string]any, 0, len(statuses))
	for _, status := range statuses {
		items = append(items, bulkRespItem(index, status))
	}
	payload, err := json.Marshal(map[string]any{"errors": true, "items": items})
	require.NoError(t, err)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
}

func newTestTraces(count int) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("test-scope")
	now := pcommon.NewTimestampFromTime(time.Now())
	for i := range count {
		span := ss.Spans().AppendEmpty()
		span.SetName(fmt.Sprintf("span-%d", i))
		span.SetTraceID(pcommon.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		span.SetSpanID(pcommon.SpanID{1, 2, 3, 4, 5, 6, 7, byte(i + 1)})
		span.SetStartTimestamp(now)
		span.SetEndTimestamp(now)
	}
	return td
}

func collectSpanNames(td ptrace.Traces) []string {
	var names []string
	for i := range td.ResourceSpans().Len() {
		names = appendScopeSpanNames(names, td.ResourceSpans().At(i).ScopeSpans())
	}
	return names
}

func appendScopeSpanNames(names []string, sss ptrace.ScopeSpansSlice) []string {
	for j := range sss.Len() {
		spans := sss.At(j).Spans()
		for k := range spans.Len() {
			names = append(names, spans.At(k).Name())
		}
	}
	return names
}

func assertRetrySpanNames(t *testing.T, err error, wantNames []string) {
	t.Helper()
	if len(wantNames) == 0 {
		return
	}
	var retryErr consumererror.Traces
	require.ErrorAs(t, err, &retryErr)
	require.Equal(t, len(wantNames), retryErr.Data().SpanCount())
	require.Equal(t, wantNames, collectSpanNames(retryErr.Data()))
}

func TestTraceBulkIndexerPartialBulkFailureRetry(t *testing.T) {
	tests := []struct {
		name          string
		statuses      []int
		wantPermanent bool
		wantNames     []string
	}{
		{
			name:      "multiple retryable item failures",
			statuses:  []int{http.StatusCreated, http.StatusTooManyRequests, http.StatusServiceUnavailable},
			wantNames: []string{"span-1", "span-2"},
		},
		{
			name:      "mixed permanent and retryable item failures",
			statuses:  []int{http.StatusBadRequest, http.StatusTooManyRequests, http.StatusServiceUnavailable},
			wantNames: []string{"span-1", "span-2"},
		},
		{
			name:          "only permanent item failures",
			statuses:      []int{http.StatusCreated, http.StatusBadRequest},
			wantPermanent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newMockBulkServer(t, "ss4o_traces-default-namespace", tt.statuses...)
			defer ts.Close()

			client, err := newOpenSearchClient(ts.URL, ts.Client(), zap.NewNop())
			require.NoError(t, err)

			cfg := newDefaultConfig().(*Config)
			model := &encodeModel{sso: true, dataset: cfg.Dataset, namespace: cfg.Namespace}
			ir := newIndexResolver("ss4o_traces", cfg.Dataset, cfg.Namespace)

			tbi := newTraceBulkIndexer(cfg.BulkAction, model, cfg.Pipeline)
			require.NoError(t, tbi.start(client))

			tbi.submit(t.Context(), newTestTraces(len(tt.statuses)), ir, cfg, time.Now())
			tbi.close(t.Context())

			joined := tbi.joinedError()
			require.Error(t, joined)
			require.Equal(t, tt.wantPermanent, consumererror.IsPermanent(joined))
			assertRetrySpanNames(t, joined, tt.wantNames)
		})
	}
}
