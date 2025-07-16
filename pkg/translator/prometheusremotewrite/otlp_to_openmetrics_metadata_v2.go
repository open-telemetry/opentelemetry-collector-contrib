// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

import (
	"github.com/prometheus/common/model"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"go.opentelemetry.io/collector/pdata/pmetric"

	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

func otelMetricTypeToPromMetricTypeV2(otelMetric pmetric.Metric) writev2.Metadata_MetricType {
	// metric metadata can be used to support Prometheus types that don't exist
	// in OpenTelemetry.
	typeFromMetadata, hasTypeFromMetadata := otelMetric.Metadata().Get(prometheustranslator.MetricMetadataTypeKey)
	switch otelMetric.Type() {
	case pmetric.MetricTypeGauge:
		if hasTypeFromMetadata && typeFromMetadata.Str() == string(model.MetricTypeUnknown) {
			return writev2.Metadata_METRIC_TYPE_UNSPECIFIED
		}
		return writev2.Metadata_METRIC_TYPE_GAUGE
	case pmetric.MetricTypeSum:
		if otelMetric.Sum().IsMonotonic() {
			return writev2.Metadata_METRIC_TYPE_COUNTER
		}
		if hasTypeFromMetadata && typeFromMetadata.Str() == string(model.MetricTypeInfo) {
			return writev2.Metadata_METRIC_TYPE_INFO
		}
		if hasTypeFromMetadata && typeFromMetadata.Str() == string(model.MetricTypeStateset) {
			return writev2.Metadata_METRIC_TYPE_STATESET
		}
		return writev2.Metadata_METRIC_TYPE_GAUGE
	case pmetric.MetricTypeHistogram:
		return writev2.Metadata_METRIC_TYPE_HISTOGRAM
	case pmetric.MetricTypeSummary:
		return writev2.Metadata_METRIC_TYPE_SUMMARY
	case pmetric.MetricTypeExponentialHistogram:
		return writev2.Metadata_METRIC_TYPE_HISTOGRAM
	}
	return writev2.Metadata_METRIC_TYPE_UNSPECIFIED
}
