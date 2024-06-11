// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package objmodel

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestSpans(t *testing.T) {
	key := "test"
	ts := pcommon.NewTimestampFromTime(time.Now())
	for _, tc := range []struct {
		name        string
		s           ptrace.SpanEventSlice
		expectedLen int
		expectedDoc Document
	}{
		{
			name:        "empty",
			s:           ptrace.NewSpanEventSlice(),
			expectedDoc: Document{},
		},
		{
			name: "spans",
			s: func() ptrace.SpanEventSlice {
				spans := ptrace.NewSpanEventSlice()
				for i := 0; i < 5; i++ {
					e := spans.AppendEmpty()
					e.SetName(fmt.Sprintf("test%d", i))
					e.SetTimestamp(ts)
					e.Attributes().FromRaw(map[string]any{
						"str":  "abc",
						"num":  1,
						"bool": true,
					})
				}

				return spans
			}(),
			expectedLen: 20,
			expectedDoc: func() Document {
				var d Document

				for i := 0; i < 5; i++ {
					k := fmt.Sprintf("%s.test%d", key, i)
					d.Add(k+".time", TimestampValue(ts))
					d.Add(k+".str", StringValue("abc"))
					d.Add(k+".num", IntValue(1))
					d.Add(k+".bool", BoolValue(true))
				}

				d.Sort()
				return d
			}(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var actual Document
			p := NewSpansProcessor(tc.s)
			p.Process(&actual, key)
			actual.Sort()

			assert.Equal(t, tc.expectedLen, p.Len())
			assert.Equal(t, tc.expectedDoc, actual)
		})
	}
}
