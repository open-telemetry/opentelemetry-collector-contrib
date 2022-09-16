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

package resourcetotelemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filtermatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
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
	// Exclude indicates which of resource attributes should be omitted during conversion.
	Exclude MatchAttributes `mapstructure:"exclude"`
}

// MatchAttributes defines the filter list of attributes to be omitted and
type MatchAttributes struct {
	// Filter indicates the type of matching for attributes
	Filter filterset.Config `mapstructure:",squash"`
	// Exclude indicates the list of attributes to exclusion
	Exclude []filterconfig.Attribute `mapstructure:"attributes"`
}

type wrapperMetricsExporter struct {
	component.MetricsExporter
	MatchAttributes
}

func (wme *wrapperMetricsExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	metrics, err := convertToMetricsAttributes(md, wme.MatchAttributes)
	if err != nil {
		return err
	}
	return wme.MetricsExporter.ConsumeMetrics(ctx, metrics)
}

func (wme *wrapperMetricsExporter) Capabilities() consumer.Capabilities {
	// Always return false since this wrapper clones the data.
	return consumer.Capabilities{MutatesData: false}
}

// WrapMetricsExporter wraps a given component.MetricsExporter and based on the given settings
// converts incoming resource attributes to metrics attributes.
func WrapMetricsExporter(set Settings, exporter component.MetricsExporter) component.MetricsExporter {
	if !set.Enabled {
		return exporter
	}
	return &wrapperMetricsExporter{MetricsExporter: exporter, MatchAttributes: set.Exclude}
}

func convertToMetricsAttributes(md pmetric.Metrics, filter MatchAttributes) (pmetric.Metrics, error) {
	cloneMd := pmetric.NewMetrics()
	md.CopyTo(cloneMd)
	rms := cloneMd.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		resource := rms.At(i).Resource()

		attrMatcher, err := filtermatcher.NewAttributesMatcher(filter.Filter, filter.Exclude)
		if err != nil {
			return cloneMd, err
		}

		ilms := rms.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)
			metricSlice := ilm.Metrics()
			for k := 0; k < metricSlice.Len(); k++ {
				addAttributesToMetric(metricSlice.At(k), resource.Attributes(), attrMatcher)
			}
		}
	}
	return cloneMd, nil
}

// addAttributesToMetric adds additional labels to the given metric
func addAttributesToMetric(metric pmetric.Metric, labelMap pcommon.Map, matcher filtermatcher.AttributesMatcher) {
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		addAttributesToNumberDataPoints(metric.Gauge().DataPoints(), labelMap, matcher)
	case pmetric.MetricTypeSum:
		addAttributesToNumberDataPoints(metric.Sum().DataPoints(), labelMap, matcher)
	case pmetric.MetricTypeHistogram:
		addAttributesToHistogramDataPoints(metric.Histogram().DataPoints(), labelMap, matcher)
	case pmetric.MetricTypeSummary:
		addAttributesToSummaryDataPoints(metric.Summary().DataPoints(), labelMap, matcher)
	case pmetric.MetricTypeExponentialHistogram:
		addAttributesToExponentialHistogramDataPoints(metric.ExponentialHistogram().DataPoints(), labelMap, matcher)
	}
}

func addAttributesToNumberDataPoints(ps pmetric.NumberDataPointSlice, newAttributeMap pcommon.Map, matcher filtermatcher.AttributesMatcher) {
	for i := 0; i < ps.Len(); i++ {
		joinAttributeMaps(newAttributeMap, ps.At(i).Attributes(), matcher)
	}
}

func addAttributesToHistogramDataPoints(ps pmetric.HistogramDataPointSlice, newAttributeMap pcommon.Map, matcher filtermatcher.AttributesMatcher) {
	for i := 0; i < ps.Len(); i++ {
		joinAttributeMaps(newAttributeMap, ps.At(i).Attributes(), matcher)
	}
}

func addAttributesToSummaryDataPoints(ps pmetric.SummaryDataPointSlice, newAttributeMap pcommon.Map, matcher filtermatcher.AttributesMatcher) {
	for i := 0; i < ps.Len(); i++ {
		joinAttributeMaps(newAttributeMap, ps.At(i).Attributes(), matcher)
	}
}

func addAttributesToExponentialHistogramDataPoints(ps pmetric.ExponentialHistogramDataPointSlice, newAttributeMap pcommon.Map, matcher filtermatcher.AttributesMatcher) {
	for i := 0; i < ps.Len(); i++ {
		joinAttributeMaps(newAttributeMap, ps.At(i).Attributes(), matcher)
	}
}

func joinAttributeMaps(from, to pcommon.Map, matcher filtermatcher.AttributesMatcher) {
	from.Range(func(k string, v pcommon.Value) bool {
		if !matcher.MatchAny(k, v) {
			v.CopyTo(to.PutEmpty(k))
		}
		return true
	})
}
