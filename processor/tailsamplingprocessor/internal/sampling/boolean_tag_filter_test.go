// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sampling

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func TestBooleanTagFilter(t *testing.T) {

	var empty = map[string]interface{}{}
	filter := NewBooleanAttributeFilter(zap.NewNop(), "example", true)

	resAttr := map[string]interface{}{}
	resAttr["example"] = 8

	cases := []struct {
		Desc     string
		Trace    *TraceData
		Decision Decision
	}{
		{
			Desc:     "non-matching span attribute",
			Trace:    newTraceBoolAttrs(empty, "non_matching", true),
			Decision: NotSampled,
		},
		{
			Desc:     "span attribute with unwanted boolean value",
			Trace:    newTraceBoolAttrs(empty, "example", false),
			Decision: NotSampled,
		},
		{
			Desc:     "span attribute with wanted boolean value",
			Trace:    newTraceBoolAttrs(empty, "example", true),
			Decision: Sampled,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			u, _ := uuid.NewRandom()
			decision, err := filter.Evaluate(pcommon.TraceID(u), c.Trace)
			assert.NoError(t, err)
			assert.Equal(t, decision, c.Decision)
		})
	}
}

func newTraceBoolAttrs(nodeAttrs map[string]interface{}, spanAttrKey string, spanAttrValue bool) *TraceData {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	//nolint:errcheck
	rs.Resource().Attributes().FromRaw(nodeAttrs)
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	span.Attributes().PutBool(spanAttrKey, spanAttrValue)
	return &TraceData{
		ReceivedBatches: traces,
	}
}
