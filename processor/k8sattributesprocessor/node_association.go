// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
)

func extractNodeID(attrs pcommon.Map, associations []kube.Association) kube.NodeIdentifier {
	if len(associations) == 0 {
		return extractNodeIDNoAssociations(attrs)
	}

	for _, asso := range associations {
		skip := false

		ret := kube.NodeIdentifier{}
		for i, source := range asso.Sources {
			switch source.From {
			case kube.ResourceSource:
				// Extract values on configured resource_attribute.
				attributeValue := stringAttributeFromMap(attrs, source.Name)
				if attributeValue == "" {
					skip = true
					break
				}

				ret[i] = kube.NodeIdentifierAttributeFromSource(source, attributeValue)
			}
		}

		// If all associations sources has been resolved, return result
		if !skip {
			return ret
		}
	}
	return kube.NodeIdentifier{}
}

func extractNodeIDNoAssociations(attrs pcommon.Map) kube.NodeIdentifier {
	nodeName := stringAttributeFromMap(attrs, string(conventions.K8SNodeNameKey))
	if nodeName != "" {
		return kube.NodeIdentifier{
			kube.NodeIdentifierAttributeFromResourceAttribute(string(conventions.K8SNodeNameKey), nodeName),
		}
	}

	return kube.NodeIdentifier{}
}
