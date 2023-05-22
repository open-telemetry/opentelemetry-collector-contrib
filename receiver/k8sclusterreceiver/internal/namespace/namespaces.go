// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package namespace // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/namespace"

import (
	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/utils"
)

func GetMetrics(ns *corev1.Namespace) []*agentmetricspb.ExportMetricsServiceRequest {
	metrics := []*metricspb.Metric{
		{
			MetricDescriptor: &metricspb.MetricDescriptor{
				Name:        "k8s.namespace.phase",
				Description: "The current phase of namespaces (1 for active and 0 for terminating)",
				Type:        metricspb.MetricDescriptor_GAUGE_INT64,
			},
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(namespacePhaseValues[ns.Status.Phase])),
			},
		},
	}

	return []*agentmetricspb.ExportMetricsServiceRequest{
		{
			Resource: getResource(ns),
			Metrics:  metrics,
		},
	}
}

func getResource(ns *corev1.Namespace) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: constants.K8sType,
		Labels: map[string]string{
			constants.K8sKeyNamespaceUID:          string(ns.UID),
			conventions.AttributeK8SNamespaceName: ns.Namespace,
		},
	}
}

var namespacePhaseValues = map[corev1.NamespacePhase]int32{
	corev1.NamespaceActive:      1,
	corev1.NamespaceTerminating: 0,
	// If phase is blank for some reason, send as -1 for unknown.
	corev1.NamespacePhase(""): -1,
}
