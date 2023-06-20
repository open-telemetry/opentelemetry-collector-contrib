package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

import (
	"github.com/prometheus/prometheus/prompb"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// CreateMetadata converts pmetric.Metrics to prometheus remote write format.
func CreateMetadata(md pmetric.Metrics) []prompb.MetricMetadata {
	var metadata []prompb.MetricMetadata

	resourceMetricsSlice := md.ResourceMetrics()
	for i := 0; i < resourceMetricsSlice.Len(); i++ {
		resourceMetrics := resourceMetricsSlice.At(i)
		scopeMetricsSlice := resourceMetrics.ScopeMetrics()

		for j := 0; j < scopeMetricsSlice.Len(); j++ {
			scopeMetrics := scopeMetricsSlice.At(j)
			for k := 0; k < scopeMetrics.Metrics().Len(); k++ {

				entry := prompb.MetricMetadata{
					Type:             prompb.MetricMetadata_MetricType(scopeMetrics.Metrics().At(k).Type()),
					MetricFamilyName: scopeMetrics.Metrics().At(k).Name(),
					Help:             scopeMetrics.Metrics().At(k).Description(),
					Unit:             scopeMetrics.Metrics().At(k).Unit(),
				}
				metadata = append(metadata, entry)
			}
		}
	}

	return metadata
}
