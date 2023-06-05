// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clusterresourcequota // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/clusterresourcequota"

import (
	"strings"

	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	quotav1 "github.com/openshift/api/quota/v1"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/utils"
)

var clusterResourceQuotaLimitMetric = &metricspb.MetricDescriptor{
	Name:        "openshift.clusterquota.limit",
	Description: "The configured upper limit for a particular resource.",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
	LabelKeys: []*metricspb.LabelKey{{
		Key: "resource",
	}},
}

var clusterResourceQuotaUsedMetric = &metricspb.MetricDescriptor{
	Name:        "openshift.clusterquota.used",
	Description: "The usage for a particular resource with a configured limit.",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
	LabelKeys: []*metricspb.LabelKey{{
		Key: "resource",
	}},
}

var appliedClusterResourceQuotaLimitMetric = &metricspb.MetricDescriptor{
	Name:        "openshift.appliedclusterquota.limit",
	Description: "The upper limit for a particular resource in a specific namespace.",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
	LabelKeys: []*metricspb.LabelKey{
		{
			Key: "resource",
		},
		{
			Key: conventions.AttributeK8SNamespaceName,
		},
	},
}

var appliedClusterResourceQuotaUsedMetric = &metricspb.MetricDescriptor{
	Name:        "openshift.appliedclusterquota.used",
	Description: "The usage for a particular resource in a specific namespace.",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
	LabelKeys: []*metricspb.LabelKey{
		{
			Key: "resource",
		},
		{
			Key: conventions.AttributeK8SNamespaceName,
		},
	},
}

func GetMetrics(rq *quotav1.ClusterResourceQuota) []*agentmetricspb.ExportMetricsServiceRequest {
	var metrics []*metricspb.Metric

	metrics = appendClusterQuotaMetrics(metrics, clusterResourceQuotaLimitMetric, rq.Status.Total.Hard, "")
	metrics = appendClusterQuotaMetrics(metrics, clusterResourceQuotaUsedMetric, rq.Status.Total.Used, "")
	for _, ns := range rq.Status.Namespaces {
		metrics = appendClusterQuotaMetrics(metrics, appliedClusterResourceQuotaLimitMetric, ns.Status.Hard, ns.Namespace)
		metrics = appendClusterQuotaMetrics(metrics, appliedClusterResourceQuotaUsedMetric, ns.Status.Used, ns.Namespace)
	}
	return []*agentmetricspb.ExportMetricsServiceRequest{
		{
			Resource: getResource(rq),
			Metrics:  metrics,
		},
	}
}

func appendClusterQuotaMetrics(metrics []*metricspb.Metric, metric *metricspb.MetricDescriptor, rl corev1.ResourceList, namespace string) []*metricspb.Metric {
	for k, v := range rl {
		val := v.Value()
		if strings.HasSuffix(string(k), ".cpu") {
			val = v.MilliValue()
		}

		labels := []*metricspb.LabelValue{{Value: string(k), HasValue: true}}
		if namespace != "" {
			labels = append(labels, &metricspb.LabelValue{Value: namespace, HasValue: true})
		}
		metrics = append(metrics,
			&metricspb.Metric{
				MetricDescriptor: metric,
				Timeseries: []*metricspb.TimeSeries{
					utils.GetInt64TimeSeriesWithLabels(val, labels),
				},
			},
		)
	}
	return metrics
}

func getResource(rq *quotav1.ClusterResourceQuota) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: constants.K8sType,
		Labels: map[string]string{
			constants.K8sKeyClusterResourceQuotaUID:  string(rq.UID),
			constants.K8sKeyClusterResourceQuotaName: rq.Name,
		},
	}
}
