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
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func newTestTraceBulkIndexer() *traceBulkIndexer {
	return &traceBulkIndexer{errs: nil}
}

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
			tbi := &traceBulkIndexer{errs: make([]error, tt.initialErrs)}
			resp := opensearchapi.BulkRespItem{Status: tt.status}
			traces := ptrace.NewTraces()
			tbi.processItemFailure(resp, nil, traces)
			if len(tbi.errs) != tt.expectedErrs {
				t.Errorf("expected %d errors, got %d", tt.expectedErrs, len(tbi.errs))
			}
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

func TestTraceOnIndexerError_ConnectionRefused_MustBeRetryable(t *testing.T) {
	tbi := newTestTraceBulkIndexer()

	netErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: errors.New("connection refused"),
	}

	tbi.onIndexerError(t.Context(), netErr)

	require.Len(t, tbi.errs, 1)
	assert.False(t, consumererror.IsPermanent(tbi.errs[0]),
		"connection refused is transient — must NOT be Permanent; exporterhelper should retry")
}

func TestTraceOnIndexerError_ContextDeadlineExceeded_MustBeRetryable(t *testing.T) {
	tbi := newTestTraceBulkIndexer()

	tbi.onIndexerError(t.Context(), context.DeadlineExceeded)

	require.Len(t, tbi.errs, 1)
	assert.False(t, consumererror.IsPermanent(tbi.errs[0]),
		"deadline exceeded is transient — must NOT be Permanent; exporterhelper should retry")
}

func TestTraceOnIndexerError_DNSFailure_MustBeRetryable(t *testing.T) {
	tbi := newTestTraceBulkIndexer()

	dnsErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: &net.DNSError{Err: "no such host", Name: "opensearch.example.com"},
	}

	tbi.onIndexerError(t.Context(), dnsErr)

	require.Len(t, tbi.errs, 1)
	assert.False(t, consumererror.IsPermanent(tbi.errs[0]),
		"DNS failure is transient — must NOT be Permanent; exporterhelper should retry")
}

func TestTraceOnIndexerError_GenericNetworkError_MustBeRetryable(t *testing.T) {
	tbi := newTestTraceBulkIndexer()

	tbi.onIndexerError(t.Context(), errors.New("EOF"))

	require.Len(t, tbi.errs, 1)
	assert.False(t, consumererror.IsPermanent(tbi.errs[0]),
		"generic network-level error from bulk flush must NOT be Permanent")
}

func TestTraceOnIndexerError_RetryableError_DoesNotWrapAsNewPermanent(t *testing.T) {
	tbi := newTestTraceBulkIndexer()

	tbi.onIndexerError(t.Context(), context.DeadlineExceeded)

	require.Len(t, tbi.errs, 1)
	assert.False(t, consumererror.IsPermanent(tbi.errs[0]),
		"onIndexerError must never produce a Permanent error — doing so "+
			"causes exporterhelper to bypass the retry queue and silently drop traces")
}

func TestTraceProcessItemFailure_Status0_NetOpError_MustBeRetryable(t *testing.T) {
	tbi := newTestTraceBulkIndexer()

	netErr := &net.OpError{
		Op:  "read",
		Net: "tcp",
		Err: errors.New("connection reset by peer"),
	}

	tbi.processItemFailure(opensearchapi.BulkRespItem{Status: 0}, netErr, ptrace.NewTraces())

	require.Len(t, tbi.errs, 1)
	assert.False(t, consumererror.IsPermanent(tbi.errs[0]),
		"status=0 + net.OpError is a transport failure — must NOT be Permanent")
}
