// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformer

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
)

func TestSpanEventConditionalAttributeTransformer(t *testing.T) {
	migrator := migrate.NewMultiConditionalAttributeSet(map[string]string{
		"event.name": "event_name",
	}, map[string][]string{
		"span.name":  {"sqlquery.start", "sqlquery.end"},
		"event.name": {"sqlquery"},
	})
	c := SpanEventConditionalAttributes{migrator}
	tests := []struct {
		name    string
		span    func() ptrace.Span
		changed bool
	}{
		{
			name: "nomatch",
			span: func() ptrace.Span {
				span := ptrace.NewSpan()
				span.SetName("nosqlquery.start")
				span.Events().AppendEmpty().SetName("nosqlquery")
				return span
			},
			changed: false,
		},
		{
			name: "spannamematches",
			span: func() ptrace.Span {
				span := ptrace.NewSpan()
				span.SetName("sqlquery.start")
				span.Events().AppendEmpty().SetName("nosqlquery")
				return span
			},
			changed: false,
		},
		{
			name: "eventnamematches",
			span: func() ptrace.Span {
				span := ptrace.NewSpan()
				span.SetName("nosqlquery.start")
				span.Events().AppendEmpty().SetName("sqlquery")
				return span
			},
			changed: false,
		},
		{
			name: "bothnamematches",
			span: func() ptrace.Span {
				span := ptrace.NewSpan()
				span.SetName("sqlquery.start")
				span.Events().AppendEmpty().SetName("sqlquery")
				return span
			},
			changed: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := tt.span()
			span.Events().At(0).Attributes().PutStr("event.name", "blah")
			err := c.Do(migrate.StateSelectorApply, span)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.changed {
				assertAttributeEquals(t, span.Events().At(0).Attributes(), "event_name", "blah")
			} else {
				assertAttributeEquals(t, span.Events().At(0).Attributes(), "event.name", "blah")
			}
		})
	}
}
