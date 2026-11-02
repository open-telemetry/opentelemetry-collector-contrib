// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_ec2) && !omit_detector_ec2

package ec2 // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ec2"

import (
	"sort"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

var _ internal.EntityDetector = (*Detector)(nil)

// Entities implements internal.EntityDetector.
//
// host.id is the EC2 instance ID. AWS documents resource IDs as tied to a
// Region, so the host is scoped by cloud.region, whose own chain carries the
// account. Without a region the scope is unspecified.
func (d *Detector) Entities(res pcommon.Resource) []internal.DetectedEntity {
	attrs := res.Attributes()
	entities := internal.CloudEntities(attrs, conventions.SchemaURL)

	if internal.HasKeys(attrs, string(conventions.HostIDKey)) {
		desc := internal.PresentKeys(attrs,
			string(conventions.HostNameKey),
			string(conventions.HostTypeKey),
			string(conventions.HostImageIDKey))
		var tags []string
		for k := range attrs.All() {
			if strings.HasPrefix(k, tagPrefix) {
				tags = append(tags, k)
			}
		}
		sort.Strings(tags)
		scope, scopeType := internal.ScopedBy(attrs, internal.ScopeCloudRegion)
		entities = append(entities, internal.DetectedEntity{
			SchemaURL:         conventions.SchemaURL,
			Type:              internal.EntityTypeHost,
			IDKeys:            []string{string(conventions.HostIDKey)},
			IDScope:           scope,
			IDScopeEntityType: scopeType,
			DescriptionKeys:   append(desc, tags...),
		})
	}
	return entities
}
