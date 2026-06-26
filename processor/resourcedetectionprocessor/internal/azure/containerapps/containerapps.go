// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package containerapps // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/containerapps"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	gocontribdetector "go.opentelemetry.io/contrib/detectors/azure/azurecontainerapps"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/containerapps/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "azurecontainerapps"
)

var _ internal.Detector = (*Detector)(nil)

// Detector detects resource attributes when running on Azure Container Apps.
type Detector struct {
	resourceAttributes metadata.ResourceAttributesConfig
}

// NewDetector creates a new Azure Container Apps detector.
func NewDetector(_ processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)
	return &Detector{resourceAttributes: cfg.ResourceAttributes}, nil
}

// Detect returns resource attributes when running on Azure Container Apps.
// Returns an empty resource when not running on Azure Container Apps.
// ErrPartialResource from the SDK is suppressed; the partial resource is returned without error.
func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	sdkRes, err := gocontribdetector.NewResourceDetector().Detect(ctx)
	if err != nil && !errors.Is(err, sdkresource.ErrPartialResource) {
		return pcommon.NewResource(), "", err
	}

	if sdkRes.Len() == 0 {
		return pcommon.NewResource(), "", nil
	}

	rb := metadata.NewResourceBuilder(d.resourceAttributes)
	iter := sdkRes.Iter()
	for iter.Next() {
		kv := iter.Attribute()
		switch string(kv.Key) {
		case "cloud.provider":
			rb.SetCloudProvider(kv.Value.AsString())
		case "cloud.platform":
			rb.SetCloudPlatform(kv.Value.AsString())
		case "service.name":
			rb.SetServiceName(kv.Value.AsString())
		case "service.instance.id":
			rb.SetServiceInstanceID(kv.Value.AsString())
		}
	}
	return rb.Emit(), sdkRes.SchemaURL(), nil
}
