// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grouper

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

func groupTraces(t *testing.T, subject string, srcTraces ptrace.Traces) ([]Group[ptrace.Traces], error) {
	parser, err := ottlspan.NewParser(
		ottlfuncs.StandardConverters[ottlspan.TransformContext](),
		componenttest.NewNopTelemetrySettings(),
	)
	require.NoError(t, err)
	valueExpression, err := parser.ParseValueExpression(subject)
	require.NoError(t, err)

	constructSubject := func(resourceSpans ptrace.ResourceSpans, scopeSpans ptrace.ScopeSpans, span ptrace.Span) (string, error) {
		subjectAsAny, err := valueExpression.Eval(t.Context(), ottlspan.NewTransformContext(
			span,
			scopeSpans.Scope(),
			resourceSpans.Resource(),
			scopeSpans,
			resourceSpans,
		))
		if err != nil {
			return "", err
		}

		subject, ok := subjectAsAny.(string)
		if !ok {
			return "", errors.New("subject is not a string")
		}
		return subject, nil
	}

	subjects := make(map[string]bool)
	var errs error
	for _, srcResourceSpans := range srcTraces.ResourceSpans().All() {
		for _, srcScopeSpans := range srcResourceSpans.ScopeSpans().All() {
			for _, srcSpan := range srcScopeSpans.Spans().All() {
				subject, err := constructSubject(srcResourceSpans, srcScopeSpans, srcSpan)
				if err == nil {
					subjects[subject] = true
				} else {
					errs = multierr.Append(errs, err)
				}
			}
		}
	}

	groups := make([]Group[ptrace.Traces], 0, len(subjects))
	for groupSubject := range subjects {
		destTraces := ptrace.NewTraces()
		srcTraces.CopyTo(destTraces)
		destTraces.ResourceSpans().RemoveIf(func(destResourceSpans ptrace.ResourceSpans) bool {
			destResourceSpans.ScopeSpans().RemoveIf(func(destScopeSpans ptrace.ScopeSpans) bool {
				destScopeSpans.Spans().RemoveIf(func(destSpan ptrace.Span) bool {
					subject, err := constructSubject(destResourceSpans, destScopeSpans, destSpan)
					if err == nil {
						return subject != groupSubject
					} else {
						return true
					}
				})
				return destScopeSpans.Spans().Len() == 0
			})
			return destResourceSpans.ScopeSpans().Len() == 0
		})
		groups = append(groups, Group[ptrace.Traces]{
			Subject: groupSubject,
			Data:    destTraces,
		})
	}
	return groups, errs
}

func TestTracesGrouper(t *testing.T) {
	t.Parallel()

	t.Run("consistent with naive implementation", func(t *testing.T) {
		tracesDir := "testdata/traces"

		entries, err := os.ReadDir(tracesDir)
		require.NoError(t, err)

		for _, entry := range entries {
			t.Run(entry.Name(), func(t *testing.T) {
				testCaseDir := filepath.Join(tracesDir, entry.Name())

				subjectAsBytes, err := os.ReadFile(filepath.Join(testCaseDir, "subject.txt"))
				require.NoError(t, err)
				subject := string(subjectAsBytes)

				srcTraces, err := golden.ReadTraces(filepath.Join(testCaseDir, "traces.yaml"))
				require.NoError(t, err)

				tracesGrouper, err := NewTracesGrouper(subject, componenttest.NewNopTelemetrySettings())
				assert.NoError(t, err)

				haveGroups, haveErr := tracesGrouper.Group(t.Context(), srcTraces)
				wantGroups, wantErr := groupTraces(t, subject, srcTraces)
				assert.ElementsMatch(t, multierr.Errors(haveErr), multierr.Errors(wantErr))

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
	})
}
