// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package node // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/node"

import (
	"fmt"
	"strings"
	"time"

	"github.com/iancoleman/strcase"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	conventions "go.opentelemetry.io/collector/semconv/v1.18.0"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/maps"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

const (
	// Keys for node metadata and entity attributes. These are NOT used by resource attributes.
	nodeCreationTime       = "node.creation_timestamp"
	k8sNodeConditionPrefix = "k8s.node.condition"
)

// Transform transforms the node to remove the fields that we don't use to reduce RAM utilization.
// IMPORTANT: Make sure to update this function before using new node fields.
func Transform(node *corev1.Node) *corev1.Node {
	newNode := &corev1.Node{
		ObjectMeta: metadata.TransformObjectMeta(node.ObjectMeta),
		Status: corev1.NodeStatus{
			Allocatable: node.Status.Allocatable,
			NodeInfo: corev1.NodeSystemInfo{
				KubeletVersion:          node.Status.NodeInfo.KubeletVersion,
				ContainerRuntimeVersion: node.Status.NodeInfo.ContainerRuntimeVersion,
				OSImage:                 node.Status.NodeInfo.OSImage,
				OperatingSystem:         node.Status.NodeInfo.OperatingSystem,
			},
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

func RecordMetrics(mb *metadata.MetricsBuilder, node *corev1.Node, ts pcommon.Timestamp) {
	for _, c := range node.Status.Conditions {
		mb.RecordK8sNodeConditionDataPoint(ts, nodeConditionValues[c.Status], string(c.Type))
	}
	rb := mb.NewResourceBuilder()
	rb.SetK8sNodeUID(string(node.UID))
	rb.SetK8sNodeName(node.Name)
	rb.SetK8sKubeletVersion(node.Status.NodeInfo.KubeletVersion)

	mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func CustomMetrics(set receiver.Settings, rb *metadata.ResourceBuilder, node *corev1.Node, nodeConditionTypesToReport,
	allocatableTypesToReport []string, ts pcommon.Timestamp,
) pmetric.ResourceMetrics {
	rm := pmetric.NewResourceMetrics()

	sm := rm.ScopeMetrics().AppendEmpty()
	// Adding 'node condition type' metrics
	for _, nodeConditionTypeValue := range nodeConditionTypesToReport {
		v1NodeConditionTypeValue := corev1.NodeConditionType(nodeConditionTypeValue)
		m := sm.Metrics().AppendEmpty()
		m.SetName(getNodeConditionMetric(nodeConditionTypeValue))
		m.SetDescription(fmt.Sprintf("%v condition status of the node (true=1, false=0, unknown=-1)", nodeConditionTypeValue))
		m.SetUnit("")
		g := m.SetEmptyGauge()
		dp := g.DataPoints().AppendEmpty()
		dp.SetIntValue(nodeConditionValue(node, v1NodeConditionTypeValue))
		dp.SetTimestamp(ts)
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
		m := sm.Metrics().AppendEmpty()
		m.SetName(getNodeAllocatableMetric(nodeAllocatableTypeValue))
		m.SetDescription(fmt.Sprintf("Amount of %v allocatable on the node", nodeAllocatableTypeValue))
		m.SetUnit(getNodeAllocatableUnit(v1NodeAllocatableTypeValue))
		g := m.SetEmptyGauge()
		dp := g.DataPoints().AppendEmpty()
		setNodeAllocatableValue(dp, v1NodeAllocatableTypeValue, quantity)
		dp.SetTimestamp(ts)
	}

	if sm.Metrics().Len() == 0 {
		return pmetric.NewResourceMetrics()
	}

	// TODO: Generate a schema URL for the node metrics in the metadata package and use them here.
	rm.SetSchemaUrl(conventions.SchemaURL)
	sm.Scope().SetName("github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver")
	sm.Scope().SetVersion(set.BuildInfo.Version)

	rb.SetK8sNodeUID(string(node.UID))
	rb.SetK8sNodeName(node.Name)
	rb.SetK8sKubeletVersion(node.Status.NodeInfo.KubeletVersion)
	rb.SetOsType(node.Status.NodeInfo.OperatingSystem)

	runtime, version := getContainerRuntimeInfo(node.Status.NodeInfo.ContainerRuntimeVersion)
	if runtime != "" {
		rb.SetContainerRuntime(runtime)
	}
	if version != "" {
		rb.SetContainerRuntimeVersion(version)
	}

	rb.SetOsDescription(node.Status.NodeInfo.OSImage)
	rb.Emit().MoveTo(rm.Resource())
	return rm
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

	// Node can have many additional conditions (gke has 18 on v1.29). Bad thresholds/implementations
	// of custom conditions can cause value to oscillate between true/false frequently. So, only sending the node
	// pressure conditions that are set by kubelet to avoid noise.
	// https://pkg.go.dev/k8s.io/api/core/v1#NodeConditionType
	kubeletConditions := map[corev1.NodeConditionType]struct{}{
		corev1.NodeReady:              {},
		corev1.NodeMemoryPressure:     {},
		corev1.NodeDiskPressure:       {},
		corev1.NodePIDPressure:        {},
		corev1.NodeNetworkUnavailable: {},
	}

	for _, c := range node.Status.Conditions {
		if _, ok := kubeletConditions[c.Type]; ok {
			meta[fmt.Sprintf("%s_%s", k8sNodeConditionPrefix, strcase.ToSnake(string(c.Type)))] = strings.ToLower(string(c.Status))
		}
	}

	nodeID := experimentalmetricmetadata.ResourceID(node.UID)
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		nodeID: {
			EntityType:    "k8s.node",
			ResourceIDKey: conventions.AttributeK8SNodeUID,
			ResourceID:    nodeID,
			Metadata:      meta,
		},
	}
}

func getContainerRuntimeInfo(rawInfo string) (runtime string, version string) {
	// Kubelet reports container runtime version in the following format:
	// <runtime-name>://<version>
	parts := strings.Split(rawInfo, "://")

	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

func getNodeConditionMetric(nodeConditionTypeValue string) string {
	return "k8s.node.condition_" + strcase.ToSnake(nodeConditionTypeValue)
}

func getNodeAllocatableUnit(res corev1.ResourceName) string {
	switch res {
	case corev1.ResourceCPU:
		return "{cpu}"
	case corev1.ResourceMemory, corev1.ResourceEphemeralStorage, corev1.ResourceStorage:
		return "By"
	case corev1.ResourcePods:
		return "{pod}"
	default:
		return fmt.Sprintf("{%s}", string(res))
	}
}

func setNodeAllocatableValue(dp pmetric.NumberDataPoint, res corev1.ResourceName, q resource.Quantity) {
	switch res {
	case corev1.ResourceCPU:
		dp.SetDoubleValue(float64(q.MilliValue()) / 1000.0)
	default:
		dp.SetIntValue(q.Value())
	}
}

func getNodeAllocatableMetric(nodeAllocatableTypeValue string) string {
	return "k8s.node.allocatable_" + strcase.ToSnake(nodeAllocatableTypeValue)
}
