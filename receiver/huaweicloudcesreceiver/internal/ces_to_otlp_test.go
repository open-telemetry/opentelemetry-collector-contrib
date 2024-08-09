package internal

import (
	"testing"
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ces/v1/model"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func stringPtr(s string) *string {
	return &s
}

func float64Ptr(f float64) *float64 {
	return &f
}
func TestConvertCESMetricsToOTLP(t *testing.T) {
	tests := []struct {
		name     string
		input    []model.BatchMetricData
		expected func() pmetric.Metrics
	}{
		{
			name: "Valid Metric Conversion",
			input: []model.BatchMetricData{
				{
					Namespace:  stringPtr("SYS.ECS"),
					MetricName: "cpu_util",
					Dimensions: &[]model.MetricsDimension{
						{
							Name:  "instance_id",
							Value: "faea5b75-e390-4e2b-8733-9226a9026070",
						},
					},
					Datapoints: []model.DatapointForBatchMetric{
						{
							Average:   float64Ptr(0.5),
							Timestamp: 1556625610000,
						},
						{
							Average:   float64Ptr(0.7),
							Timestamp: 1556625715000,
						},
					},
					Unit: stringPtr("%"),
				},
				{
					Namespace:  stringPtr("SYS.ECS"),
					MetricName: "network_vm_connections",
					Dimensions: &[]model.MetricsDimension{
						{
							Name:  "instance_id",
							Value: "06b4020f-461a-4a52-84da-53fa71c2f42b",
						},
					},
					Datapoints: []model.DatapointForBatchMetric{
						{
							Average:   float64Ptr(1),
							Timestamp: 1556625612000,
						},
						{
							Average:   float64Ptr(3),
							Timestamp: 1556625717000,
						},
					},
					Unit: stringPtr("count"),
				},
			},
			expected: func() pmetric.Metrics {
				metrics := pmetric.NewMetrics()
				resourceMetric := metrics.ResourceMetrics().AppendEmpty()

				resource := resourceMetric.Resource()
				resource.Attributes().PutStr("cloud.provider", "huawei_cloud")
				resource.Attributes().PutStr("project.id", "project_1")
				resource.Attributes().PutStr("region", "eu-west-101")

				scopedMetric := resourceMetric.ScopeMetrics().AppendEmpty()
				scopedMetric.Scope().SetName("huawei_cloud_ces")
				scopedMetric.Scope().SetVersion("v1")

				metric := scopedMetric.Metrics().AppendEmpty()
				metric.SetName("cpu_util")
				metric.SetUnit("%")
				metric.Metadata().PutStr("instance_id", "faea5b75-e390-4e2b-8733-9226a9026070")
				metric.Metadata().PutStr("service.namespace", "SYS.ECS")

				dataPoints := metric.SetEmptyGauge().DataPoints()
				dp := dataPoints.AppendEmpty()
				dp.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(1556625610000)))
				dp.SetDoubleValue(0.5)

				dp = dataPoints.AppendEmpty()
				dp.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(1556625715000)))
				dp.SetDoubleValue(0.7)

				scopedMetric = resourceMetric.ScopeMetrics().AppendEmpty()
				scopedMetric.Scope().SetName("huawei_cloud_ces")
				scopedMetric.Scope().SetVersion("v1")

				metric = scopedMetric.Metrics().AppendEmpty()
				metric.SetName("network_vm_connections")
				metric.SetUnit("count")
				metric.Metadata().PutStr("instance_id", "06b4020f-461a-4a52-84da-53fa71c2f42b")
				metric.Metadata().PutStr("service.namespace", "SYS.ECS")

				dataPoints = metric.SetEmptyGauge().DataPoints()
				dp = dataPoints.AppendEmpty()
				dp.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(1556625612000)))
				dp.SetDoubleValue(1)

				dp = dataPoints.AppendEmpty()
				dp.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(1556625717000)))
				dp.SetDoubleValue(3)
				return metrics
			},
		},
		{
			name:  "Empty Metric Data",
			input: []model.BatchMetricData{},
			expected: func() pmetric.Metrics {
				return pmetric.NewMetrics()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected(), ConvertCESMetricsToOTLP("project_1", "eu-west-101", "average", tt.input))
		})
	}
}
