// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcetotelemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// Settings defines configuration for converting resource attributes to telemetry attributes or constant labels.
// When used, it must be embedded in the exporter configuration:
//
//	type Config struct {
//	  // ...
//	  resourcetotelemetry.Settings `mapstructure:"resource_constant_labels"`
//	}
type Settings struct {
	// Included represents a list of patterns (supporting wildcards like "*") of resource attribute keys to include.
	// Note: if Included is empty and Excluded is non-empty, all resource attributes except those matched by Excluded will be included.
	Included []string `mapstructure:"included"`
	// Excluded represents a list of patterns (supporting wildcards like "*") of resource attribute keys to exclude,
	// overriding anything that matched Included. If Included is empty, setting Excluded implies including all non-excluded attributes.
	Excluded []string `mapstructure:"excluded"`
	// Enabled indicates whether to convert resource attributes to telemetry attributes. Default is `false`.
	//
	// Deprecated: Use Included and Excluded instead. To convert all resource attributes, set Included to ["*"].
	Enabled bool `mapstructure:"enabled"`
	// ExcludeServiceAttributes indicates whether to exclude `service.name`, `service.instance.id` and `service.namespace`
	// resource attributes from being converted to metric attributes. Default is `false`.
	// When set to `true`, these attributes will not be added to metric labels since they are
	// already mapped to Prometheus `job` and `instance` labels respectively.
	//
	// Deprecated: Use Excluded instead. To exclude service attributes, add "service.name", "service.instance.id", and "service.namespace" to Excluded.
	ExcludeServiceAttributes bool `mapstructure:"exclude_service_attributes"`
}

// IsEmpty returns true if neither legacy fields nor Included/Excluded patterns are configured.
func (s *Settings) IsEmpty() bool {
	return !s.Enabled && !s.ExcludeServiceAttributes && len(s.Included) == 0 && len(s.Excluded) == 0
}

// Validate checks if the Settings configuration is valid.
func (s *Settings) Validate() error {
	if (s.Enabled || s.ExcludeServiceAttributes) && (len(s.Included) > 0 || len(s.Excluded) > 0) {
		return errors.New("cannot configure both legacy enabled/exclude_service_attributes and included/excluded patterns; enabled and exclude_service_attributes are deprecated")
	}
	for _, p := range s.Included {
		if _, err := globToRegexp(p); err != nil {
			return fmt.Errorf("invalid included pattern %q: %w", p, err)
		}
	}
	for _, p := range s.Excluded {
		if _, err := globToRegexp(p); err != nil {
			return fmt.Errorf("invalid excluded pattern %q: %w", p, err)
		}
	}
	return nil
}

func globToRegexp(pattern string) (*regexp.Regexp, error) {
	quoted := regexp.QuoteMeta(pattern)
	regexStr := "^" + strings.ReplaceAll(strings.ReplaceAll(quoted, `\*`, `.*`), `\?`, `.`) + "$"
	return regexp.Compile(regexStr)
}

type wrapperMetricsExporter struct {
	exporter.Metrics
	excludeServiceAttributes bool
	constantLabelsMatcher    *resourceAttributeMatcher
}

type resourceAttributeMatcher struct {
	included []*regexp.Regexp
	excluded []*regexp.Regexp
}

func newResourceAttributeMatcher(set Settings) *resourceAttributeMatcher {
	m := &resourceAttributeMatcher{
		included: make([]*regexp.Regexp, 0, len(set.Included)),
		excluded: make([]*regexp.Regexp, 0, len(set.Excluded)),
	}
	for _, p := range set.Included {
		if re, err := globToRegexp(p); err == nil {
			m.included = append(m.included, re)
		}
	}
	for _, p := range set.Excluded {
		if re, err := globToRegexp(p); err == nil {
			m.excluded = append(m.excluded, re)
		}
	}
	return m
}

func (m *resourceAttributeMatcher) matches(key string) bool {
	for _, re := range m.excluded {
		if re.MatchString(key) {
			return false
		}
	}
	if len(m.included) == 0 {
		return len(m.excluded) > 0
	}
	for _, re := range m.included {
		if re.MatchString(key) {
			return true
		}
	}
	return false
}

func (wme *wrapperMetricsExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return wme.Metrics.ConsumeMetrics(ctx, wme.convertToMetricsAttributes(md))
}

func (*wrapperMetricsExporter) Capabilities() consumer.Capabilities {
	// Always return true since this wrapper modifies data inplace.
	return consumer.Capabilities{MutatesData: true}
}

