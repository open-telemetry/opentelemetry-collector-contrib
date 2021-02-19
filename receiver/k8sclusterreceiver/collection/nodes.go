// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collection

import (
	"fmt"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/iancoleman/strcase"
	"go.opentelemetry.io/collector/translator/conventions"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testing/util"
	metadataPkg "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/utils"
)

const (
	// Keys for node metadata.
	nodeCreationTime = "node.creation_timestamp"
)

func getMetricsForNode(node *corev1.Node, nodeConditionTypesToReport []string) []*resourceMetrics {
	metrics := make([]*metricspb.Metric, len(nodeConditionTypesToReport))

	for i, nodeConditionTypeValue := range nodeConditionTypesToReport {
		nodeConditionMetric := getNodeConditionMetric(nodeConditionTypeValue)
		v1NodeConditionTypeValue := corev1.NodeConditionType(nodeConditionTypeValue)

		metrics[i] = &metricspb.Metric{
			MetricDescriptor: &metricspb.MetricDescriptor{
				Name: nodeConditionMetric,
				Description: fmt.Sprintf("Whether this node is %s (1), "+
					"not %s (0) or in an unknown state (-1)", nodeConditionTypeValue, nodeConditionTypeValue),
				Type: metricspb.MetricDescriptor_GAUGE_INT64,
			},
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(nodeConditionValue(node, v1NodeConditionTypeValue)),
			},
		}
	}

	return []*resourceMetrics{
		{
			resource: getResourceForNode(node),
			metrics:  metrics,
		},
	}
}

func getNodeConditionMetric(nodeConditionTypeValue string) string {
	return fmt.Sprintf("k8s.node.condition_%s", strcase.ToSnake(nodeConditionTypeValue))
}

func getResourceForNode(node *corev1.Node) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: k8sType,
		Labels: map[string]string{
			k8sKeyNodeUID:                   string(node.UID),
			k8sKeyNodeName:                  node.Name,
			conventions.AttributeK8sCluster: node.ClusterName,
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

func getMetadataForNode(node *corev1.Node) map[metadataPkg.ResourceID]*KubernetesMetadata {
	metadata := util.MergeStringMaps(map[string]string{}, node.Labels)

	metadata[k8sKeyNodeName] = node.Name
	metadata[nodeCreationTime] = node.GetCreationTimestamp().Format(time.RFC3339)

	nodeID := metadataPkg.ResourceID(node.UID)
	return map[metadataPkg.ResourceID]*KubernetesMetadata{
		nodeID: {
			resourceIDKey: k8sKeyNodeUID,
			resourceID:    nodeID,
			metadata:      metadata,
		},
	}
}
