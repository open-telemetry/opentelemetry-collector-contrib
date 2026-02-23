// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"errors"
	"testing"
	"time"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
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
