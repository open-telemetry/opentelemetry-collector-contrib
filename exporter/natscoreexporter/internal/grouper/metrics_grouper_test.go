// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grouper

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
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func generateMetrics() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	for range 10 {
		resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
		resourceMetrics.Resource().Attributes().PutStr("id", uuid.NewString())
		for range 10 {
			scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
			scopeMetrics.Scope().Attributes().PutStr("id", uuid.NewString())
			for range 10 {
				metric := scopeMetrics.Metrics().AppendEmpty()
				metric.Metadata().PutStr("id", uuid.NewString())
				metric.Metadata().PutStr("subject", strconv.Itoa(rand.IntN(10)))
			}
		}
	}
	return metrics
}

func groupMetrics(t *testing.T, subject string, srcMetrics pmetric.Metrics) []Group[pmetric.Metrics] {
	parser, err := ottlmetric.NewParser(
		ottlfuncs.StandardConverters[ottlmetric.TransformContext](),
		componenttest.NewNopTelemetrySettings(),
	)
	require.NoError(t, err)
	valueExpression, err := parser.ParseValueExpression(subject)
	require.NoError(t, err)

	constructSubject := func(resourceMetrics pmetric.ResourceMetrics, scopeMetrics pmetric.ScopeMetrics, metric pmetric.Metric) string {
		subjectAsAny, err := valueExpression.Eval(t.Context(), ottlmetric.NewTransformContext(
			metric,
			scopeMetrics.Metrics(),
			scopeMetrics.Scope(),
			resourceMetrics.Resource(),
			scopeMetrics,
			resourceMetrics,
		))
		require.NoError(t, err)
		return subjectAsAny.(string)
	}

	subjects := make(map[string]bool)
	for _, srcResourceMetrics := range srcMetrics.ResourceMetrics().All() {
		for _, srcScopeMetrics := range srcResourceMetrics.ScopeMetrics().All() {
			for _, srcMetric := range srcScopeMetrics.Metrics().All() {
				subjects[constructSubject(srcResourceMetrics, srcScopeMetrics, srcMetric)] = true
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
					return constructSubject(destResourceMetrics, destScopeMetrics, destMetric) != groupSubject
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
	return groups
}

func TestMetricsGrouper(t *testing.T) {
	t.Parallel()

	t.Run("consistent with naive implementation", func(t *testing.T) {
		subject := "metric.metadata[\"subject\"]"
		srcMetrics := generateMetrics()

		metricsGrouper, err := NewMetricsGrouper(subject, componenttest.NewNopTelemetrySettings())
		assert.NoError(t, err)
		haveGroups, err := metricsGrouper.Group(t.Context(), srcMetrics)
		assert.NoError(t, err)

		wantGroups := groupMetrics(t, subject, srcMetrics)

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
