// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package containerapps // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/containerapps"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	gocontribdetector "go.opentelemetry.io/contrib/detectors/azure/azurecontainerapps"
	"go.opentelemetry.io/otel/attribute"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/sdkbridge"
)

const (
	// TypeStr is type of detector.
	TypeStr = "azurecontainerapps"

	// azure.container_app.instance.id is not part of the semantic conventions yet
	// but is emitted by the Go SDK detector.
	attrContainerAppInstanceID = "azure.container_app.instance.id"
)

var _ internal.Detector = (*Detector)(nil)

// Detector detects resource attributes when running on Azure Container Apps.
type Detector struct {
	sdkDetector sdkresource.Detector
}

// NewDetector creates a new Azure Container Apps detector.
func NewDetector(_ processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	ra := dcfg.(Config).ResourceAttributes

	enabled := map[string]bool{
		string(conventions.CloudProviderKey): ra.CloudProvider.Enabled,
		string(conventions.CloudPlatformKey): ra.CloudPlatform.Enabled,
		string(conventions.ServiceNameKey):   ra.ServiceName.Enabled,
		attrContainerAppInstanceID:           ra.AzureContainerAppInstanceID.Enabled,
	}

	return &Detector{
		sdkDetector: gocontribdetector.NewResourceDetector(
			gocontribdetector.WithAttributeFilter(func(kv attribute.KeyValue) bool {
				return enabled[string(kv.Key)]
			}),
		),
	}, nil
}

// Detect returns resource attributes when running on Azure Container Apps.
// Returns an empty resource when not running on Azure Container Apps.
func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	return sdkbridge.Detect(ctx, d.sdkDetector)
}
