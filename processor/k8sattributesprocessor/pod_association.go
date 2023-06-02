// Copyright The OpenTelemetry Authors
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

package k8sattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"

import (
	"context"
	"net"
	"strings"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
)

// extractPodIds returns pod identifier for first association matching all sources
func extractPodID(ctx context.Context, attrs pcommon.Map, associations []kube.Association) kube.PodIdentifier {
	// If pod association is not set
	if len(associations) == 0 {
		return extractPodIDNoAssociations(ctx, attrs)
	}

	connectionIP := connectionIP(ctx)
	for _, asso := range associations {
		skip := false

		ret := kube.PodIdentifier{}
		for i, source := range asso.Sources {
			// If association configured to take IP address from connection
			switch {
			case source.From == kube.ConnectionSource:
				if connectionIP == "" {
					skip = true
					break
				}
				ret[i] = kube.PodIdentifierAttributeFromConnection(connectionIP)
			case source.From == kube.ResourceSource:
				// Extract values based on configured resource_attribute.
				attributeValue := stringAttributeFromMap(attrs, source.Name)
				if attributeValue == "" {
					skip = true
					break
				}

				// If association configured by resource_attribute
				// In k8s environment, host.name label set to a pod IP address.
				// If the value doesn't represent an IP address, we skip it.
				if asso.Name == conventions.AttributeHostName && net.ParseIP(attributeValue) == nil {
					skip = true
					break
				}

				ret[i] = kube.PodIdentifierAttributeFromSource(source, attributeValue)
			}
		}

		// If all association sources has been resolved, return result
		if !skip {
			return ret
		}
	}
	return kube.PodIdentifier{}
}

// extractPodIds returns pod identifier for first association matching all sources
func extractPodIDNoAssociations(ctx context.Context, attrs pcommon.Map) kube.PodIdentifier {
	var podIP, labelIP string
	podIP = stringAttributeFromMap(attrs, kube.K8sIPLabelName)
	if podIP != "" {
		return kube.PodIdentifier{
			kube.PodIdentifierAttributeFromConnection(podIP),
		}
	}

	labelIP = stringAttributeFromMap(attrs, clientIPLabelName)
	if labelIP != "" {
		return kube.PodIdentifier{
			kube.PodIdentifierAttributeFromConnection(labelIP),
		}
	}

	connectionIP := connectionIP(ctx)
	if connectionIP != "" {
		return kube.PodIdentifier{
			kube.PodIdentifierAttributeFromConnection(connectionIP),
		}
	}

	hostname := stringAttributeFromMap(attrs, conventions.AttributeHostName)
	if net.ParseIP(hostname) != nil {
		return kube.PodIdentifier{
			kube.PodIdentifierAttributeFromConnection(hostname),
		}
	}

	return kube.PodIdentifier{}
}

func connectionIP(ctx context.Context) string {
	c := client.FromContext(ctx)
	if c.Addr == nil {
		return ""
	}
	switch addr := c.Addr.(type) {
	case *net.UDPAddr:
		return addr.IP.String()
	case *net.TCPAddr:
		return addr.IP.String()
	case *net.IPAddr:
		return addr.IP.String()
	}

	// If this is not a known address type, check for known "untyped" formats.
	// 1.1.1.1:<port>

	lastColonIndex := strings.LastIndex(c.Addr.String(), ":")
	if lastColonIndex != -1 {
		ipString := c.Addr.String()[:lastColonIndex]
		ip := net.ParseIP(ipString)
		if ip != nil {
			return ip.String()
		}
	}

	return c.Addr.String()

}

func stringAttributeFromMap(attrs pcommon.Map, key string) string {
	if val, ok := attrs.Get(key); ok {
		if val.Type() == pcommon.ValueTypeStr {
			return val.Str()
		}
	}
	return ""
}
