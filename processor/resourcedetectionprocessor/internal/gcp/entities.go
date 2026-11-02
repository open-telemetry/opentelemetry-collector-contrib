// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_gcp) && !omit_detector_gcp

package gcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp"

import (
	"sort"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

var _ internal.EntityDetector = (*detector)(nil)

// Entities implements internal.EntityDetector.
//
// host.id is the GCE instance ID, a server-assigned number unique within a
// project at least, so host is scoped by cloud. A GKE cluster name is unique
// within a project and a location, which is a zone for zonal clusters and a
// region for regional ones; the detector records which one applies. A Cloud
// Run or Cloud Functions name is unique within a project and region.
func (d *detector) Entities(res pcommon.Resource) []internal.DetectedEntity {
	attrs := res.Attributes()
	entities := internal.CloudEntities(attrs, conventions.SchemaURL)

	if internal.HasKeys(attrs, string(conventions.HostIDKey)) {
		desc := internal.PresentKeys(attrs,
			string(conventions.HostNameKey),
			string(conventions.HostTypeKey))
		var extra []string
		for k := range attrs.All() {
			if strings.HasPrefix(k, "gcp.gce.") {
				extra = append(extra, k)
			}
		}
		sort.Strings(extra)
		scope, scopeType := internal.ScopedBy(attrs, internal.ScopeCloud)
		entities = append(entities, internal.DetectedEntity{
			SchemaURL:         conventions.SchemaURL,
			Type:              internal.EntityTypeHost,
			IDKeys:            []string{string(conventions.HostIDKey)},
			IDScope:           scope,
			IDScopeEntityType: scopeType,
			DescriptionKeys:   append(desc, extra...),
		})
	}

	if internal.HasKeys(attrs, string(conventions.K8SClusterNameKey)) {
		scope, scopeType := internal.ScopedBy(attrs, internal.ScopeCloudAvailabilityZone, internal.ScopeCloudRegion)
		entities = append(entities, internal.DetectedEntity{
			SchemaURL:         conventions.SchemaURL,
			Type:              internal.EntityTypeK8sCluster,
			IDKeys:            []string{string(conventions.K8SClusterNameKey)},
			IDScope:           scope,
			IDScopeEntityType: scopeType,
		})
	}

	if internal.HasKeys(attrs, string(conventions.FaaSNameKey)) {
		scope, scopeType := internal.ScopedBy(attrs, internal.ScopeCloudRegion)
		entities = append(entities, internal.DetectedEntity{
			SchemaURL:         conventions.SchemaURL,
			Type:              internal.EntityTypeFaas,
			IDKeys:            []string{string(conventions.FaaSNameKey)},
			IDScope:           scope,
			IDScopeEntityType: scopeType,
			DescriptionKeys: internal.PresentKeys(attrs,
				string(conventions.FaaSVersionKey),
				string(conventions.FaaSInstanceKey)),
		})
	}
	return entities
}
