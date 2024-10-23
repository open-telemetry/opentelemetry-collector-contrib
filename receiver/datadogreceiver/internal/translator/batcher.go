// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

type batcher struct {
	pmetric.Metrics

	resourceMetrics map[identity.Resource]pmetric.ResourceMetrics
	scopeMetrics    map[identity.Scope]pmetric.ScopeMetrics
	metrics         map[identity.Metric]pmetric.Metric
}

func newBatcher() batcher {
	return batcher{
		Metrics:         pmetric.NewMetrics(),
		resourceMetrics: make(map[identity.Resource]pmetric.ResourceMetrics),
		scopeMetrics:    make(map[identity.Scope]pmetric.ScopeMetrics),
		metrics:         make(map[identity.Metric]pmetric.Metric),
	}
}

// Dimensions stores the properties of the series that are needed in order
// to unique identify the series. This is needed in order to batch metrics by
// resource, scope, and datapoint attributes
type dimensions struct {
	name          string
	metricType    pmetric.MetricType
	resourceAttrs pcommon.Map
	scopeAttrs    pcommon.Map
	dpAttrs       pcommon.Map
	buildInfo     string
}

var metricTypeMap = map[string]pmetric.MetricType{
	"count":         pmetric.MetricTypeSum,
	"gauge":         pmetric.MetricTypeGauge,
	"rate":          pmetric.MetricTypeSum,
	"service_check": pmetric.MetricTypeGauge,
	"sketch":        pmetric.MetricTypeExponentialHistogram,
}

func parseSeriesProperties(name string, metricType string, tags []string, host string, version string, stringPool *StringPool) dimensions {
	attrs := tagsToAttributes(tags, host, stringPool)
	return dimensions{
		name:          name,
		metricType:    metricTypeMap[metricType],
		buildInfo:     version,
		resourceAttrs: attrs.resource,
		scopeAttrs:    attrs.scope,
		dpAttrs:       attrs.dp,
	}
}

func (b batcher) Lookup(dim dimensions) (pmetric.Metric, identity.Metric) {
	resource := dim.Resource()
	resourceID := identity.OfResource(resource)
	resourceMetrics, ok := b.resourceMetrics[resourceID]
	if !ok {
		resourceMetrics = b.Metrics.ResourceMetrics().AppendEmpty()
		resource.MoveTo(resourceMetrics.Resource())
		b.resourceMetrics[resourceID] = resourceMetrics
	}

	scope := dim.Scope()
	scopeID := identity.OfScope(resourceID, scope)
	scopeMetrics, ok := b.scopeMetrics[scopeID]
	if !ok {
		scopeMetrics = resourceMetrics.ScopeMetrics().AppendEmpty()
		scope.MoveTo(scopeMetrics.Scope())
		b.scopeMetrics[scopeID] = scopeMetrics
	}

	m := dim.Metric()
	metricID := identity.OfMetric(scopeID, m)
	metric, ok := b.metrics[metricID]
	if !ok {
		metric = scopeMetrics.Metrics().AppendEmpty()
		m.MoveTo(metric)
		b.metrics[metricID] = metric
	}

	return metric, metricID
}

func (d dimensions) Resource() pcommon.Resource {
	resource := pcommon.NewResource()
	d.resourceAttrs.CopyTo(resource.Attributes()) // TODO(jesus.vazquez) review this copy
	return resource
}

func (d dimensions) Scope() pcommon.InstrumentationScope {
	scope := pcommon.NewInstrumentationScope()
	scope.SetName("github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator")
	scope.SetVersion(d.buildInfo)
	d.scopeAttrs.CopyTo(scope.Attributes())
	return scope
}

func (d dimensions) Metric() pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(d.name)
	switch d.metricType {
	case pmetric.MetricTypeSum:
		metric.SetEmptySum()
	case pmetric.MetricTypeGauge:
		metric.SetEmptyGauge()
	case pmetric.MetricTypeExponentialHistogram:
		metric.SetEmptyExponentialHistogram()
	}
	return metric
}
