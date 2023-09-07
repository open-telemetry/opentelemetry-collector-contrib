// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aks // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/aks"

import (
	"context"
	"os"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/azure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/aks/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "aks"

	// Environment variable that is set when running on Kubernetes
	kubernetesServiceHostEnvVar = "KUBERNETES_SERVICE_HOST"
)

type Detector struct {
	provider           azure.Provider
	resourceAttributes metadata.ResourceAttributesConfig
}

// NewDetector creates a new AKS detector
func NewDetector(_ processor.CreateSettings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)
	return &Detector{provider: azure.NewProvider(), resourceAttributes: cfg.ResourceAttributes}, nil
}

func (d *Detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	res := pcommon.NewResource()

	if !onK8s() {
		return res, "", nil
	}

	// If we can't get a response from the metadata endpoint, we're not running in Azure
	if !azureMetadataAvailable(ctx, d.provider) {
		return res, "", nil
	}

	attrs := res.Attributes()
	if d.resourceAttributes.CloudProvider.Enabled {
		attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAzure)
	}
	if d.resourceAttributes.CloudPlatform.Enabled {
		attrs.PutStr(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAzureAKS)
	}

	return res, conventions.SchemaURL, nil
}

func onK8s() bool {
	return os.Getenv(kubernetesServiceHostEnvVar) != ""
}

func azureMetadataAvailable(ctx context.Context, p azure.Provider) bool {
	_, err := p.Metadata(ctx)
	return err == nil
}
