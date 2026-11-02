// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_k8s_api) && !omit_detector_k8s_api

package k8sapi // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/k8sapi"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	xentity "go.opentelemetry.io/collector/pdata/xpdata/entity"
	conventions "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

var _ internal.EntityDetector = (*detector)(nil)

// Entities implements internal.EntityDetector.
//
// Kubernetes UIDs are UUIDs, so k8s.node and k8s.cluster identified by UID have
// a global ID scope. A node identified only by name is unique within its
// cluster. The cluster entity is not required to be present for that: another
// detector (for example gcp) may add it, which completes the node's Global
// Identity without this detector knowing about it. host.id is the node's
// machine ID, global.
func (d *detector) Entities(res pcommon.Resource) []internal.DetectedEntity {
	attrs := res.Attributes()
	var entities []internal.DetectedEntity

	node := internal.DetectedEntity{SchemaURL: conventions.SchemaURL, Type: internal.EntityTypeK8sNode}
	switch {
	case internal.HasKeys(attrs, string(conventions.K8SNodeUIDKey)):
		node.IDKeys = []string{string(conventions.K8SNodeUIDKey)}
		node.IDScope = xentity.IdScopeGlobal
		node.DescriptionKeys = internal.PresentKeys(attrs, string(conventions.K8SNodeNameKey))
	case internal.HasKeys(attrs, string(conventions.K8SNodeNameKey)):
		node.IDKeys = []string{string(conventions.K8SNodeNameKey)}
		node.IDScope = xentity.IdScopeLocal
		node.IDScopeEntityType = internal.EntityTypeK8sCluster
	}
	if len(node.IDKeys) > 0 {
		entities = append(entities, node)
	}

	if internal.HasKeys(attrs, string(conventions.K8SClusterUIDKey)) {
		entities = append(entities, internal.DetectedEntity{
			SchemaURL: conventions.SchemaURL,
			Type:      internal.EntityTypeK8sCluster,
			IDKeys:    []string{string(conventions.K8SClusterUIDKey)},
			IDScope:   xentity.IdScopeGlobal,
		})
	}

	if internal.HasKeys(attrs, string(conventions.HostIDKey)) {
		entities = append(entities, internal.DetectedEntity{
			SchemaURL: conventions.SchemaURL,
			Type:      internal.EntityTypeHost,
			IDKeys:    []string{string(conventions.HostIDKey)},
			IDScope:   xentity.IdScopeGlobal,
		})
	}
	return entities
}
