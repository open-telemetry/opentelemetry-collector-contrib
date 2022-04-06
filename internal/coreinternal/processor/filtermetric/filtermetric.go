// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filtermetric // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filtermetric"

import (
	"fmt"

	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterexpr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filtermatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
)

type FilterType int

const (
	INCLUDE FilterType = iota + 1
	EXCLUDE
)

type Matcher interface {
	MatchMetric(metric pdata.Metric) (bool, error)
	MatchWholeMetric(metric pdata.Metric, resource pdata.Resource, library pdata.InstrumentationScope, filterType FilterType) (bool, error)
	MatchAttributes(atts pdata.AttributeMap, resource pdata.Resource, library pdata.InstrumentationScope) bool
	ChecksAttributes() bool
}

// propertiesMatcher allows matching a metric against various metric properties.
type propertiesMatcher struct {
	filtermatcher.PropertiesMatcher

	// Service names to compare to.
	serviceFilters filterset.FilterSet

	// Span names to compare to.
	nameFilters filterset.FilterSet

	// expr matchers
	exprMatchers []filterexpr.Matcher
}

// NewMatcher creates a span Matcher that matches based on the given MatchProperties.
func NewMatcher(mp *filterconfig.MatchProperties) (Matcher, error) {
	if mp == nil {
		return nil, nil
	}

	if err := mp.ValidateForMetrics(); err != nil {
		return nil, err
	}

	rm, err := filtermatcher.NewMatcher(mp)
	if err != nil {
		return nil, err
	}

	var serviceFS filterset.FilterSet
	if len(mp.Services) > 0 {
		serviceFS, err = filterset.CreateFilterSet(mp.Services, &mp.Config)
		if err != nil {
			return nil, fmt.Errorf("error creating service name filters: %v", err)
		}
	}

	var nameFS filterset.FilterSet
	if len(mp.MetricNames) > 0 {
		nameFS, err = filterset.CreateFilterSet(mp.MetricNames, &mp.Config)
		if err != nil {
			return nil, fmt.Errorf("error creating metric name filters: %v", err)
		}
	}

	var em []filterexpr.Matcher
	if mp.MatchType == filterset.Expr {
		for _, e := range mp.Expressions {
			expr, err := filterexpr.NewMatcher(e)
			if err != nil {
				return nil, fmt.Errorf("error creating expression filters: %v", err)
			}
			em = append(em, *expr)
		}
	}

	return &propertiesMatcher{
		PropertiesMatcher: rm,
		serviceFilters:    serviceFS,
		nameFilters:       nameFS,
		exprMatchers:      em,
	}, nil
}

// Filters have the ability to include and exclude metrics based on the metric's properties.
// The default is to not skip. If include is defined, the metric must match or it will be skipped.
// If include is not defined but exclude is, metric will be skipped if it matches exclude. Metric
// is included if neither specified.
func SkipMetric(include, exclude Matcher, metric pdata.Metric, resource pdata.Resource, library pdata.InstrumentationScope, logger *zap.Logger) bool {
	if include != nil {
		// A false (or an error) returned in this case means the metric should not be processed.
		i, err := include.MatchWholeMetric(metric, resource, library, INCLUDE)
		if !i || err != nil {
			logger.Debug("Skipping metric",
				zap.String("metric_name", metric.Name()),
				zap.Error(err)) // zap.Error handles case where err is nil
			return true
		}
	}

	if exclude != nil {
		// A true (or an error) returned in this case means the metric should not be processed.
		e, err := exclude.MatchWholeMetric(metric, resource, library, EXCLUDE)
		if e || err != nil {
			logger.Debug("Skipping metric",
				zap.String("metric_name", metric.Name()),
				zap.Error(err)) // zap.Error handles case where err is nil
			return true
		}
	}

	return false
}

// MatchMetric matches a metric and service to a set of properties.
// see filterconfig.MatchProperties for more details
func (mp *propertiesMatcher) MatchMetric(metric pdata.Metric) (bool, error) {
	if len(mp.exprMatchers) > 0 {
		for _, matcher := range mp.exprMatchers {
			matched, err := matcher.MatchMetric(metric)
			if err != nil {
				return false, err
			}
			if matched {
				return true, nil
			}
		}
	}
	if mp.nameFilters != nil {
		return mp.nameFilters.Matches(metric.Name()), nil
	} else {
		return false, nil
	}
}

// MatchWholeMetric matches a metric, resource and libraries.
// To avoid double-traversal through all data points when we revisit it in the attributesprocessor,
// we omit checking attributes on individual data points in this function.
// This is the reason we also take a FilterType as an argument - so as not to exclude
// data points prematurely.
// see filterconfig.MatchProperties for more details
func (mp *propertiesMatcher) MatchWholeMetric(metric pdata.Metric, resource pdata.Resource, library pdata.InstrumentationScope, filterType FilterType) (bool, error) {
	// If a set of properties was not in the mp, all spans are considered to match on that property
	if mp.serviceFilters != nil {
		serviceName := serviceNameForResource(resource)
		if !mp.serviceFilters.Matches(serviceName) {
			return false, nil
		}
	}

	if mp.nameFilters != nil && !mp.nameFilters.Matches(metric.Name()) {
		return false, nil
	}

	// checking individual datapoints is less efficient. Skip it if possible
	if !mp.PropertiesMatcher.ChecksAttributes() {
		return mp.PropertiesMatcher.Match(pdata.AttributeMap{}, resource, library), nil
	}

	// if we check the attributes to determine matching, then we need to default to 'includes' to be safe
	if filterType == EXCLUDE {
		return false, nil
	}
	if filterType == INCLUDE {
		return true, nil
	}
	return false, fmt.Errorf("unrecognised filterType")
}

// MatchAttributes matches metric attributes, resource and libraries.
// It omits checks on metrics name and services.
func (mp *propertiesMatcher) MatchAttributes(atts pdata.AttributeMap, resource pdata.Resource, library pdata.InstrumentationScope) bool {
	return mp.PropertiesMatcher.Match(atts, resource, library)
}

// serviceNameForResource gets the service name for a specified Resource.
func serviceNameForResource(resource pdata.Resource) string {
	service, found := resource.Attributes().Get(conventions.AttributeServiceName)
	if !found {
		executable, found := resource.Attributes().Get(conventions.AttributeProcessExecutableName)
		if !found {
			return "unknown_service"
		}
		return fmt.Sprintf("unknown_service:%s", executable.StringVal())
	}

	return service.StringVal()
}
