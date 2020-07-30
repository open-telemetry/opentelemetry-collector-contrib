// Copyright OpenTelemetry Authors
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

package stackdriverexporter

import (
	"go.opencensus.io/resource/resourcekeys"
)

type resourceTypeDetector struct {
	// label presence to check against
	labelKey string
	// matching resource type
	resourceType string
}

// mapping of label presence to inferred resource type
// NOTE: defined in the priority order (first match wins)
var attributeToResourceType = []resourceTypeDetector{
	{
		// See https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/resource/semantic_conventions/container.md
		labelKey:     resourcekeys.ContainerKeyName,
		resourceType: resourcekeys.ContainerType,
	},
	{
		// See https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/resource/semantic_conventions/k8s.md#pod
		labelKey: resourcekeys.K8SKeyPodName,
		// NOTE: OpenCensus is using "k8s" rather than "k8s.pod" for Pod
		resourceType: resourcekeys.K8SType,
	},
	{
		// See https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/resource/semantic_conventions/host.md
		labelKey:     resourcekeys.HostKeyName,
		resourceType: resourcekeys.HostType,
	},
	{
		// See https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/resource/semantic_conventions/cloud.md
		labelKey:     resourcekeys.CloudKeyProvider,
		resourceType: resourcekeys.CloudType,
	},
}

func inferResourceType(labels map[string]string) (string, bool) {
	if labels == nil {
		return "", false
	}

	for _, detector := range attributeToResourceType {
		if _, ok := labels[detector.labelKey]; ok {
			return detector.resourceType, true
		}
	}

	return "", false
}
