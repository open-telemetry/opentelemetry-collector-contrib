// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package schemaprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestTraces_SpanRenameAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		in              ptrace.Traces
		out             ptrace.Traces
		transformations string
		targetVersion   string
	}{
		{
			name: "one_version_downgrade",
			in: func() ptrace.Traces {
				in := ptrace.NewTraces()
				in.ResourceSpans().AppendEmpty()
				in.ResourceSpans().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.9.0")
				in.ResourceSpans().At(0).ScopeSpans().AppendEmpty()
				s := in.ResourceSpans().At(0).ScopeSpans().At(0).Spans().AppendEmpty()
				s.SetName("http.request")
				s.Attributes().PutStr("new.attr.name", "test-cluster")
				s.SetKind(ptrace.SpanKindConsumer)
				s.CopyTo(in.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0))
				return in
			}(),
			out: func() ptrace.Traces {
				out := ptrace.NewTraces()
				out.ResourceSpans().AppendEmpty()
				out.ResourceSpans().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.8.0")
				out.ResourceSpans().At(0).ScopeSpans().AppendEmpty()
				s := out.ResourceSpans().At(0).ScopeSpans().At(0).Spans().AppendEmpty()
				out.ResourceSpans().At(0).ScopeSpans().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.8.0")
				s.SetName("http.request")
				s.Attributes().PutStr("old.attr.name", "test-cluster")

				s.SetKind(ptrace.SpanKindConsumer)
				s.CopyTo(out.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0))
				return out
			}(),
			transformations: `
  1.9.0:
    spans:
      changes:
        - rename_attributes:
            attribute_map:
              old.attr.name: new.attr.name
  1.8.0:`,
			targetVersion: "1.8.0",
		},
		{
			name: "one_version_upgrade",
			in: func() ptrace.Traces {
				in := ptrace.NewTraces()
				in.ResourceSpans().AppendEmpty()
				in.ResourceSpans().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.8.0")
				in.ResourceSpans().At(0).ScopeSpans().AppendEmpty()
				s := in.ResourceSpans().At(0).ScopeSpans().At(0).Spans().AppendEmpty()
				s.SetName("http.request")
				s.Attributes().PutStr("old.attr.name", "test-cluster")
				s.SetKind(ptrace.SpanKindConsumer)
				s.CopyTo(in.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0))
				return in
			}(),
			out: func() ptrace.Traces {
				out := ptrace.NewTraces()
				out.ResourceSpans().AppendEmpty()
				out.ResourceSpans().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.9.0")
				out.ResourceSpans().At(0).ScopeSpans().AppendEmpty()
				s := out.ResourceSpans().At(0).ScopeSpans().At(0).Spans().AppendEmpty()
				out.ResourceSpans().At(0).ScopeSpans().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.9.0")
				s.SetName("http.request")
				s.Attributes().PutStr("new.attr.name", "test-cluster")

				s.SetKind(ptrace.SpanKindConsumer)
				s.CopyTo(out.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0))
				return out
			}(),
			transformations: `
  1.9.0:
    spans:
     changes:
       - rename_attributes:
          attribute_map:
           old.attr.name: new.attr.name
  1.8.0:`,
			targetVersion: "1.9.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := newTestSchemaProcessor(t, tt.transformations, tt.targetVersion)
			ctx := context.Background()
			out, err := pr.processTraces(ctx, tt.in)
			if err != nil {
				t.Errorf("Error while processing traces: %v", err)
			}
			assert.Equal(t, tt.out, out, "Traces transformation failed")
		})
	}
}
