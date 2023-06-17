// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package node // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/node"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/maps"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	imetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/node/internal/metadata"
)

const (
	// Keys for node metadata.
	nodeCreationTime = "node.creation_timestamp"
)

// Transform transforms the node to remove the fields that we don't use to reduce RAM utilization.
// IMPORTANT: Make sure to update this function when using a new node fields.
func Transform(node *corev1.Node) *corev1.Node {
	newNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   node.ObjectMeta.Name,
			UID:    node.ObjectMeta.UID,
			Labels: node.ObjectMeta.Labels,
		},
		Status: corev1.NodeStatus{
			Allocatable: node.Status.Allocatable,
		},
	}
	for _, c := range node.Status.Conditions {
		newNode.Status.Conditions = append(newNode.Status.Conditions, corev1.NodeCondition{
			Type:   c.Type,
			Status: c.Status,
		})
	}
	return newNode
}

func GetMetrics(set receiver.CreateSettings, node *corev1.Node, nodeConditionTypesToReport, allocatableTypesToReport []string) pmetric.Metrics {
	mb := imetadata.NewMetricsBuilder(imetadata.DefaultMetricsBuilderConfig(), set)
	ts := pcommon.NewTimestampFromTime(time.Now())

	// Adding 'node condition type' metrics
	for _, nodeConditionTypeValue := range nodeConditionTypesToReport {
		v1NodeConditionTypeValue := corev1.NodeConditionType(nodeConditionTypeValue)
		switch v1NodeConditionTypeValue {
		case corev1.NodeReady:
			mb.RecordK8sNodeConditionReadyDataPoint(ts, nodeConditionValue(node, v1NodeConditionTypeValue))
		case corev1.NodeMemoryPressure:
			mb.RecordK8sNodeConditionMemoryPressureDataPoint(ts, nodeConditionValue(node, v1NodeConditionTypeValue))
		case corev1.NodeDiskPressure:
			mb.RecordK8sNodeConditionDiskPressureDataPoint(ts, nodeConditionValue(node, v1NodeConditionTypeValue))
		case corev1.NodeNetworkUnavailable:
			mb.RecordK8sNodeConditionNetworkUnavailableDataPoint(ts, nodeConditionValue(node, v1NodeConditionTypeValue))
		case corev1.NodePIDPressure:
			mb.RecordK8sNodeConditionPidPressureDataPoint(ts, nodeConditionValue(node, v1NodeConditionTypeValue))
		default:
			set.Logger.Warn("unknown node condition type", zap.String("conditionType", nodeConditionTypeValue))
		}
	}

	// Adding 'node allocatable type' metrics
	for _, nodeAllocatableTypeValue := range allocatableTypesToReport {
		v1NodeAllocatableTypeValue := corev1.ResourceName(nodeAllocatableTypeValue)
		quantity, ok := node.Status.Allocatable[v1NodeAllocatableTypeValue]
		if !ok {
			set.Logger.Debug(fmt.Errorf("allocatable type %v not found in node %v", nodeAllocatableTypeValue,
				node.GetName()).Error())
			continue
		}
		switch v1NodeAllocatableTypeValue {
		case corev1.ResourceCPU:
			// cpu metrics must be of the double type to adhere to opentelemetry system.cpu metric specifications
			mb.RecordK8sNodeAllocatableCPUDataPoint(ts, float64(quantity.MilliValue())/1000.0)
		case corev1.ResourceMemory:
			mb.RecordK8sNodeAllocatableMemoryDataPoint(ts, quantity.Value())
		case corev1.ResourceEphemeralStorage:
			mb.RecordK8sNodeAllocatableEphemeralStorageDataPoint(ts, quantity.Value())
		case corev1.ResourceStorage:
			mb.RecordK8sNodeAllocatableStorageDataPoint(ts, quantity.Value())
		}
	}
	return mb.Emit(imetadata.WithK8sNodeUID(string(node.UID)), imetadata.WithK8sNodeName(node.Name), imetadata.WithOpencensusResourcetype("k8s"))

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
