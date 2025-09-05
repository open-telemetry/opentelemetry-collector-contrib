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
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func groupMetrics(t *testing.T, subject string, srcMetrics pmetric.Metrics) ([]Group[pmetric.Metrics], error) {
	parser, err := ottlmetric.NewParser(
		ottlfuncs.StandardConverters[ottlmetric.TransformContext](),
		componenttest.NewNopTelemetrySettings(),
	)
	require.NoError(t, err)
	valueExpression, err := parser.ParseValueExpression(subject)
	require.NoError(t, err)

	constructSubject := func(resourceMetrics pmetric.ResourceMetrics, scopeMetrics pmetric.ScopeMetrics, metric pmetric.Metric) (string, error) {
		subjectAsAny, err := valueExpression.Eval(t.Context(), ottlmetric.NewTransformContext(
			metric,
			scopeMetrics.Metrics(),
			scopeMetrics.Scope(),
			resourceMetrics.Resource(),
			scopeMetrics,
			resourceMetrics,
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
	for _, srcResourceMetrics := range srcMetrics.ResourceMetrics().All() {
		for _, srcScopeMetrics := range srcResourceMetrics.ScopeMetrics().All() {
			for _, srcMetric := range srcScopeMetrics.Metrics().All() {
				subject, err := constructSubject(srcResourceMetrics, srcScopeMetrics, srcMetric)
				if err == nil {
					subjects[subject] = true
				} else {
					errs = multierr.Append(errs, err)
				}
			}
		}
	}

	groups := make([]Group[pmetric.Metrics], 0, len(subjects))
	for groupSubject := range subjects {
		destMetrics := pmetric.NewMetrics()
		srcMetrics.CopyTo(destMetrics)
		destMetrics.ResourceMetrics().RemoveIf(func(destResourceMetrics pmetric.ResourceMetrics) bool {
			destResourceMetrics.ScopeMetrics().RemoveIf(func(destScopeMetrics pmetric.ScopeMetrics) bool {
				destScopeMetrics.Metrics().RemoveIf(func(destMetric pmetric.Metric) bool {
					subject, err := constructSubject(destResourceMetrics, destScopeMetrics, destMetric)
					if err == nil {
						return subject != groupSubject
					} else {
						return true
					}
				})
				return destScopeMetrics.Metrics().Len() == 0
			})
			return destResourceMetrics.ScopeMetrics().Len() == 0
		})
		groups = append(groups, Group[pmetric.Metrics]{
			Subject: groupSubject,
			Data:    destMetrics,
		})
	}
	return groups, errs
}

func TestMetricsGrouper(t *testing.T) {
	t.Parallel()

	t.Run("consistent with naive implementation", func(t *testing.T) {
		metricsDir := "testdata/metrics"

		entries, err := os.ReadDir(metricsDir)
		require.NoError(t, err)

		for _, entry := range entries {
			t.Run(entry.Name(), func(t *testing.T) {
				testCaseDir := filepath.Join(metricsDir, entry.Name())

				subjectAsBytes, err := os.ReadFile(filepath.Join(testCaseDir, "subject.txt"))
				require.NoError(t, err)
				subject := string(subjectAsBytes)

				srcMetrics, err := golden.ReadMetrics(filepath.Join(testCaseDir, "metrics.yaml"))
				require.NoError(t, err)

				metricsGrouper, err := NewMetricsGrouper(subject, componenttest.NewNopTelemetrySettings())
				assert.NoError(t, err)

				haveGroups, haveErr := metricsGrouper.Group(t.Context(), srcMetrics)
				wantGroups, wantErr := groupMetrics(t, subject, srcMetrics)
				assert.ElementsMatch(t, multierr.Errors(haveErr), multierr.Errors(wantErr))

				compareGroups := func(a, b Group[pmetric.Metrics]) int {
					return strings.Compare(a.Subject, b.Subject)
				}
				slices.SortFunc(wantGroups, compareGroups)
				slices.SortFunc(haveGroups, compareGroups)

				assert.Len(t, wantGroups, len(haveGroups))
				for i := range len(wantGroups) {
					assert.NoError(t, pmetrictest.CompareMetrics(
						wantGroups[i].Data,
						haveGroups[i].Data,
					))
				}
			})
		}
	})
}
