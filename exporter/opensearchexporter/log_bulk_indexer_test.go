// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

func TestJoinedError(t *testing.T) {
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
			lbi := &logBulkIndexer{errs: tt.errs}
			err := lbi.joinedError()
			if (err != nil) != tt.hasError {
				t.Errorf("joinedError() = %v, expected error: %v", err, tt.hasError)
			}
		})
	}
}

func TestProcessItemFailure(t *testing.T) {
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
			lbi := &logBulkIndexer{}
			resp := opensearchapi.BulkRespItem{Status: tt.status}
			logs := plog.NewLogs()
			lbi.processItemFailure(resp, tt.itemErr, logs)
			err := lbi.joinedError()
			require.Error(t, err)
			require.Equal(t, tt.wantPermanent, consumererror.IsPermanent(err))
		})
	}
}

func TestNewLogBulkIndexerWithPipeline(t *testing.T) {
	tests := []struct {
		name     string
		pipeline string
	}{
		{"empty pipeline", ""},
		{"with pipeline", "my-pipeline"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lbi := newLogBulkIndexer("create", nil, tt.pipeline)
			if lbi.pipeline != tt.pipeline {
				t.Errorf("expected pipeline %q, got %q", tt.pipeline, lbi.pipeline)
			}
			if lbi.bulkAction != "create" {
				t.Errorf("expected bulkAction 'create', got %s", lbi.bulkAction)
			}
		})
	}
}

func TestNewBulkIndexerItem(t *testing.T) {
	lbi := &logBulkIndexer{bulkAction: "index"}
	payload := []byte(`{"test": "data"}`)
	indexName := "test-index"
	item := lbi.newBulkIndexerItem(payload, indexName)

	if item.Action != "index" {
		t.Errorf("expected action 'index', got %s", item.Action)
	}
	if item.Index != indexName {
		t.Errorf("expected index %s, got %s", indexName, item.Index)
	}
	if item.Body == nil {
		t.Error("expected body to be set")
	}
}

func TestMakeLog(t *testing.T) {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("service.name", "test-service")
	scope := pcommon.NewInstrumentationScope()
	scope.SetName("test-scope")
	logRecord := plog.NewLogRecord()
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	logs := makeLog(resource, "resource-schema", scope, "scope-schema", logRecord)

	if logs.ResourceLogs().Len() != 1 {
		t.Error("expected 1 resource log")
	}
	rl := logs.ResourceLogs().At(0)
	if rl.SchemaUrl() != "resource-schema" {
		t.Errorf("expected schema 'resource-schema', got %s", rl.SchemaUrl())
	}
	if rl.ScopeLogs().Len() != 1 {
		t.Error("expected 1 scope log")
	}
	sl := rl.ScopeLogs().At(0)
	if sl.SchemaUrl() != "scope-schema" {
		t.Errorf("expected schema 'scope-schema', got %s", sl.SchemaUrl())
	}
	if sl.LogRecords().Len() != 1 {
		t.Error("expected 1 log record")
	}
}

func newTestLogs(count int) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test-service")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("test-scope")
	for i := range count {
		lr := sl.LogRecords().AppendEmpty()
		lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		lr.Body().SetStr(fmt.Sprintf("log-%d", i))
	}
	return ld
}

func collectLogBodies(ld plog.Logs) []string {
	var bodies []string
	for i := range ld.ResourceLogs().Len() {
		bodies = appendScopeLogBodies(bodies, ld.ResourceLogs().At(i).ScopeLogs())
	}
	return bodies
}

func appendScopeLogBodies(bodies []string, sls plog.ScopeLogsSlice) []string {
	for j := range sls.Len() {
		lrs := sls.At(j).LogRecords()
		for k := range lrs.Len() {
			bodies = append(bodies, lrs.At(k).Body().AsString())
		}
	}
	return bodies
}

func assertRetryLogBodies(t *testing.T, err error, wantBodies []string) {
	t.Helper()
	if len(wantBodies) == 0 {
		return
	}
	var retryErr consumererror.Logs
	require.ErrorAs(t, err, &retryErr)
	require.Equal(t, len(wantBodies), retryErr.Data().LogRecordCount())
	require.Equal(t, wantBodies, collectLogBodies(retryErr.Data()))
}

func TestLogBulkIndexerPartialBulkFailureRetry(t *testing.T) {
	tests := []struct {
		name          string
		statuses      []int
		wantPermanent bool
		wantBodies    []string
	}{
		{
			name:       "multiple retryable item failures",
			statuses:   []int{http.StatusCreated, http.StatusTooManyRequests, http.StatusServiceUnavailable},
			wantBodies: []string{"log-1", "log-2"},
		},
		{
			name:       "mixed permanent and retryable item failures",
			statuses:   []int{http.StatusBadRequest, http.StatusTooManyRequests, http.StatusServiceUnavailable},
			wantBodies: []string{"log-1", "log-2"},
		},
		{
			name:          "only permanent item failures",
			statuses:      []int{http.StatusCreated, http.StatusBadRequest},
			wantPermanent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newMockBulkServer(t, "ss4o_logs-default-namespace", tt.statuses...)
			defer ts.Close()

			client, err := newOpenSearchClient(ts.URL, ts.Client(), zap.NewNop())
			require.NoError(t, err)

			cfg := newDefaultConfig().(*Config)
			model := &encodeModel{sso: true, dataset: cfg.Dataset, namespace: cfg.Namespace}
			ir := newIndexResolver("ss4o_logs", cfg.Dataset, cfg.Namespace)

			lbi := newLogBulkIndexer(cfg.BulkAction, model, cfg.Pipeline)
			require.NoError(t, lbi.start(client))

			lbi.submit(t.Context(), newTestLogs(len(tt.statuses)), ir, cfg, time.Now())
			lbi.close(t.Context())

			joined := lbi.joinedError()
			require.Error(t, joined)
			require.Equal(t, tt.wantPermanent, consumererror.IsPermanent(joined))
			assertRetryLogBodies(t, joined, tt.wantBodies)
		})
	}
}
