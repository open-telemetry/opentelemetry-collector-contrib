// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcetotelemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// Settings defines configuration for converting resource attributes to telemetry attributes.
// When used, it must be embedded in the exporter configuration:
//
//	type Config struct {
//	  // ...
//	  resourcetotelemetry.Settings `mapstructure:"resource_to_telemetry_conversion"`
//	}
type Settings struct {
	// Enabled indicates whether to convert resource attributes to telemetry attributes. Default is `false`.
	Enabled bool `mapstructure:"enabled"`
	// ExcludeServiceInfo indicates whether to exclude `service.name` and `service.instance.id`
	// resource attributes from being converted to metric attributes. Default is `false`.
	// When set to `true`, these attributes will not be added to metric labels since they are
	// already mapped to Prometheus `job` and `instance` labels respectively.
	ExcludeServiceInfo bool `mapstructure:"exclude_service_info"`
}

// serviceNameKey and serviceInstanceIDKey are resource attribute keys for service identification.
// These are excluded from metric attributes when ExcludeServiceInfo is true.
const (
	serviceNameKey       = "service.name"
	serviceInstanceIDKey = "service.instance.id"
)

type wrapperMetricsExporter struct {
	exporter.Metrics
	excludeServiceInfo bool
}

func (wme *wrapperMetricsExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return wme.Metrics.ConsumeMetrics(ctx, wme.convertToMetricsAttributes(md))
}

func (*wrapperMetricsExporter) Capabilities() consumer.Capabilities {
	// Always return true since this wrapper modifies data inplace.
	return consumer.Capabilities{MutatesData: true}
}

// WrapMetricsExporter wraps a given exporter.Metrics and based on the given settings
// converts incoming resource attributes to metrics attributes.
func WrapMetricsExporter(set Settings, exporter exporter.Metrics) exporter.Metrics {
	if !set.Enabled {
		return exporter
	}
	return &wrapperMetricsExporter{
		Metrics:            exporter,
		excludeServiceInfo: set.ExcludeServiceInfo,
	}
}

func (wme *wrapperMetricsExporter) convertToMetricsAttributes(md pmetric.Metrics) pmetric.Metrics {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		resourceAttrs := rms.At(i).Resource().Attributes()

		// Filter resource attributes if excludeServiceInfo is enabled
		attrsToAdd := resourceAttrs
		if wme.excludeServiceInfo {
			attrsToAdd = filterServiceAttributes(resourceAttrs)
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

// filterServiceAttributes returns a new Map without service.name and service.instance.id attributes.
func filterServiceAttributes(attrs pcommon.Map) pcommon.Map {
	filtered := pcommon.NewMap()
	filtered.EnsureCapacity(attrs.Len())
	for k, v := range attrs.All() {
		if k == serviceNameKey || k == serviceInstanceIDKey {
			continue
		}
		v.CopyTo(filtered.PutEmpty(k))
	}
	return filtered
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
