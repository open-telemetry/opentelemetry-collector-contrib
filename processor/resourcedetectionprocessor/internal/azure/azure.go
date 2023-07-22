// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azure // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/azure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "azure"
)

var _ internal.Detector = (*Detector)(nil)

// Detector is an Azure metadata detector
type Detector struct {
	provider           azure.Provider
	logger             *zap.Logger
	resourceAttributes metadata.ResourceAttributesConfig
}

// NewDetector creates a new Azure metadata detector
func NewDetector(p processor.CreateSettings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)
	return &Detector{
		provider:           azure.NewProvider(),
		logger:             p.Logger,
		resourceAttributes: cfg.ResourceAttributes,
	}, nil
}

// Detect detects system metadata and returns a resource with the available ones
func (d *Detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	compute, err := d.provider.Metadata(ctx)
	if err != nil {
		d.logger.Debug("Azure detector metadata retrieval failed", zap.Error(err))
		// return an empty Resource and no error
		return pcommon.NewResource(), "", nil
	}

	rb := metadata.NewResourceBuilder(d.resourceAttributes)

	rb.SetCloudProvider(conventions.AttributeCloudProviderAzure)
	rb.SetCloudPlatform(conventions.AttributeCloudPlatformAzureVM)
	rb.SetHostName(compute.Name)
	rb.SetCloudRegion(compute.Location)
	rb.SetHostID(compute.VMID)
	rb.SetCloudAccountID(compute.SubscriptionID)

	// Also save compute.Name in "azure.vm.name" as host.id (AttributeHostName) is
	// used by system detector.
	rb.SetAzureVMName(compute.Name)
	rb.SetAzureVMSize(compute.VMSize)
	rb.SetAzureVMScalesetName(compute.VMScaleSetName)
	rb.SetAzureResourcegroupName(compute.ResourceGroupName)

	return rb.Emit(), conventions.SchemaURL, nil
}
