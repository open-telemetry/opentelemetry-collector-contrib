// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grouper // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/grouper"

import (
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

func generateTraces(t *testing.T) ptrace.Traces {
	t.Helper()

	traces := ptrace.NewTraces()
	for range 10 {
		resourceSpans := traces.ResourceSpans().AppendEmpty()
		resourceSpans.Resource().Attributes().PutStr("id", uuid.NewString())
		for range 10 {
			scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
			scopeSpans.Scope().Attributes().PutStr("id", uuid.NewString())
			for range 10 {
				span := scopeSpans.Spans().AppendEmpty()
				span.Attributes().PutStr("id", uuid.NewString())
				span.Attributes().PutStr("subject", strconv.Itoa(rand.IntN(10)))
			}
		}
	}
	return traces
}

func groupTraces(t *testing.T, subject string, srcTraces ptrace.Traces) []Group[ptrace.Traces] {
	t.Helper()

	parser, err := ottlspan.NewParser(
		ottlfuncs.StandardConverters[ottlspan.TransformContext](),
		componenttest.NewNopTelemetrySettings(),
	)
	require.NoError(t, err)
	valueExpression, err := parser.ParseValueExpression(subject)
	require.NoError(t, err)

	subjectBySpan := make(map[ptrace.Span]string)
	for _, srcResourceSpans := range srcTraces.ResourceSpans().All() {
		for _, srcScopeSpans := range srcResourceSpans.ScopeSpans().All() {
			for _, srcSpan := range srcScopeSpans.Spans().All() {
				transformContext := ottlspan.NewTransformContext(
					srcSpan,
					srcScopeSpans.Scope(),
					srcResourceSpans.Resource(),
					srcScopeSpans,
					srcResourceSpans,
				)

				subjectAsAny, err := valueExpression.Eval(t.Context(), transformContext)
				require.NoError(t, err)
				subject := subjectAsAny.(string)

				subjectBySpan[srcSpan] = subject
			}
		}
	}

	subjects := make(map[string]bool)
	for _, subject := range subjectBySpan {
		subjects[subject] = true
	}

	groups := make([]Group[ptrace.Traces], 0, len(subjects))
	for groupSubject := range subjects {
		destResourceSpansSlice := ptrace.NewResourceSpansSlice()
		for _, srcResourceSpans := range srcTraces.ResourceSpans().All() {
			destScopeSpansSlice := ptrace.NewScopeSpansSlice()
			for _, srcScopeSpans := range srcResourceSpans.ScopeSpans().All() {
				destSpanSlice := ptrace.NewSpanSlice()
				for _, srcSpan := range srcScopeSpans.Spans().All() {
					if subjectBySpan[srcSpan] == groupSubject {
						srcSpan.CopyTo(destSpanSlice.AppendEmpty())
					}
				}

				if destSpanSlice.Len() > 0 {
					destScopeSpans := destScopeSpansSlice.AppendEmpty()
					srcScopeSpans.CopyTo(destScopeSpans)
					destSpanSlice.CopyTo(destScopeSpans.Spans())
				}
			}

			if destScopeSpansSlice.Len() > 0 {
				destResourceSpans := destResourceSpansSlice.AppendEmpty()
				srcResourceSpans.CopyTo(destResourceSpans)
				destScopeSpansSlice.CopyTo(destResourceSpans.ScopeSpans())
			}
		}

		if destResourceSpansSlice.Len() > 0 {
			destTraces := ptrace.NewTraces()
			destResourceSpansSlice.CopyTo(destTraces.ResourceSpans())
			groups = append(groups, Group[ptrace.Traces]{
				Subject: groupSubject,
				Data:    destTraces,
			})
		}
	}
	return groups
}

func TestTracesGrouper(t *testing.T) {
	t.Parallel()

	t.Run("consistent with naive implementation", func(t *testing.T) {
		subject := "span.attributes[\"subject\"]"
		srcTraces := generateTraces(t)

		tracesGrouper, err := NewTracesGrouper(subject, componenttest.NewNopTelemetrySettings())
		assert.NoError(t, err)
		haveGroups, err := tracesGrouper.Group(t.Context(), srcTraces)
		assert.NoError(t, err)

		wantGroups := groupTraces(t, subject, srcTraces)

		compareGroups := func(a, b Group[ptrace.Traces]) int {
			return strings.Compare(a.Subject, b.Subject)
		}
		slices.SortFunc(wantGroups, compareGroups)
		slices.SortFunc(haveGroups, compareGroups)

		assert.Len(t, wantGroups, len(haveGroups))
		for i := range len(wantGroups) {
			assert.NoError(t, ptracetest.CompareTraces(
				wantGroups[i].Data,
				haveGroups[i].Data,
			))
		}
	})
}
