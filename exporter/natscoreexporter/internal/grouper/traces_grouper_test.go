// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grouper // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/grouper"

import (
	"context"
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
)

func generateTraces(t *testing.T) ptrace.Traces {
	t.Helper()

	traces := ptrace.NewTraces()
	for range 10 {
		resourceSpans := traces.ResourceSpans().AppendEmpty()
		resourceSpans.Resource().Attributes().PutStr("id", uuid.New().String())
		for range 10 {
			scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
			scopeSpans.Scope().Attributes().PutStr("id", uuid.New().String())
			for range 10 {
				span := scopeSpans.Spans().AppendEmpty()
				span.Attributes().PutStr("id", uuid.New().String())
				span.Attributes().PutStr("subject", strconv.Itoa(rand.Intn(10)))
			}
		}
	}
	return traces
}

type naiveTracesGrouper struct {
	valueExpression *ottl.ValueExpression[ottlspan.TransformContext]
}

func (g *naiveTracesGrouper) Group(
	ctx context.Context,
	srcTraces ptrace.Traces,
) ([]Group[ptrace.Traces], error) {
	var errs error

	subjectByLogRecord := make(map[ptrace.Span]string)
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
				subjectAsAny, err := g.valueExpression.Eval(ctx, transformContext)
				if err != nil {
					errs = multierr.Append(errs, err)
					continue
				}
				subject := subjectAsAny.(string)
				subjectByLogRecord[srcSpan] = subject
			}
		}
	}

	subjects := make(map[string]bool)
	for _, subject := range subjectByLogRecord {
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
					if subjectByLogRecord[srcSpan] == groupSubject {
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
	return groups, errs
}

var _ Grouper[ptrace.Traces] = &naiveTracesGrouper{}

func newNaiveTracesGrouper(
	subject string,
	telemetrySettings component.TelemetrySettings,
) (Grouper[ptrace.Traces], error) {
	parser, err := ottlspan.NewParser(
		ottlfuncs.StandardConverters[ottlspan.TransformContext](),
		telemetrySettings,
	)
	if err != nil {
		return nil, err
	}

	valueExpression, err := parser.ParseValueExpression(subject)
	if err != nil {
		return nil, err
	}

	return &naiveTracesGrouper{valueExpression: valueExpression}, nil
}

var _ NewGrouperFunc[ptrace.Traces] = newNaiveTracesGrouper

func TestTracesGrouper(t *testing.T) {
	t.Parallel()

	t.Run("matches naive implementation", func(t *testing.T) {
		subject := "span.attributes[\"subject\"]"
		telemetrySettings := componenttest.NewNopTelemetrySettings()
		ctx := context.Background()
		srcTraces := generateTraces(t)

		naiveTracesGrouper, err := newNaiveTracesGrouper(subject, telemetrySettings)
		require.NoError(t, err)
		tracesGrouper, err := NewTracesGrouper(subject, telemetrySettings)
		assert.NoError(t, err)

		wantGroups, err := naiveTracesGrouper.Group(ctx, srcTraces)
		require.NoError(t, err)
		haveGroups, err := tracesGrouper.Group(ctx, srcTraces)
		assert.NoError(t, err)

		compareGroups := func(a, b Group[ptrace.Traces]) int {
			return strings.Compare(a.Subject, b.Subject)
		}
		slices.SortFunc(wantGroups, compareGroups)
		slices.SortFunc(haveGroups, compareGroups)
		assert.Equal(t, len(wantGroups), len(haveGroups))
		for i := range len(wantGroups) {
			assert.NoError(t, ptracetest.CompareTraces(
				wantGroups[i].Data,
				haveGroups[i].Data,
			))
		}
	})
}