// WrapMetricsExporter wraps a given exporter.Metrics and based on the given settings
// converts incoming resource attributes to metrics attributes or constant labels.
func WrapMetricsExporter(set Settings, exporter exporter.Metrics) exporter.Metrics {
	if len(set.Included) > 0 || len(set.Excluded) > 0 {
		return &wrapperMetricsExporter{
			Metrics:               exporter,
			constantLabelsMatcher: newResourceAttributeMatcher(set),
		}
	}
	if !set.Enabled {
		return exporter
	}
	return &wrapperMetricsExporter{
		Metrics:                  exporter,
		excludeServiceAttributes: set.ExcludeServiceAttributes,
	}
}

func (wme *wrapperMetricsExporter) convertToMetricsAttributes(md pmetric.Metrics) pmetric.Metrics {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		resourceAttrs := rms.At(i).Resource().Attributes()

		// Filter resource attributes based on allowed attributes or excludeServiceAttributes setting
		var attrsToAdd pcommon.Map
		switch {
		case wme.constantLabelsMatcher != nil:
			attrsToAdd = filterMatchingAttributes(resourceAttrs, wme.constantLabelsMatcher)
		case wme.excludeServiceAttributes:
			attrsToAdd = filterServiceAttributes(resourceAttrs)
		default:
			attrsToAdd = resourceAttrs
		}

		ilms := rms.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)
			metricSlice := ilm.Metrics()
			for k := 0; k < metricSlice.Len(); k++ {
				addAttributesToMetric(metricSlice.At(k), attrsToAdd)
			}
		}
	}
	return md
}

func filterMatchingAttributes(attrs pcommon.Map, matcher *resourceAttributeMatcher) pcommon.Map {
	filtered := pcommon.NewMap()
	filtered.EnsureCapacity(attrs.Len())
	for k, v := range attrs.All() {
		if matcher.matches(k) {
			v.CopyTo(filtered.PutEmpty(k))
		}
	}
	return filtered
}

// filterServiceAttributes returns a new Map without service.name and service.instance.id attributes.
func filterServiceAttributes(attrs pcommon.Map) pcommon.Map {
	filtered := pcommon.NewMap()
	filtered.EnsureCapacity(attrs.Len())
	for k, v := range attrs.All() {
		if shouldSkipResourceAttributeKey(k) {
			continue
		}
		v.CopyTo(filtered.PutEmpty(k))
	}
	return filtered
}

func shouldSkipResourceAttributeKey(k string) bool {
	switch k {
	case string(conventions.ServiceNameKey),
		string(conventions.ServiceInstanceIDKey),
		string(conventions.ServiceNamespaceKey):
		return true
	default:
		return false
	}
}

// addAttributesToMetric adds additional labels to the given metric
func addAttributesToMetric(metric pmetric.Metric, labelMap pcommon.Map) {
	//exhaustive:enforce
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		addAttributesToNumberDataPoints(metric.Gauge().DataPoints(), labelMap)
	case pmetric.MetricTypeSum:
		addAttributesToNumberDataPoints(metric.Sum().DataPoints(), labelMap)
	case pmetric.MetricTypeHistogram:
		addAttributesToHistogramDataPoints(metric.Histogram().DataPoints(), labelMap)
	case pmetric.MetricTypeSummary:
		addAttributesToSummaryDataPoints(metric.Summary().DataPoints(), labelMap)
	case pmetric.MetricTypeExponentialHistogram:
		addAttributesToExponentialHistogramDataPoints(metric.ExponentialHistogram().DataPoints(), labelMap)
	}
}

func addAttributesToNumberDataPoints(ps pmetric.NumberDataPointSlice, newAttributeMap pcommon.Map) {
	for i := 0; i < ps.Len(); i++ {
		joinAttributeMaps(newAttributeMap, ps.At(i).Attributes())
	}
}

func addAttributesToHistogramDataPoints(ps pmetric.HistogramDataPointSlice, newAttributeMap pcommon.Map) {
	for i := 0; i < ps.Len(); i++ {
		joinAttributeMaps(newAttributeMap, ps.At(i).Attributes())
	}
}

func addAttributesToSummaryDataPoints(ps pmetric.SummaryDataPointSlice, newAttributeMap pcommon.Map) {
	for i := 0; i < ps.Len(); i++ {
		joinAttributeMaps(newAttributeMap, ps.At(i).Attributes())
	}
}

func addAttributesToExponentialHistogramDataPoints(ps pmetric.ExponentialHistogramDataPointSlice, newAttributeMap pcommon.Map) {
	for i := 0; i < ps.Len(); i++ {
		joinAttributeMaps(newAttributeMap, ps.At(i).Attributes())
	}
}

func joinAttributeMaps(from, to pcommon.Map) {
	to.EnsureCapacity(from.Len() + to.Len())
	for k, v := range from.All() {
		v.CopyTo(to.PutEmpty(k))
	}
}
