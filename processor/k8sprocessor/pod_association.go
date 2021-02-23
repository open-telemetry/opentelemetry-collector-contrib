// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8sprocessor

import (
	"context"
	"net"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor/kube"
)

// extractPodIds extracts IP and pod UID from attributes or request context.
// It returns a value pair containing configured label and IP Address and/or Pod UID
func extractPodIds(ctx context.Context, attrs pdata.AttributeMap, associations []kube.Association) (string, kube.PodIdentifier) {
	hostname := stringAttributeFromMap(attrs, conventions.AttributeHostName)
	var connectionIP kube.PodIdentifier
	if c, ok := client.FromContext(ctx); ok {
		connectionIP = kube.PodIdentifier(c.IP)
	}
	// If pod association is not set
	if len(associations) == 0 {
		var podIP, labelIP kube.PodIdentifier
		podIP = kube.PodIdentifier(stringAttributeFromMap(attrs, k8sIPLabelName))
		labelIP = kube.PodIdentifier(stringAttributeFromMap(attrs, clientIPLabelName))

		if podIP != "" {
			return k8sIPLabelName, podIP
		} else if labelIP != "" {
			return k8sIPLabelName, labelIP
		} else if connectionIP != "" {
			return k8sIPLabelName, connectionIP
		} else if net.ParseIP(hostname) != nil {
			return k8sIPLabelName, kube.PodIdentifier(hostname)
		}
		return "", kube.PodIdentifier("")
	}

	for _, asso := range associations {
		// If association configured to take IP address from connection
		if asso.From == "connection" && connectionIP != "" {
			return k8sIPLabelName, connectionIP
		} else if asso.From == "labels" { // If association configured by labels
			// In k8s environment, host.name label set to a pod IP address.
			// If the value doesn't represent an IP address, we skip it.
			if asso.Name == conventions.AttributeHostName {
				if net.ParseIP(hostname) != nil {
					return k8sIPLabelName, kube.PodIdentifier(hostname)
				}
			} else {
				// Extract values based od configured labels.
				attributeValue := stringAttributeFromMap(attrs, asso.Name)
				if attributeValue != "" {
					return asso.Name, kube.PodIdentifier(attributeValue)
				}
			}
		}
	}
	return "", kube.PodIdentifier("")
}

func stringAttributeFromMap(attrs pdata.AttributeMap, key string) string {
	if val, ok := attrs.Get(key); ok {
		if val.Type() == pdata.AttributeValueSTRING {
			return val.StringVal()
		}
	}
	return ""
}
