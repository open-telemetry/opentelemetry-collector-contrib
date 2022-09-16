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
	Enabled           bool     `mapstructure:"enabled"`
	ExcludeAttributes []string `mapstructure:"exclude_attributes"`
}

type wrapperMetricsExporter struct {
	component.MetricsExporter
	ExcludeAttributes map[string]bool
}

func (wme *wrapperMetricsExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return wme.MetricsExporter.ConsumeMetrics(ctx, convertToMetricsAttributes(md, wme.ExcludeAttributes))
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
	return &wrapperMetricsExporter{MetricsExporter: exporter, ExcludeAttributes: sliceToStringSet(set.ExcludeAttributes)}
}

func convertToMetricsAttributes(md pmetric.Metrics, excludeAttributes map[string]bool) pmetric.Metrics {
	cloneMd := md.Clone()
	rms := cloneMd.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		resource := rms.At(i).Resource()

		labelMap := extractLabelsFromResource(&resource, excludeAttributes)

		ilms := rms.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)
			metricSlice := ilm.Metrics()
			for k := 0; k < metricSlice.Len(); k++ {
				addAttributesToMetric(metricSlice.At(k), labelMap)
			}
		}
	}
	return cloneMd
}

// extractAttributesFromResource extracts the attributes from a given resource and
// returns them as a StringMap.
func extractLabelsFromResource(resource *pcommon.Resource, excludeAttributes map[string]bool) pcommon.Map {
	if len(excludeAttributes) == 0 {
		return resource.Attributes()
	}
	labelMap := pcommon.NewMap()
	attributes := resource.Attributes()
	attrMap := attributes.AsRaw()
	for k := range attrMap {
		shouldBeSkipped := excludeAttributes[k]
		if !shouldBeSkipped {
			stringLabel, _ := attributes.Get(k)
			labelMap.UpsertString(k, stringLabel.AsString())
		}
	}
	return labelMap
}

// addAttributesToMetric adds additional labels to the given metric
func addAttributesToMetric(metric pmetric.Metric, labelMap pcommon.Map) {
	switch metric.DataType() {
	case pmetric.MetricDataTypeGauge:
		addAttributesToNumberDataPoints(metric.Gauge().DataPoints(), labelMap)
	case pmetric.MetricDataTypeSum:
		addAttributesToNumberDataPoints(metric.Sum().DataPoints(), labelMap)
	case pmetric.MetricDataTypeHistogram:
		addAttributesToHistogramDataPoints(metric.Histogram().DataPoints(), labelMap)
	case pmetric.MetricDataTypeSummary:
		addAttributesToSummaryDataPoints(metric.Summary().DataPoints(), labelMap)
	case pmetric.MetricDataTypeExponentialHistogram:
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
	from.Range(func(k string, v pcommon.Value) bool {
		v.CopyTo(to.PutEmpty(k))
		return true
	})
}

// sliceToSet converts slice of strings to set of strings
// Returns the set of strings
func sliceToStringSet(slice []string) map[string]bool {
	set := make(map[string]bool, len(slice))
	for _, s := range slice {
		set[s] = true
	}
	return set
}
