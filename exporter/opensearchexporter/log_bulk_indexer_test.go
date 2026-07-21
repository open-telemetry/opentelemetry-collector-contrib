// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"go.opentelemetry.io/collector/consumer/consumererror"
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

func TestOnIndexerErrorIsRetryable(t *testing.T) {
	lbi := &logBulkIndexer{}
	// A transport failure surfaces through the bulk indexer's OnError callback.
	lbi.onIndexerError(t.Context(), &net.OpError{Op: "dial", Err: errors.New("connection refused")})
	err := lbi.joinedError()
	if err == nil {
		t.Fatal("expected an error")
	}
	if consumererror.IsPermanent(err) {
		t.Error("indexer-level transport error must be retryable, not permanent (otherwise retry_on_failure silently drops the batch)")
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"deadline exceeded", context.DeadlineExceeded, true},
		{"canceled", context.Canceled, true},
		{"net.OpError", &net.OpError{Op: "dial", Err: errors.New("connection refused")}, true},
		{
			"url.Error wrapping net.OpError",
			&url.Error{Op: "Post", URL: "http://localhost", Err: &net.OpError{Op: "dial", Err: errors.New("connection refused")}},
			true,
		},
		{"flush-wrapped deadline", fmt.Errorf("flush: %w", context.DeadlineExceeded), true},
		{"encoding error", errors.New("json: unsupported value"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableError(tt.err); got != tt.want {
				t.Errorf("isRetryableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
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
