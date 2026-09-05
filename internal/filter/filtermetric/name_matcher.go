// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filtermetric // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filtermetric"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filtermatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

// nameMatcher matches metrics by metric properties against prespecified values for each property.
type nameMatcher struct {
	nameFilters     filterset.FilterSet
	resourceFilters filtermatcher.AttributesMatcher
}

func newNameMatcher(mp *filterconfig.MetricMatchProperties) (*nameMatcher, error) {
	fsCfg := filterset.Config{
		MatchType:    filterset.MatchType(mp.MatchType),
		RegexpConfig: mp.RegexpConfig,
	}

	m := &nameMatcher{}

	if len(mp.MetricNames) > 0 {
		nameFS, err := filterset.CreateFilterSet(mp.MetricNames, &fsCfg)
		if err != nil {
			return nil, err
		}
		m.nameFilters = nameFS
	}

	if len(mp.ResourceAttributes) > 0 {
		rm, err := filtermatcher.NewAttributesMatcher(fsCfg, mp.ResourceAttributes)
		if err != nil {
			return nil, fmt.Errorf("error creating resource filters: %w", err)
		}
		m.resourceFilters = rm
	}

	return m, nil
}

// Eval matches a metric using the metric properties configured on the nameMatcher.
// A metric only matches if every metric property configured on the nameMatcher is a match.
func (m *nameMatcher) Eval(_ context.Context, tCtx *ottlmetric.TransformContext) (bool, error) {
	if m.nameFilters != nil && !m.nameFilters.Matches(tCtx.GetMetric().Name()) {
		return false, nil
	}
	if m.resourceFilters != nil && !m.resourceFilters.Match(tCtx.GetResource().Attributes()) {
		return false, nil
	}
	return true, nil
}
