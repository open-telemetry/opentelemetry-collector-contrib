// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ces/v1/model"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func ConvertCESMetricsToOTLP(projectID, region, filter string, cesMetrics []model.BatchMetricData) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	if len(cesMetrics) == 0 {
		return metrics
	}
	resourceMetric := metrics.ResourceMetrics().AppendEmpty()

	resource := resourceMetric.Resource()
	resource.Attributes().PutStr("cloud.provider", "huawei_cloud")
	resource.Attributes().PutStr("project.id", projectID)
	resource.Attributes().PutStr("region", region)

	for _, cesMetric := range cesMetrics {
		scopedMetric := resourceMetric.ScopeMetrics().AppendEmpty()
		scopedMetric.Scope().SetName("huawei_cloud_ces")
		scopedMetric.Scope().SetVersion("v1")

		metric := scopedMetric.Metrics().AppendEmpty()
		metric.SetName(cesMetric.MetricName)
		if cesMetric.Unit != nil {
			metric.SetUnit(*cesMetric.Unit)
		}
		if cesMetric.Dimensions != nil {
			for _, dimension := range *cesMetric.Dimensions {
				metric.Metadata().PutStr(dimension.Name, dimension.Value)
			}
		}
		if cesMetric.Namespace != nil {
			metric.Metadata().PutStr("service.namespace", *cesMetric.Namespace)
		}
		dataPoints := metric.SetEmptyGauge().DataPoints()
		for _, dataPoint := range cesMetric.Datapoints {
			dp := dataPoints.AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(dataPoint.Timestamp)))
			switch filter {
			case "max":
				dp.SetDoubleValue(*dataPoint.Max)
			case "min":
				dp.SetDoubleValue(*dataPoint.Min)
			case "average":
				dp.SetDoubleValue(*dataPoint.Average)
			case "sum":
				dp.SetDoubleValue(*dataPoint.Sum)
			case "variance":
				dp.SetDoubleValue(*dataPoint.Variance)
			}
		}
	}

	return metrics
}
