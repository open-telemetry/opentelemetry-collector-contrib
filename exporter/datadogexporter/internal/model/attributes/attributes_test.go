// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package attributes

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.6.1"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestTagsFromAttributes(t *testing.T) {
	attributeMap := map[string]interface{}{
		conventions.AttributeProcessExecutableName: "otelcol",
		conventions.AttributeProcessExecutablePath: "/usr/bin/cmd/otelcol",
		conventions.AttributeProcessCommand:        "cmd/otelcol",
		conventions.AttributeProcessCommandLine:    "cmd/otelcol --config=\"/path/to/config.yaml\"",
		conventions.AttributeProcessPID:            1,
		conventions.AttributeProcessOwner:          "root",
		conventions.AttributeOSType:                "linux",
		conventions.AttributeK8SDaemonSetName:      "daemon_set_name",
		conventions.AttributeAWSECSClusterARN:      "cluster_arn",
		"tags.datadoghq.com/service":               "service_name",
	}
	attrs := pcommon.NewMapFromRaw(attributeMap)

	assert.ElementsMatch(t, []string{
		fmt.Sprintf("%s:%s", conventions.AttributeProcessExecutableName, "otelcol"),
		fmt.Sprintf("%s:%s", conventions.AttributeOSType, "linux"),
		fmt.Sprintf("%s:%s", "kube_daemon_set", "daemon_set_name"),
		fmt.Sprintf("%s:%s", "ecs_cluster_name", "cluster_arn"),
		fmt.Sprintf("%s:%s", "service", "service_name"),
	}, TagsFromAttributes(attrs))
}

func TestTagsFromAttributesEmpty(t *testing.T) {
	attrs := pcommon.NewMap()

	assert.Equal(t, []string{}, TagsFromAttributes(attrs))
}

func TestContainerTagFromAttributes(t *testing.T) {
	attributeMap := map[string]string{
		conventions.AttributeContainerName:         "sample_app",
		conventions.AttributeContainerImageTag:     "sample_app_image_tag",
		conventions.AttributeK8SContainerName:      "kube_sample_app",
		conventions.AttributeK8SReplicaSetName:     "sample_replica_set",
		conventions.AttributeK8SDaemonSetName:      "sample_daemonset_name",
		conventions.AttributeK8SPodName:            "sample_pod_name",
		conventions.AttributeCloudProvider:         "sample_cloud_provider",
		conventions.AttributeCloudRegion:           "sample_region",
		conventions.AttributeCloudAvailabilityZone: "sample_zone",
		conventions.AttributeAWSECSTaskFamily:      "sample_task_family",
		conventions.AttributeAWSECSClusterARN:      "sample_ecs_cluster_name",
		conventions.AttributeAWSECSContainerARN:    "sample_ecs_container_name",
		"custom_tag":                               "example_custom_tag",
		"":                                         "empty_string_key",
		"empty_string_val":                         "",
	}

	assert.Equal(t, "container_name:sample_app,image_tag:sample_app_image_tag,kube_container_name:kube_sample_app,kube_replica_set:sample_replica_set,kube_daemon_set:sample_daemonset_name,pod_name:sample_pod_name,cloud_provider:sample_cloud_provider,region:sample_region,zone:sample_zone,task_family:sample_task_family,ecs_cluster_name:sample_ecs_cluster_name,ecs_container_name:sample_ecs_container_name", ContainerTagFromAttributes(attributeMap))
}

func TestContainerTagFromAttributesEmpty(t *testing.T) {
	var empty string
	attributeMap := map[string]string{}

	assert.Equal(t, empty, ContainerTagFromAttributes(attributeMap))
}

func TestOriginIDFromAttributes(t *testing.T) {
	tests := []struct {
		name     string
		attrs    pcommon.Map
		originID string
	}{
		{
			name: "pod UID and container ID",
			attrs: pcommon.NewMapFromRaw(map[string]interface{}{
				conventions.AttributeContainerID: "container_id_goes_here",
				conventions.AttributeK8SPodUID:   "k8s_pod_uid_goes_here",
			}),
			originID: "container_id://container_id_goes_here",
		},
		{
			name: "only container ID",
			attrs: pcommon.NewMapFromRaw(map[string]interface{}{
				conventions.AttributeContainerID: "container_id_goes_here",
			}),
			originID: "container_id://container_id_goes_here",
		},
		{
			name: "only pod UID",
			attrs: pcommon.NewMapFromRaw(map[string]interface{}{
				conventions.AttributeK8SPodUID: "k8s_pod_uid_goes_here",
			}),
			originID: "kubernetes_pod_uid://k8s_pod_uid_goes_here",
		},
		{
			name:  "none",
			attrs: pcommon.NewMap(),
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			originID := OriginIDFromAttributes(testInstance.attrs)
			assert.Equal(t, testInstance.originID, originID)
		})
	}
}
