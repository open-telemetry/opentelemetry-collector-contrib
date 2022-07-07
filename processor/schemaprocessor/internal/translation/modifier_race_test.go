// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package translation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/fixture"
)

func TestRaceModify(t *testing.T) {
	t.Parallel()

	mod := modify{
		names: map[string]string{
			"foo": "bar",
		},
		appliesTo: map[string]struct{}{},
		attrs: map[string]string{
			"old":     "new",
			"compute": "cloud",
		},
	}

	traces := ptrace.NewTraces()
	rSpan := traces.ResourceSpans().AppendEmpty()
	rSpan.Resource().Attributes().PutEmpty("old")
	rSpan.Resource().Attributes().PutEmpty("compute")
	span := rSpan.ScopeSpans().AppendEmpty()
	s := span.Spans().AppendEmpty()
	s.Attributes().PutEmpty("old")
	s.Attributes().PutEmpty("compute")
	s.SetName("foo")

	fixture.ParallelRaceCompute(t, 100, func() error {
		trace := ptrace.NewTraces()
		traces.CopyTo(trace)

		for i := 0; i < trace.ResourceSpans().Len(); i++ {
			spans := trace.ResourceSpans().At(i)
			mod.UpdateAttrs(spans.Resource().Attributes())
			for t := 0; t < spans.ScopeSpans().Len(); t++ {
				trace := spans.ScopeSpans().At(t)
				for s := 0; s < trace.Spans().Len(); s++ {
					span := trace.Spans().At(s)
					mod.UpdateAttrs(span.Attributes())
					mod.UpdateAttrsIf(span.Name(), span.Attributes())
					mod.UpdateSignal(span)
					mod.RevertSignal(span)
					mod.RevertAttrsIf(span.Name(), span.Attributes())
					mod.RevertAttrs(span.Attributes())
				}
			}
			mod.RevertAttrs(spans.Resource().Attributes())
		}

		assert.NoError(t, ptracetest.CompareTraces(traces, trace))
		return nil
	})
}
