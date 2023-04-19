package sampling

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"testing"
)

func TestEvaluate_OTTL(t *testing.T) {
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	cases := []struct {
		Desc               string
		SpanStatement      []string
		SpanEventStatement []string
		Spans              []spanWithAttributes
		Decision           Decision
	}{
		{
			// policy
			"OTTL statement not set",
			[]string{},
			[]string{},
			[]spanWithAttributes{{SpanAttributes: map[string]string{"attr_k_1": "attr_v_1"}}},
			NotSampled,
		},
		{
			"OTTL statement match specific span attributes 1",
			[]string{"attributes[\"attr_k_1\"] == \"attr_v_1\""},
			[]string{},
			[]spanWithAttributes{{SpanAttributes: map[string]string{"attr_k_1": "attr_v_1"}}},
			Sampled,
		},
		{
			"OTTL statement match specific span attributes 2",
			[]string{"attributes[\"attr_k_1\"] != \"attr_v_1\""},
			[]string{},
			[]spanWithAttributes{{SpanAttributes: map[string]string{"attr_k_1": "attr_v_1"}}},
			NotSampled,
		},
		{
			"OTTL statement match specific span event attributes",
			[]string{},
			[]string{"attributes[\"event_attr_k_1\"] == \"event_attr_v_1\""},
			[]spanWithAttributes{{SpanEventAttributes: map[string]string{"event_attr_k_1": "event_attr_v_1"}}},
			Sampled,
		},
		{
			"OTTL statement match specific span event name",
			[]string{},
			[]string{"name != \"incorrect event name\""},
			[]spanWithAttributes{{SpanEventAttributes: nil}},
			Sampled,
		},
		{
			"OTTL statement not matched",
			[]string{"attributes[\"attr_k_1\"] == \"attr_v_1\""},
			[]string{"attributes[\"event_attr_k_1\"] == \"event_attr_v_1\""},
			[]spanWithAttributes{},
			NotSampled,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			filter, _ := NewOTTLStatementFilter(zap.NewNop(), c.SpanStatement, c.SpanEventStatement, ottl.IgnoreError)
			decision, err := filter.Evaluate(context.Background(), traceID, newTraceWithSpansAttributes(c.Spans))

			assert.NoError(t, err)
			assert.Equal(t, decision, c.Decision)
		})
	}
}

type spanWithAttributes struct {
	SpanAttributes      map[string]string
	SpanEventAttributes map[string]string
}

func newTraceWithSpansAttributes(spans []spanWithAttributes) *TraceData {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	for _, s := range spans {
		span := ils.Spans().AppendEmpty()
		span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
		for k, v := range s.SpanAttributes {
			span.Attributes().PutStr(k, v)
		}
		spanEvent := span.Events().AppendEmpty()
		spanEvent.SetName("test event")
		for k, v := range s.SpanEventAttributes {
			spanEvent.Attributes().PutStr(k, v)
		}
	}

	return &TraceData{
		ReceivedBatches: traces,
	}
}
