// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grouper

import (
	"context"
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
)

func generateMetrics(t *testing.T) pmetric.Metrics {
	t.Helper()

	metrics := pmetric.NewMetrics()
	for range 10 {
		resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
		resourceMetrics.Resource().Attributes().PutStr("id", uuid.New().String())
		for range 10 {
			scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
			scopeMetrics.Scope().Attributes().PutStr("id", uuid.New().String())
			for range 10 {
				metric := scopeMetrics.Metrics().AppendEmpty()
				metric.Metadata().PutStr("id", uuid.New().String())
				metric.Metadata().PutStr("subject", strconv.Itoa(rand.Intn(10)))
			}
		}
	}
	return metrics
}

type naiveMetricsGrouper struct {
	valueExpression *ottl.ValueExpression[ottlmetric.TransformContext]
}

func (g *naiveMetricsGrouper) Group(
	ctx context.Context,
	srcMetrics pmetric.Metrics,
) ([]Group[pmetric.Metrics], error) {
	var errs error

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
				subjectAsAny, err := g.valueExpression.Eval(ctx, transformContext)
				if err != nil {
					errs = multierr.Append(errs, err)
					continue
				}
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
	return groups, errs
}

var _ Grouper[pmetric.Metrics] = &naiveMetricsGrouper{}

func newNaiveMetricsGrouper(
	subject string,
	telemetrySettings component.TelemetrySettings,
) (Grouper[pmetric.Metrics], error) {
	parser, err := ottlmetric.NewParser(
		ottlfuncs.StandardConverters[ottlmetric.TransformContext](),
		telemetrySettings,
	)
	if err != nil {
		return nil, err
	}

	valueExpression, err := parser.ParseValueExpression(subject)
	if err != nil {
		return nil, err
	}

	return &naiveMetricsGrouper{valueExpression: valueExpression}, nil
}

var _ NewGrouperFunc[pmetric.Metrics] = newNaiveMetricsGrouper

func TestMetricsGrouper(t *testing.T) {
	t.Parallel()

	t.Run("matches naive implementation", func(t *testing.T) {
		subject := "metric.metadata[\"subject\"]"
		telemetrySettings := componenttest.NewNopTelemetrySettings()
		ctx := context.Background()
		srcMetrics := generateMetrics(t)

		naiveMetricsGrouper, err := newNaiveMetricsGrouper(subject, telemetrySettings)
		require.NoError(t, err)
		metricsGrouper, err := NewMetricsGrouper(subject, telemetrySettings)
		assert.NoError(t, err)

		wantGroups, err := naiveMetricsGrouper.Group(ctx, srcMetrics)
		require.NoError(t, err)
		haveGroups, err := metricsGrouper.Group(ctx, srcMetrics)
		assert.NoError(t, err)

		compareGroups := func(a, b Group[pmetric.Metrics]) int {
			return strings.Compare(a.Subject, b.Subject)
		}
		slices.SortFunc(wantGroups, compareGroups)
		slices.SortFunc(haveGroups, compareGroups)
		assert.Equal(t, len(wantGroups), len(haveGroups))
		for i := range len(wantGroups) {
			assert.NoError(t, pmetrictest.CompareMetrics(
				wantGroups[i].Data,
				haveGroups[i].Data,
			))
		}
	})
}
