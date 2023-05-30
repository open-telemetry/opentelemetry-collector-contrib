// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hpa // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/hpa"

import (
	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	autoscalingv2beta2 "k8s.io/api/autoscaling/v2beta2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/utils"
)

var hpaMaxReplicasMetric = &metricspb.MetricDescriptor{
	Name:        "k8s.hpa.max_replicas",
	Description: "Maximum number of replicas to which the autoscaler can scale up",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var hpaMinReplicasMetric = &metricspb.MetricDescriptor{
	Name:        "k8s.hpa.min_replicas",
	Description: "Minimum number of replicas to which the autoscaler can scale down",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var hpaCurrentReplicasMetric = &metricspb.MetricDescriptor{
	Name:        "k8s.hpa.current_replicas",
	Description: "Current number of pod replicas managed by this autoscaler",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var hpaDesiredReplicasMetric = &metricspb.MetricDescriptor{
	Name:        "k8s.hpa.desired_replicas",
	Description: "Desired number of pod replicas managed by this autoscaler",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

func GetMetrics(hpa *autoscalingv2.HorizontalPodAutoscaler) []*agentmetricspb.ExportMetricsServiceRequest {
	metrics := []*metricspb.Metric{
		{
			MetricDescriptor: hpaMaxReplicasMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(hpa.Spec.MaxReplicas)),
			},
		},
		{
			MetricDescriptor: hpaMinReplicasMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(*hpa.Spec.MinReplicas)),
			},
		},
		{
			MetricDescriptor: hpaCurrentReplicasMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(hpa.Status.CurrentReplicas)),
			},
		},
		{
			MetricDescriptor: hpaDesiredReplicasMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(hpa.Status.DesiredReplicas)),
			},
		},
	}

	return []*agentmetricspb.ExportMetricsServiceRequest{
		{
			Resource: getResourceForHPA(&hpa.ObjectMeta),
			Metrics:  metrics,
		},
	}
}

func GetMetricsBeta(hpa *autoscalingv2beta2.HorizontalPodAutoscaler) []*agentmetricspb.ExportMetricsServiceRequest {
	metrics := []*metricspb.Metric{
		{
			MetricDescriptor: hpaMaxReplicasMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(hpa.Spec.MaxReplicas)),
			},
		},
		{
			MetricDescriptor: hpaMinReplicasMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(*hpa.Spec.MinReplicas)),
			},
		},
		{
			MetricDescriptor: hpaCurrentReplicasMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(hpa.Status.CurrentReplicas)),
			},
		},
		{
			MetricDescriptor: hpaDesiredReplicasMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(hpa.Status.DesiredReplicas)),
			},
		},
	}

	return []*agentmetricspb.ExportMetricsServiceRequest{
		{
			Resource: getResourceForHPA(&hpa.ObjectMeta),
			Metrics:  metrics,
		},
	}
}

func getResourceForHPA(om *v1.ObjectMeta) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: constants.K8sType,
		Labels: map[string]string{
			constants.K8sKeyHPAUID:                string(om.UID),
			constants.K8sKeyHPAName:               om.Name,
			conventions.AttributeK8SNamespaceName: om.Namespace,
		},
	}
}

func GetMetadata(hpa *autoscalingv2.HorizontalPodAutoscaler) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		experimentalmetricmetadata.ResourceID(hpa.UID): metadata.GetGenericMetadata(&hpa.ObjectMeta, "HPA"),
	}
}

func GetMetadataBeta(hpa *autoscalingv2beta2.HorizontalPodAutoscaler) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		experimentalmetricmetadata.ResourceID(hpa.UID): metadata.GetGenericMetadata(&hpa.ObjectMeta, "HPA"),
	}
}
