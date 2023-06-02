// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package node // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/node"

import (
	"fmt"
	"time"

	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/iancoleman/strcase"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/maps"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/utils"
)

const (
	// Keys for node metadata.
	nodeCreationTime = "node.creation_timestamp"
)

var allocatableDesciption = map[string]string{
	"cpu":               "How many CPU cores remaining that the node can allocate to pods",
	"memory":            "How many bytes of RAM memory remaining that the node can allocate to pods",
	"ephemeral-storage": "How many bytes of ephemeral storage remaining that the node can allocate to pods",
	"storage":           "How many bytes of storage remaining that the node can allocate to pods",
}

func GetMetrics(node *corev1.Node, nodeConditionTypesToReport, allocatableTypesToReport []string, logger *zap.Logger) []*agentmetricspb.ExportMetricsServiceRequest {
	metrics := make([]*metricspb.Metric, 0, len(nodeConditionTypesToReport)+len(allocatableTypesToReport))
	// Adding 'node condition type' metrics
	for _, nodeConditionTypeValue := range nodeConditionTypesToReport {
		nodeConditionMetric := getNodeConditionMetric(nodeConditionTypeValue)
		v1NodeConditionTypeValue := corev1.NodeConditionType(nodeConditionTypeValue)

		metrics = append(metrics, &metricspb.Metric{
			MetricDescriptor: &metricspb.MetricDescriptor{
				Name: nodeConditionMetric,
				Description: fmt.Sprintf("Whether this node is %s (1), "+
					"not %s (0) or in an unknown state (-1)", nodeConditionTypeValue, nodeConditionTypeValue),
				Type: metricspb.MetricDescriptor_GAUGE_INT64,
			},
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(nodeConditionValue(node, v1NodeConditionTypeValue)),
			},
		})
	}

	// Adding 'node allocatable type' metrics
	for _, nodeAllocatableTypeValue := range allocatableTypesToReport {
		nodeAllocatableMetric := getNodeAllocatableMetric(nodeAllocatableTypeValue)
		v1NodeAllocatableTypeValue := corev1.ResourceName(nodeAllocatableTypeValue)
		valType := metricspb.MetricDescriptor_GAUGE_INT64
		quantity, ok := node.Status.Allocatable[v1NodeAllocatableTypeValue]
		if !ok {
			logger.Debug(fmt.Errorf("allocatable type %v not found in node %v", nodeAllocatableTypeValue,
				node.GetName()).Error())
			continue
		}
		val := utils.GetInt64TimeSeries(quantity.Value())
		if v1NodeAllocatableTypeValue == corev1.ResourceCPU {
			// cpu metrics must be of the double type to adhere to opentelemetry system.cpu metric specifications
			val = utils.GetDoubleTimeSeries(float64(quantity.MilliValue()) / 1000.0)
			valType = metricspb.MetricDescriptor_GAUGE_DOUBLE
		}
		metrics = append(metrics, &metricspb.Metric{
			MetricDescriptor: &metricspb.MetricDescriptor{
				Name:        nodeAllocatableMetric,
				Description: allocatableDesciption[v1NodeAllocatableTypeValue.String()],
				Type:        valType,
			},
			Timeseries: []*metricspb.TimeSeries{
				val,
			},
		})
	}

	return []*agentmetricspb.ExportMetricsServiceRequest{
		{
			Resource: getResourceForNode(node),
			Metrics:  metrics,
		},
	}
}

func getNodeConditionMetric(nodeConditionTypeValue string) string {
	return fmt.Sprintf("k8s.node.condition_%s", strcase.ToSnake(nodeConditionTypeValue))
}

func getNodeAllocatableMetric(nodeAllocatableTypeValue string) string {
	return fmt.Sprintf("k8s.node.allocatable_%s", strcase.ToSnake(nodeAllocatableTypeValue))
}

func getResourceForNode(node *corev1.Node) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: constants.K8sType,
		Labels: map[string]string{
			conventions.AttributeK8SNodeUID:  string(node.UID),
			conventions.AttributeK8SNodeName: node.Name,
		},
	}
}

var nodeConditionValues = map[corev1.ConditionStatus]int64{
	corev1.ConditionTrue:    1,
	corev1.ConditionFalse:   0,
	corev1.ConditionUnknown: -1,
}

func nodeConditionValue(node *corev1.Node, condType corev1.NodeConditionType) int64 {
	status := corev1.ConditionUnknown
	for _, c := range node.Status.Conditions {
		if c.Type == condType {
			status = c.Status
			break
		}
	}
	return nodeConditionValues[status]
}

func GetMetadata(node *corev1.Node) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	meta := maps.MergeStringMaps(map[string]string{}, node.Labels)

	meta[conventions.AttributeK8SNodeName] = node.Name
	meta[nodeCreationTime] = node.GetCreationTimestamp().Format(time.RFC3339)

	nodeID := experimentalmetricmetadata.ResourceID(node.UID)
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		nodeID: {
			ResourceIDKey: conventions.AttributeK8SNodeUID,
			ResourceID:    nodeID,
			Metadata:      meta,
		},
	}
}
