// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudcesreceiver/internal"

import (
	"fmt"
	"strings"
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ces/v1/model"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type MetricData struct {
	MetricName string
	Dimensions []model.MetricsDimension
	Namespace  string
	Unit       string
	Datapoints []model.Datapoint
}

func GetMetricKey(m model.MetricInfoList) string {
	strArray := make([]string, len(m.Dimensions))
	for i, ms := range m.Dimensions {
		strArray[i] = ms.String()
	}
	return fmt.Sprintf("metric_name=%s,dimensions=%s", m.MetricName, strings.Join(strArray, " "))
}

func GetDimension(dimensions []model.MetricsDimension, index int) *string {
	if len(dimensions) > index {
		dimValue := dimensions[index].Name + "," + dimensions[index].Value
		return &dimValue
	}
	return nil
}

func ConvertCESMetricsToOTLP(projectID, regionID, filter string, cesMetrics map[string]MetricData) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	if len(cesMetrics) == 0 {
		return metrics
	}
	resourceMetric := metrics.ResourceMetrics().AppendEmpty()

	resource := resourceMetric.Resource()
	resource.Attributes().PutStr("cloud.provider", "huawei_cloud")
	resource.Attributes().PutStr("project.id", projectID)
	resource.Attributes().PutStr("region.id", regionID)

	for _, cesMetric := range cesMetrics {
		scopedMetric := resourceMetric.ScopeMetrics().AppendEmpty()
		scopedMetric.Scope().SetName("huawei_cloud_ces")
		scopedMetric.Scope().SetVersion("v1")

		metric := scopedMetric.Metrics().AppendEmpty()
		metric.SetName(cesMetric.MetricName)
		metric.SetUnit(cesMetric.Unit)
		for _, dimension := range cesMetric.Dimensions {
			metric.Metadata().PutStr(dimension.Name, dimension.Value)
		}

		metric.Metadata().PutStr("service.namespace", cesMetric.Namespace)

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
