// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_system) && !omit_detector_system

package system // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/system"

import (
	"sort"

	"go.opentelemetry.io/collector/pdata/pcommon"
	xentity "go.opentelemetry.io/collector/pdata/xpdata/entity"
	conventions "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

var _ internal.EntityDetector = (*Detector)(nil)

// Entities implements internal.EntityDetector.
//
// host.id is the machine ID (/etc/machine-id, IOPlatformUUID, MachineGuid),
// which is generated to be unique on its own, so the host entity it identifies
// has a global ID scope. When only host.name is available the scope is left
// unspecified: a hostname is unique within some network the detector cannot
// name.
func (d *Detector) Entities(res pcommon.Resource) []internal.DetectedEntity {
	attrs := res.Attributes()
	ent := internal.DetectedEntity{
		SchemaURL: conventions.SchemaURL,
		Type:      internal.EntityTypeHost,
	}
	switch {
	case internal.HasKeys(attrs, string(conventions.HostIDKey)):
		ent.IDKeys = []string{string(conventions.HostIDKey)}
		ent.IDScope = xentity.IdScopeGlobal
	case internal.HasKeys(attrs, string(conventions.HostNameKey)):
		ent.IDKeys = []string{string(conventions.HostNameKey)}
		ent.IDScope = xentity.IdScopeUnspecified
	default:
		return nil
	}
	for k := range attrs.All() {
		if k != ent.IDKeys[0] {
			ent.DescriptionKeys = append(ent.DescriptionKeys, k)
		}
	}
	sort.Strings(ent.DescriptionKeys)
	return []internal.DetectedEntity{ent}
}
