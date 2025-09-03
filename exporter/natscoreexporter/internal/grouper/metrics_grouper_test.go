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

func generateMetrics(t *testing.T) pmetric.Metrics {
	t.Helper()

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
	t.Helper()

	parser, err := ottlmetric.NewParser(
		ottlfuncs.StandardConverters[ottlmetric.TransformContext](),
		componenttest.NewNopTelemetrySettings(),
	)
	require.NoError(t, err)
	valueExpression, err := parser.ParseValueExpression(subject)
	require.NoError(t, err)

	subjectByMetric := make(map[pmetric.Metric]string)
	for _, srcResourceMetrics := range srcMetrics.ResourceMetrics().All() {
		for _, srcScopeMetrics := range srcResourceMetrics.ScopeMetrics().All() {
			for _, srcMetric := range srcScopeMetrics.Metrics().All() {
				transformContext := ottlmetric.NewTransformContext(
					srcMetric,
					srcScopeMetrics.Metrics(),
					srcScopeMetrics.Scope(),
					srcResourceMetrics.Resource(),
					srcScopeMetrics,
					srcResourceMetrics,
				)

				subjectAsAny, err := valueExpression.Eval(t.Context(), transformContext)
				require.NoError(t, err)
				subject := subjectAsAny.(string)

				subjectByMetric[srcMetric] = subject
			}
		}
	}

	subjects := make(map[string]bool)
	for _, subject := range subjectByMetric {
		subjects[subject] = true
	}

	groups := make([]Group[pmetric.Metrics], 0, len(subjects))
	for groupSubject := range subjects {
		destResourceMetricsSlice := pmetric.NewResourceMetricsSlice()
		for _, srcResourceMetrics := range srcMetrics.ResourceMetrics().All() {
			destScopeMetricsSlice := pmetric.NewScopeMetricsSlice()
			for _, srcScopeMetrics := range srcResourceMetrics.ScopeMetrics().All() {
				destMetricSlice := pmetric.NewMetricSlice()
				for _, srcMetric := range srcScopeMetrics.Metrics().All() {
					if subjectByMetric[srcMetric] == groupSubject {
						srcMetric.CopyTo(destMetricSlice.AppendEmpty())
					}
				}

				if destMetricSlice.Len() > 0 {
					destScopeMetrics := destScopeMetricsSlice.AppendEmpty()
					srcScopeMetrics.CopyTo(destScopeMetrics)
					destMetricSlice.CopyTo(destScopeMetrics.Metrics())
				}
			}

			if destScopeMetricsSlice.Len() > 0 {
				destResourceMetrics := destResourceMetricsSlice.AppendEmpty()
				srcResourceMetrics.CopyTo(destResourceMetrics)
				destScopeMetricsSlice.CopyTo(destResourceMetrics.ScopeMetrics())
			}
		}

		if destResourceMetricsSlice.Len() > 0 {
			destMetrics := pmetric.NewMetrics()
			destResourceMetricsSlice.CopyTo(destMetrics.ResourceMetrics())
			groups = append(groups, Group[pmetric.Metrics]{
				Subject: groupSubject,
				Data:    destMetrics,
			})
		}
	}
	return groups
}

func TestMetricsGrouper(t *testing.T) {
	t.Parallel()

	t.Run("consistent with naive implementation", func(t *testing.T) {
		subject := "metric.metadata[\"subject\"]"
		srcMetrics := generateMetrics(t)

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
