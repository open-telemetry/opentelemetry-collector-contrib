// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_aws_eks) && !omit_detector_aws_eks

package eks // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

var _ internal.EntityDetector = (*detector)(nil)

// Entities implements internal.EntityDetector.
//
// host.id is the EC2 instance ID of the node (scoped by cloud.region, as in
// the ec2 detector). An EKS cluster name is unique within an account and
// region, so k8s.cluster is scoped by cloud.region too.
func (d *detector) Entities(res pcommon.Resource) []internal.DetectedEntity {
	attrs := res.Attributes()
	entities := internal.CloudEntities(attrs, conventions.SchemaURL)

	if internal.HasKeys(attrs, string(conventions.HostIDKey)) {
		scope, scopeType := internal.ScopedBy(attrs, internal.ScopeCloudRegion)
		entities = append(entities, internal.DetectedEntity{
			SchemaURL:         conventions.SchemaURL,
			Type:              internal.EntityTypeHost,
			IDKeys:            []string{string(conventions.HostIDKey)},
			IDScope:           scope,
			IDScopeEntityType: scopeType,
			DescriptionKeys: internal.PresentKeys(attrs,
				string(conventions.HostNameKey),
				string(conventions.HostTypeKey),
				string(conventions.HostImageIDKey)),
		})
	}
	if internal.HasKeys(attrs, string(conventions.K8SClusterNameKey)) {
		scope, scopeType := internal.ScopedBy(attrs, internal.ScopeCloudRegion)
		entities = append(entities, internal.DetectedEntity{
			SchemaURL:         conventions.SchemaURL,
			Type:              internal.EntityTypeK8sCluster,
			IDKeys:            []string{string(conventions.K8SClusterNameKey)},
			IDScope:           scope,
			IDScopeEntityType: scopeType,
		})
	}
	return entities
}
