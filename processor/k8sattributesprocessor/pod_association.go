// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"

import (
	"context"
	"net"
	"strings"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.41.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/clientutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
)

// extractPodIds returns pod identifier for first association matching all sources
func extractPodID(ctx context.Context, attrs pcommon.Map, associations []kube.Association) kube.PodIdentifier {
	// If pod association is not set
	if len(associations) == 0 {
		return extractPodIDNoAssociations(ctx, attrs)
	}

	connectionIP := clientutil.Address(client.FromContext(ctx))
	for _, asso := range associations {
		skip := false

		ret := kube.PodIdentifier{}
		for i, source := range asso.Sources {
			// If association configured to take IP address from connection
			switch source.From {
			case kube.ConnectionSource:
				if connectionIP == "" {
					skip = true
					break
				}
				ret[i] = kube.PodIdentifierAttributeFromConnection(connectionIP)
			case kube.ResourceSource:
				// Extract values based on configured resource_attribute.
				attributeValue := stringAttributeFromMap(attrs, source.Name)
				if attributeValue == "" {
					skip = true
					break
				}

				// If association configured by resource_attribute
				// In k8s environment, host.name label set to a pod IP address.
				// If the value doesn't represent an IP address, we skip it.
				if source.Name == string(conventions.HostNameKey) && net.ParseIP(attributeValue) == nil {
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
	podIP = stringAttributeFromMap(attrs, string(conventions.K8SPodIPKey))
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

	connectionIP := clientutil.Address(client.FromContext(ctx))
	if connectionIP != "" {
		return kube.PodIdentifier{
			kube.PodIdentifierAttributeFromConnection(connectionIP),
		}
	}

	hostname := stringAttributeFromMap(attrs, string(conventions.HostNameKey))
	if net.ParseIP(hostname) != nil {
		return kube.PodIdentifier{
			kube.PodIdentifierAttributeFromConnection(hostname),
		}
	}

	return kube.PodIdentifier{}
}

// buildPodIdentifierString returns a low-cardinality string representing which sources
// were used to build the PodIdentifier, formatted as "from" or "from/name" for each
// non-empty slot, joined by "+". Actual identifier values are intentionally excluded
// to avoid unbounded metric cardinality.
// Examples: "connection", "resource_attribute/k8s.pod.ip",
// "resource_attribute/k8s.pod.uid+resource_attribute/container.id"
func buildPodIdentifierString(id kube.PodIdentifier) string {
	var parts []string
	for _, attr := range id {
		if attr.Source.From == "" {
			break
		}
		if attr.Source.Name != "" {
			parts = append(parts, attr.Source.From+"/"+attr.Source.Name)
		} else {
			parts = append(parts, attr.Source.From)
		}
	}
	return strings.Join(parts, "+")
}

func stringAttributeFromMap(attrs pcommon.Map, key string) string {
	if val, ok := attrs.Get(key); ok {
		if val.Type() == pcommon.ValueTypeStr {
			return val.Str()
		}
	}
	return ""
}
