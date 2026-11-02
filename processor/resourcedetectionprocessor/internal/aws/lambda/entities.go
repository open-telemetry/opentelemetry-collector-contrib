// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_aws_lambda) && !omit_detector_aws_lambda

package lambda // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/lambda"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	xentity "go.opentelemetry.io/collector/pdata/xpdata/entity"
	conventions "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

var _ internal.EntityDetector = (*detector)(nil)

// Entities implements internal.EntityDetector.
//
// A Lambda function name is unique within an account and region. The
// environment exposes the region but not the account, so this detector
// declares faas scoped by cloud.region and the cloud.region entity itself,
// but not cloud. The chain stays unresolved until a component that knows the
// account adds the cloud entity.
func (d *detector) Entities(res pcommon.Resource) []internal.DetectedEntity {
	attrs := res.Attributes()
	if !internal.HasKeys(attrs, string(conventions.FaaSNameKey)) {
		return nil
	}
	var entities []internal.DetectedEntity
	if internal.HasKeys(attrs, string(conventions.CloudRegionKey)) {
		entities = append(entities, internal.DetectedEntity{
			SchemaURL:         conventions.SchemaURL,
			Type:              internal.EntityTypeCloudRegion,
			IDKeys:            []string{string(conventions.CloudRegionKey)},
			IDScope:           xentity.IdScopeLocal,
			IDScopeEntityType: internal.EntityTypeCloud,
		})
	}
	scope, scopeType := internal.ScopedBy(attrs, internal.ScopeCloudRegion)
	return append(entities, internal.DetectedEntity{
		SchemaURL:         conventions.SchemaURL,
		Type:              internal.EntityTypeFaas,
		IDKeys:            []string{string(conventions.FaaSNameKey)},
		IDScope:           scope,
		IDScopeEntityType: scopeType,
		DescriptionKeys: internal.PresentKeys(attrs,
			string(conventions.FaaSVersionKey),
			string(conventions.FaaSInstanceKey),
			string(conventions.FaaSMaxMemoryKey)),
	})
}
