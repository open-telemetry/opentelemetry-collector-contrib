// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common_test // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/common_test"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/common"
)

var (
	TestSpanStartTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	TestSpanStartTimestamp = pcommon.NewTimestampFromTime(TestSpanStartTime)

	TestSpanEndTime      = time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC)
	TestSpanEndTimestamp = pcommon.NewTimestampFromTime(TestSpanEndTime)

	spanID2 = [8]byte{8, 7, 6, 5, 4, 3, 2, 1}
)

func TestFilterTraceProcessorWithOTTL(t *testing.T) {
	tests := []struct {
		name             string
		conditions       []string
		filterEverything bool
		want             func(td ptrace.Traces)
		wantErr          bool
		errorMode        ottl.ErrorMode
	}{
		{
			name: "drop spans",
			conditions: []string{
				`span.name == "operationA"`,
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().RemoveIf(func(span ptrace.Span) bool {
					return span.Name() == "operationA"
				})
			},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "drop everything by dropping all spans",
			conditions: []string{
				`IsMatch(span.name, "operation.*")`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "drop span events",
			conditions: []string{
				`spanevent.name == "spanEventA"`,
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Events().RemoveIf(func(event ptrace.SpanEvent) bool {
					return event.Name() == "spanEventA"
				})
			},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "multiple conditions",
			conditions: []string{
				`span.name == "operationZ"`,
				`span.span_id != nil`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "filters resource",
			conditions: []string{
				`resource.schema_url == "test_schema_url"`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "with error conditions",
			conditions: []string{
				`Substring("", 0, 100) == "test"`,
			},
			want:      func(_ ptrace.Traces) {},
			wantErr:   true,
			errorMode: ottl.IgnoreError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collection, err := common.NewTraceParserCollection(component.TelemetrySettings{Logger: zap.NewNop()}, common.WithSpanParser(filterottl.StandardSpanFuncs()), common.WithSpanEventParser(filterottl.StandardSpanEventFuncs()))
			assert.NoError(t, err)
			got, err := collection.ParseContextConditions(common.ContextConditions{Conditions: tt.conditions, ErrorMode: tt.errorMode})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err, "error parsing conditions")
			finalTraces := constructTraces()
			consumeErr := got.ConsumeTraces(context.Background(), finalTraces)
			if tt.filterEverything && !tt.wantErr {
				assert.Equal(t, processorhelper.ErrSkipProcessingData, consumeErr)
			} else if tt.wantErr {
				assert.Error(t, consumeErr)
			} else {
				assert.NoError(t, consumeErr)
				exTd := constructTraces()
				tt.want(exTd)
				assert.Equal(t, exTd, finalTraces)
			}
		})
	}
}

func constructTraces() ptrace.Traces {
	td := ptrace.NewTraces()
	rs0 := td.ResourceSpans().AppendEmpty()
	rs0.SetSchemaUrl("test_schema_url")
	rs0.Resource().Attributes().PutStr("host.name", "localhost")
	rs0ils0 := rs0.ScopeSpans().AppendEmpty()
	rs0ils0.SetSchemaUrl("test_schema_url")
	rs0ils0.Scope().SetName("scope")
	fillSpanOne(rs0ils0.Spans().AppendEmpty())
	fillSpanTwo(rs0ils0.Spans().AppendEmpty())
	return td
}

func fillSpanOne(span ptrace.Span) {
	span.SetName("operationA")
	span.SetSpanID(spanID)
	span.SetParentSpanID(spanID2)
	span.SetTraceID(traceID)
	span.SetStartTimestamp(TestSpanStartTimestamp)
	span.SetEndTimestamp(TestSpanEndTimestamp)
	span.SetDroppedAttributesCount(1)
	span.SetDroppedLinksCount(1)
	span.SetDroppedEventsCount(1)
	span.SetKind(1)
	span.TraceState().FromRaw("new")
	span.Attributes().PutStr("http.method", "get")
	span.Attributes().PutStr("http.path", "/health")
	span.Attributes().PutStr("http.url", "http://localhost/health")
	span.Attributes().PutStr("flags", "A|B|C")
	span.Attributes().PutStr("total.string", "123456789")
	status := span.Status()
	status.SetCode(ptrace.StatusCodeError)
	status.SetMessage("status-cancelled")
	event := span.Events().AppendEmpty()
	event.SetName("eventA")
}

func fillSpanTwo(span ptrace.Span) {
	span.SetName("operationB")
	span.SetStartTimestamp(TestSpanStartTimestamp)
	span.SetEndTimestamp(TestSpanEndTimestamp)
	span.Attributes().PutStr("http.method", "get")
	span.Attributes().PutStr("http.path", "/health")
	span.Attributes().PutStr("http.url", "http://localhost/health")
	span.Attributes().PutStr("flags", "C|D")
	span.Attributes().PutStr("total.string", "345678")
	link0 := span.Links().AppendEmpty()
	link0.SetDroppedAttributesCount(4)
	link1 := span.Links().AppendEmpty()
	link1.SetDroppedAttributesCount(4)
	span.SetDroppedLinksCount(3)
	status := span.Status()
	status.SetCode(ptrace.StatusCodeError)
	status.SetMessage("status-cancelled")
	event := span.Events().AppendEmpty()
	event.SetName("eventB")
}
