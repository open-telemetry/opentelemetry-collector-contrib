// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcequota // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/resourcequota"

import (
	"strings"

	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/utils"
)

var resourceQuotaHardLimitMetric = &metricspb.MetricDescriptor{
	Name: "k8s.resource_quota.hard_limit",
	Description: "The upper limit for a particular resource in a specific namespace." +
		" Will only be sent if a quota is specified. CPU requests/limits will be sent as millicores",
	Type: metricspb.MetricDescriptor_GAUGE_INT64,
	LabelKeys: []*metricspb.LabelKey{{
		Key: "resource",
	}},
}

var resourceQuotaUsedMetric = &metricspb.MetricDescriptor{
	Name: "k8s.resource_quota.used",
	Description: "The usage for a particular resource in a specific namespace." +
		" Will only be sent if a quota is specified. CPU requests/limits will be sent as millicores",
	Type: metricspb.MetricDescriptor_GAUGE_INT64,
	LabelKeys: []*metricspb.LabelKey{{
		Key: "resource",
	}},
}

func GetMetrics(rq *corev1.ResourceQuota) []*agentmetricspb.ExportMetricsServiceRequest {
	var metrics []*metricspb.Metric

	for _, t := range []struct {
		metric *metricspb.MetricDescriptor
		rl     corev1.ResourceList
	}{
		{
			resourceQuotaHardLimitMetric,
			rq.Status.Hard,
		},
		{
			resourceQuotaUsedMetric,
			rq.Status.Used,
		},
	} {
		for k, v := range t.rl {

			val := v.Value()
			if strings.HasSuffix(string(k), ".cpu") {
				val = v.MilliValue()
			}

			metrics = append(metrics,
				&metricspb.Metric{
					MetricDescriptor: t.metric,
					Timeseries: []*metricspb.TimeSeries{
						utils.GetInt64TimeSeriesWithLabels(val, []*metricspb.LabelValue{{Value: string(k), HasValue: true}}),
					},
				},
			)
		}
	}

	return []*agentmetricspb.ExportMetricsServiceRequest{
		{
			Resource: getResource(rq),
			Metrics:  metrics,
		},
	}
}

func getResource(rq *corev1.ResourceQuota) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: constants.K8sType,
		Labels: map[string]string{
			constants.K8sKeyResourceQuotaUID:      string(rq.UID),
			constants.K8sKeyResourceQuotaName:     rq.Name,
			conventions.AttributeK8SNamespaceName: rq.Namespace,
		},
	}
}
