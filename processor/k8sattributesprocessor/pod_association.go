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

// extractPodIds extracts IP and pod UID from attributes or request context.
// It returns a value pair containing configured label and IP Address and/or Pod UID.
// If empty value in return it means that attributes does not contains configured label to match resources for Pod.
func extractPodID(ctx context.Context, attrs pcommon.Map, associations []kube.Association) (string, kube.PodIdentifier) {
	// If pod association is not set
	if len(associations) == 0 {
		return extractPodIDNoAssociations(ctx, attrs)
	}

	connectionIP := getConnectionIP(ctx)
	hostname := stringAttributeFromMap(attrs, conventions.AttributeHostName)
	for _, asso := range associations {
		// If association configured to take IP address from connection
		switch {
		case asso.From == "connection" && connectionIP != "":
			return k8sIPLabelName, connectionIP
		case asso.From == "resource_attribute":
			// If association configured by resource_attribute
			// In k8s environment, host.name label set to a pod IP address.
			// If the value doesn't represent an IP address, we skip it.
			if asso.Name == conventions.AttributeHostName {
				if net.ParseIP(hostname) != nil {
					return k8sIPLabelName, kube.PodIdentifier(hostname)
				}
			} else {
				// Extract values based on configured resource_attribute.
				attributeValue := stringAttributeFromMap(attrs, asso.Name)
				if attributeValue != "" {
					return asso.Name, kube.PodIdentifier(attributeValue)
				}
			}
		}
	}
	return "", ""
}

func extractPodIDNoAssociations(ctx context.Context, attrs pcommon.Map) (string, kube.PodIdentifier) {
	var podIP, labelIP kube.PodIdentifier
	podIP = kube.PodIdentifier(stringAttributeFromMap(attrs, k8sIPLabelName))
	if podIP != "" {
		return k8sIPLabelName, podIP
	}

	labelIP = kube.PodIdentifier(stringAttributeFromMap(attrs, clientIPLabelName))
	if labelIP != "" {
		return k8sIPLabelName, labelIP
	}

	connectionIP := getConnectionIP(ctx)
	if connectionIP != "" {
		return k8sIPLabelName, connectionIP
	}

	hostname := stringAttributeFromMap(attrs, conventions.AttributeHostName)
	if net.ParseIP(hostname) != nil {
		return k8sIPLabelName, kube.PodIdentifier(hostname)
	}

	return "", ""
}

func getConnectionIP(ctx context.Context) kube.PodIdentifier {
	c := client.FromContext(ctx)
	if c.Addr == nil {
		return ""
	}
	switch addr := c.Addr.(type) {
	case *net.UDPAddr:
		return kube.PodIdentifier(addr.IP.String())
	case *net.TCPAddr:
		return kube.PodIdentifier(addr.IP.String())
	case *net.IPAddr:
		return kube.PodIdentifier(addr.IP.String())
	}

	// If this is not a known address type, check for known "untyped" formats.
	// 1.1.1.1:<port>

	lastColonIndex := strings.LastIndex(c.Addr.String(), ":")
	if lastColonIndex != -1 {
		ipString := c.Addr.String()[:lastColonIndex]
		ip := net.ParseIP(ipString)
		if ip != nil {
			return kube.PodIdentifier(ip.String())
		}
	}

	return kube.PodIdentifier(c.Addr.String())

}

func stringAttributeFromMap(attrs pcommon.Map, key string) string {
	if val, ok := attrs.Get(key); ok {
		if val.Type() == pcommon.ValueTypeString {
			return val.StringVal()
		}
	}
	return ""
}
