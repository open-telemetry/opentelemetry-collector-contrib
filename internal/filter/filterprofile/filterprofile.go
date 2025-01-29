// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprofile // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterprofile"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filtermatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
)

var UseOTTLBridge = featuregate.GlobalRegistry().MustRegister(
	"filter.filterprofile.useOTTLBridge",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, filterprofile will convert filterprofile configuration to OTTL and use filterottl evaluation"),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18642"),
)

// NewSkipExpr creates a BoolExpr that on evaluation returns true if a profile should NOT be processed or kept.
// The logic determining if a metric should be processed is based on include and exclude settings.
// Include properties are checked before exclude settings are checked.
func NewSkipExpr(mp *filterconfig.MatchConfig) (expr.BoolExpr[ottlprofile.TransformContext], error) {
	if UseOTTLBridge.IsEnabled() {
		return filterottl.NewProfileSkipExprBridge(mp)
	}
	var matchers []expr.BoolExpr[ottlprofile.TransformContext]
	inclExpr, err := newExpr(mp.Include)
	if err != nil {
		return nil, err
	}
	if inclExpr != nil {
		matchers = append(matchers, expr.Not(inclExpr))
	}
	exclExpr, err := newExpr(mp.Exclude)
	if err != nil {
		return nil, err
	}
	if exclExpr != nil {
		matchers = append(matchers, exclExpr)
	}
	return expr.Or(matchers...), nil
}

// newExpr creates a BoolExpr that matches based on the given MatchProperties.
func newExpr(mp *filterconfig.MatchProperties) (expr.BoolExpr[ottlprofile.TransformContext], error) {
	if mp == nil {
		return nil, nil
	}

	rm, err := filtermatcher.NewMatcher(mp)
	if err != nil {
		return nil, err
	}

	var serviceFS filterset.FilterSet
	if len(mp.Services) > 0 {
		serviceFS, err = filterset.CreateFilterSet(mp.Services, &mp.Config)
		if err != nil {
			return nil, fmt.Errorf("error creating service name filters: %w", err)
		}
	}

	return &propertiesMatcher{
		PropertiesMatcher: rm,
		serviceFilters:    serviceFS,
	}, nil
}

type propertiesMatcher struct {
	filtermatcher.PropertiesMatcher

	// Service names to compare to.
	serviceFilters filterset.FilterSet
}

// Eval matches a span and service to a set of properties.
// see filterconfig.MatchProperties for more details
func (mp *propertiesMatcher) Eval(_ context.Context, tCtx ottlprofile.TransformContext) (bool, error) {
	if mp.serviceFilters != nil {
		// Check resource and spans for service.name
		serviceName := serviceNameForResource(tCtx.GetResource())

		if !mp.serviceFilters.Matches(serviceName) {
			return false, nil
		}
	}

	p := tCtx.GetProfile()
	attrs := pprofile.FromAttributeIndices(p.AttributeTable(), p)
	return mp.PropertiesMatcher.Match(attrs, tCtx.GetResource(), tCtx.GetInstrumentationScope()), nil
}

// serviceNameForResource gets the service name for a specified Resource.
func serviceNameForResource(resource pcommon.Resource) string {
	service, found := resource.Attributes().Get(conventions.AttributeServiceName)
	if !found {
		return "<nil-service-name>"
	}
	return service.AsString()
}
