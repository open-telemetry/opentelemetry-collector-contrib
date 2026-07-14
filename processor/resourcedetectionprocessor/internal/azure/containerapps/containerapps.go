// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package containerapps // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/containerapps"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	gocontribdetector "go.opentelemetry.io/contrib/detectors/azure/azurecontainerapps"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/containerapps/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/sdkbridge"
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
func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	return sdkbridge.Detect(ctx, gocontribdetector.NewResourceDetector(), map[string]bool{
		"cloud.provider":                  d.resourceAttributes.CloudProvider.Enabled,
		"cloud.platform":                  d.resourceAttributes.CloudPlatform.Enabled,
		"service.name":                    d.resourceAttributes.ServiceName.Enabled,
		"azure.container_app.instance.id": d.resourceAttributes.AzureContainerAppInstanceID.Enabled,
	})
}
