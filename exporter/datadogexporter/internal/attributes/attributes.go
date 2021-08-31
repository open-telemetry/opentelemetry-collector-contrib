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

	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
)

var (
	// conventionsMappings defines the mapping between OpenTelemetry semantic conventions
	// and Datadog Agent conventions
	conventionsMapping = map[string]string{
		// Datadog conventions
		// https://docs.datadoghq.com/getting_started/tagging/unified_service_tagging/
		conventions.AttributeDeploymentEnvironment: "env",
		conventions.AttributeServiceName:           "service",
		conventions.AttributeServiceVersion:        "version",

		// Cloud conventions
		// https://www.datadoghq.com/blog/tagging-best-practices/
		conventions.AttributeCloudProvider:         "cloud_provider",
		conventions.AttributeCloudRegion:           "region",
		conventions.AttributeCloudAvailabilityZone: "zone",

		// ECS conventions
		// https://github.com/DataDog/datadog-agent/blob/e081bed/pkg/tagger/collectors/ecs_extract.go
		conventions.AttributeAWSECSTaskFamily: "task_family",
		conventions.AttributeAWSECSClusterARN: "ecs_cluster_name",
		"aws.ecs.task.revision":               "task_version",

		// Kubernetes resource name (via semantic conventions)
		// https://github.com/DataDog/datadog-agent/blob/e081bed/pkg/util/kubernetes/const.go
		conventions.AttributeK8SPodName:         "pod_name",
		conventions.AttributeK8SDeploymentName:  "kube_deployment",
		conventions.AttributeK8SReplicaSetName:  "kube_replica_set",
		conventions.AttributeK8SStatefulSetName: "kube_stateful_set",
		conventions.AttributeK8SDaemonSetName:   "kube_daemon_set",
		conventions.AttributeK8SJobName:         "kube_job",
		conventions.AttributeK8SCronJobName:     "kube_cronjob",
	}

	// Kubernetes mappings defines the mapping between Kubernetes conventions (both general and Datadog specific)
	// and Datadog Agent conventions. The Datadog Agent conventions can be found at
	// https://github.com/DataDog/datadog-agent/blob/e081bed/pkg/tagger/collectors/const.go and
	// https://github.com/DataDog/datadog-agent/blob/e081bed/pkg/util/kubernetes/const.go
	kubernetesMapping = map[string]string{
		// Standard Datadog labels
		"tags.datadoghq.com/env":     "env",
		"tags.datadoghq.com/service": "service",
		"tags.datadoghq.com/version": "version",

		// Standard Kubernetes labels
		"app.kubernetes.io/name":       "kube_app_name",
		"app.kubernetes.io/instance":   "kube_app_instance",
		"app.kubernetes.io/version":    "kube_app_version",
		"app.kuberenetes.io/component": "kube_app_component",
		"app.kubernetes.io/part-of":    "kube_app_part_of",
		"app.kubernetes.io/managed-by": "kube_app_managed_by",
	}
)

// TagsFromAttributes converts a selected list of attributes
// to a tag list that can be added to metrics.
func TagsFromAttributes(attrs pdata.AttributeMap) []string {
	tags := make([]string, 0, attrs.Len())

	var processAttributes processAttributes
	var systemAttributes systemAttributes

	attrs.Range(func(key string, value pdata.AttributeValue) bool {
		switch key {
		// Process attributes
		case conventions.AttributeProcessExecutableName:
			processAttributes.ExecutableName = value.StringVal()
		case conventions.AttributeProcessExecutablePath:
			processAttributes.ExecutablePath = value.StringVal()
		case conventions.AttributeProcessCommand:
			processAttributes.Command = value.StringVal()
		case conventions.AttributeProcessCommandLine:
			processAttributes.CommandLine = value.StringVal()
		case conventions.AttributeProcessPID:
			processAttributes.PID = value.IntVal()
		case conventions.AttributeProcessOwner:
			processAttributes.Owner = value.StringVal()

		// System attributes
		case conventions.AttributeOSType:
			systemAttributes.OSType = value.StringVal()
		}

		// conventions mapping
		if datadogKey, found := conventionsMapping[key]; found && value.StringVal() != "" {
			tags = append(tags, fmt.Sprintf("%s:%s", datadogKey, value.StringVal()))
		}

		// Kubernetes labels mapping
		if datadogKey, found := kubernetesMapping[key]; found && value.StringVal() != "" {
			tags = append(tags, fmt.Sprintf("%s:%s", datadogKey, value.StringVal()))
		}
		return true
	})

	tags = append(tags, processAttributes.extractTags()...)
	tags = append(tags, systemAttributes.extractTags()...)

	return tags
}
