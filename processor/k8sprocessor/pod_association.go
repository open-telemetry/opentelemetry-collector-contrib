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

// k8sPodAssociationFromAttributes extracts IP and pod UID from attributes based on association config.
// It returns a map containing specified labels as a keys and IP Address and/or Pod UID
func k8sPodAssociationFromAttributes(ctx context.Context, attrs pdata.AttributeMap, associations []kube.Association) map[string]string {
	podAssociation := make(map[string]string)
	var clientIP string
	if c, ok := client.FromContext(ctx); ok {
		clientIP = c.IP
	}
	// If pod association is not set
	if len(associations) == 0 {
		var podIP, labelIP string
		podIP = stringAttributeFromMap(attrs, k8sIPLabelName)
		labelIP = stringAttributeFromMap(attrs, clientIPLabelName)

		if podIP != "" {
			podAssociation[k8sIPLabelName] = podIP
		} else if labelIP != "" {
			podAssociation[k8sIPLabelName] = labelIP
		} else if clientIP != "" {
			podAssociation[k8sIPLabelName] = clientIP
		}
		return podAssociation
	}

	for _, asso := range associations {
		// If association configured to take IP address from connection
		if asso.From == "connection" && clientIP != "" {
			podAssociation[k8sIPLabelName] = clientIP
		} else if asso.From == "labels" { // If association configured by labels
			// Special case for host.name label
			if asso.Name == conventions.AttributeHostName {
				hostname := stringAttributeFromMap(attrs, conventions.AttributeHostName)
				if net.ParseIP(hostname) != nil {
					podAssociation[k8sIPLabelName] = hostname
				}
				continue
			}
			// Extract values based od configured labels.
			attributeValue := stringAttributeFromMap(attrs, asso.Name)
			if attributeValue != "" {
				podAssociation[asso.Name] = attributeValue
			}
		}
	}
	return podAssociation
}

func stringAttributeFromMap(attrs pdata.AttributeMap, key string) string {
	if val, ok := attrs.Get(key); ok {
		if val.Type() == pdata.AttributeValueSTRING {
			return val.StringVal()
		}
	}
	return ""
}
