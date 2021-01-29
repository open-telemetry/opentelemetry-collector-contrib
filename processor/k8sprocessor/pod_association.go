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

// k8sPodAssociationFromAttributes extracts IP and pod UID from attributes
func k8sPodAssociationFromAttributes(ctx context.Context, attrs pdata.AttributeMap, associations kube.Associations) map[string]string {
	var podIP, clientIP, contextIP string
	podAssociation := make(map[string]string)

	podIP = stringAttributeFromMap(attrs, k8sIPLabelName)
	clientIP = stringAttributeFromMap(attrs, clientIPLabelName)
	if c, ok := client.FromContext(ctx); ok {
		contextIP = c.IP
	}

	// If pod association is not set
	if len(associations.Associations) == 0 {
		if podIP != "" {
			podAssociation[k8sIPLabelName] = podIP
		} else if clientIP != "" {
			podAssociation[k8sIPLabelName] = clientIP
		} else if contextIP != "" {
			podAssociation[k8sIPLabelName] = contextIP
		}
		return podAssociation

	}

	for _, asso := range associations.Associations {
		if asso.Name == podUIDLabelName {
			uid := stringAttributeFromMap(attrs, asso.Name)
			if uid != "" {
				podAssociation[k8sPodUIDLabelName] = uid
			}
		}
		if _, ok := podAssociation[k8sIPLabelName]; !ok {
			if asso.Name == k8sIPLabelName && podIP != "" {
				podAssociation[k8sIPLabelName] = podIP
				continue
			}
			if asso.Name == "ip" {
				switch asso.From {
				case "labels":
					if clientIP != "" {
						podAssociation[k8sIPLabelName] = clientIP
					}
				case "connection":
					if contextIP != "" {
						podAssociation[k8sIPLabelName] = contextIP
					}
				default:
					continue
				}
			}
		}
		if asso.Name == hostnameLabelName {
			hostname := stringAttributeFromMap(attrs, conventions.AttributeHostName)
			if net.ParseIP(hostname) != nil {
				podAssociation[k8sIPLabelName] = hostname
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
