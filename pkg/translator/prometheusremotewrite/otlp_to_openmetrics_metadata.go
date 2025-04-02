// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

import (
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"go.opentelemetry.io/collector/pdata/pmetric"

	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

func otelMetricTypeToPromMetricType(otelMetric pmetric.Metric) prompb.MetricMetadata_MetricType {
	// metric metadata can be used to support Prometheus types that don't exist
	// in OpenTelemetry.
	typeFromMetadata, hasTypeFromMetadata := otelMetric.Metadata().Get(prometheustranslator.MetricMetadataTypeKey)
	switch otelMetric.Type() {
	case pmetric.MetricTypeGauge:
		if hasTypeFromMetadata && typeFromMetadata.Str() == string(model.MetricTypeUnknown) {
			return prompb.MetricMetadata_UNKNOWN
		}
		return prompb.MetricMetadata_GAUGE
	case pmetric.MetricTypeSum:
		if otelMetric.Sum().IsMonotonic() {
			return prompb.MetricMetadata_COUNTER
		}
		if hasTypeFromMetadata && typeFromMetadata.Str() == string(model.MetricTypeInfo) {
			return prompb.MetricMetadata_INFO
		}
		if hasTypeFromMetadata && typeFromMetadata.Str() == string(model.MetricTypeStateset) {
			return prompb.MetricMetadata_STATESET
		}
		return prompb.MetricMetadata_GAUGE
	case pmetric.MetricTypeHistogram:
		return prompb.MetricMetadata_HISTOGRAM
	case pmetric.MetricTypeSummary:
		return prompb.MetricMetadata_SUMMARY
	case pmetric.MetricTypeExponentialHistogram:
		return prompb.MetricMetadata_HISTOGRAM
	}
	return prompb.MetricMetadata_UNKNOWN
}

func OtelMetricsToMetadata(md pmetric.Metrics, addMetricSuffixes bool) []*prompb.MetricMetadata {
	resourceMetricsSlice := md.ResourceMetrics()

	metadataLength := 0
	for i := 0; i < resourceMetricsSlice.Len(); i++ {
		scopeMetricsSlice := resourceMetricsSlice.At(i).ScopeMetrics()
		for j := 0; j < scopeMetricsSlice.Len(); j++ {
			metadataLength += scopeMetricsSlice.At(j).Metrics().Len()
		}
	}

	metadata := make([]*prompb.MetricMetadata, 0, metadataLength)
	for i := 0; i < resourceMetricsSlice.Len(); i++ {
		resourceMetrics := resourceMetricsSlice.At(i)
		scopeMetricsSlice := resourceMetrics.ScopeMetrics()

		for j := 0; j < scopeMetricsSlice.Len(); j++ {
			scopeMetrics := scopeMetricsSlice.At(j)
			for k := 0; k < scopeMetrics.Metrics().Len(); k++ {
				metric := scopeMetrics.Metrics().At(k)
				entry := prompb.MetricMetadata{
					Type:             otelMetricTypeToPromMetricType(metric),
					MetricFamilyName: prometheustranslator.BuildCompliantName(metric, "", addMetricSuffixes),
					Unit:             prometheustranslator.BuildCompliantPrometheusUnit(metric.Unit()),
					Help:             metric.Description(),
				}
				metadata = append(metadata, &entry)
			}
		}
	}

	return metadata
}
