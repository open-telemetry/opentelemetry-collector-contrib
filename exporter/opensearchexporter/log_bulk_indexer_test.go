// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"errors"
	"testing"
	"time"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
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
		name         string
		status       int
		initialErrs  int
		expectedErrs int
	}{
		{"retry status", 500, 0, 1},
		{"permanent status", 400, 0, 1},
		{"no status", 0, 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lbi := &logBulkIndexer{errs: make([]error, tt.initialErrs)}
			resp := opensearchapi.BulkRespItem{Status: tt.status}
			logs := plog.NewLogs()
			lbi.processItemFailure(resp, nil, logs)
			if len(lbi.errs) != tt.expectedErrs {
				t.Errorf("expected %d errors, got %d", tt.expectedErrs, len(lbi.errs))
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

func TestForEachLog(t *testing.T) {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test-service")
	sl := rl.ScopeLogs().AppendEmpty()
	log := sl.LogRecords().AppendEmpty()
	log.Attributes().PutStr("key", "value")

	resolver := newIndexResolver()
	cfg := &Config{LogsIndex: "test-%{service.name}", LogsIndexFallback: "", LogsIndexTimeFormat: ""}
	ts := time.Now()
	keys := resolver.extractPlaceholderKeys(cfg.LogsIndex)
	timeSuffix := resolver.calculateTimeSuffix(cfg.LogsIndexTimeFormat, ts)

	var results []string
	forEachLog(ld, resolver, cfg, timeSuffix, keys, func(_ pcommon.Resource, _ string, _ pcommon.InstrumentationScope, _ string, _ plog.LogRecord, indexName string) {
		results = append(results, indexName)
	})

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	expected := "test-test-service"
	if results[0] != expected {
		t.Errorf("expected index %s, got %s", expected, results[0])
	}
}
