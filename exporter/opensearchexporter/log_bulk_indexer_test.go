// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestLogBulkIndexer() *logBulkIndexer {
	return &logBulkIndexer{errs: nil}
}

func isRetryableError(err error) bool {
	return !consumererror.IsPermanent(err)
}

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

func TestOnIndexerError_ConnectionRefused_MustBeRetryable(t *testing.T) {
	lbi := newTestLogBulkIndexer()

	netErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: errors.New("connection refused"),
	}

	lbi.onIndexerError(context.Background(), netErr)

	require.Len(t, lbi.errs, 1)
	assert.True(t, isRetryableError(lbi.errs[0]),
		"connection refused is transient — must NOT be Permanent; exporterhelper should retry")
}

func TestOnIndexerError_ContextDeadlineExceeded_MustBeRetryable(t *testing.T) {
	lbi := newTestLogBulkIndexer()

	lbi.onIndexerError(context.Background(), context.DeadlineExceeded)

	require.Len(t, lbi.errs, 1)
	assert.True(t, isRetryableError(lbi.errs[0]),
		"deadline exceeded is transient — must NOT be Permanent; exporterhelper should retry")
}

func TestOnIndexerError_DNSFailure_MustBeRetryable(t *testing.T) {
	lbi := newTestLogBulkIndexer()

	dnsErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: &net.DNSError{Err: "no such host", Name: "opensearch.example.com"},
	}

	lbi.onIndexerError(context.Background(), dnsErr)

	require.Len(t, lbi.errs, 1)
	assert.True(t, isRetryableError(lbi.errs[0]),
		"DNS failure is transient — must NOT be Permanent; exporterhelper should retry")
}

func TestOnIndexerError_GenericNetworkError_MustBeRetryable(t *testing.T) {
	lbi := newTestLogBulkIndexer()

	lbi.onIndexerError(context.Background(), errors.New("EOF"))

	require.Len(t, lbi.errs, 1)
	assert.True(t, isRetryableError(lbi.errs[0]),
		"generic network-level error from bulk flush must NOT be Permanent")
}

func TestOnIndexerError_RetryableError_DoesNotWrapAsNewPermanent(t *testing.T) {
	lbi := newTestLogBulkIndexer()

	lbi.onIndexerError(context.Background(), context.DeadlineExceeded)

	require.Len(t, lbi.errs, 1)
	assert.False(t, consumererror.IsPermanent(lbi.errs[0]),
		"onIndexerError must never produce a Permanent error — doing so "+
			"causes exporterhelper to bypass the retry queue and silently drop logs")
}

func TestProcessItemFailure_Status0_NetOpError_MustBeRetryable(t *testing.T) {
	lbi := newTestLogBulkIndexer()

	netErr := &net.OpError{
		Op:  "read",
		Net: "tcp",
		Err: errors.New("connection reset by peer"),
	}

	lbi.processItemFailure(opensearchapi.BulkRespItem{Status: 0}, netErr, plog.NewLogs())

	require.Len(t, lbi.errs, 1)
	assert.True(t, isRetryableError(lbi.errs[0]),
		"status=0 + net.OpError is a transport failure — must NOT be Permanent")
}
