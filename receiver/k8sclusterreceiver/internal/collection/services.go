package collection

import (
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/utils"
)

func getMetricsForService(svc *corev1.Service) []*resourceMetrics {
	metrics := []*metricspb.Metric{
		{
			MetricDescriptor: &metricspb.MetricDescriptor{
				Name:        "k8s.service.port_count",
				Description: "The number of ports in the service",
				Type:        metricspb.MetricDescriptor_GAUGE_INT64,
			},
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(len(svc.Spec.Ports))),
			},
		},
	}

	return []*resourceMetrics{
		{
			resource: getResourceForService(svc),
			metrics:  metrics,
		},
	}
}

func getResourceForService(svc *corev1.Service) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: k8sType,
		Labels: map[string]string{
			k8sKeyServiceUID:                      string(svc.UID),
			conventions.AttributeServiceNamespace: svc.ObjectMeta.Namespace,
			conventions.AttributeServiceName:      svc.ObjectMeta.Name,
			"k8s.service.cluster_ip":              svc.Spec.ClusterIP,
			"k8s.service.type":                    string(svc.Spec.Type),
			"k8s.cluster.name":                    "unknown",
		},
	}
}
