// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package containerapps // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/containerapps"

import (
	"context"
	"os"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/containerapps/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "azurecontainerapps"

	// Environment variables that are set when running in Azure Container Apps.
	// https://learn.microsoft.com/en-us/azure/container-apps/environment-variables?tabs=portal#built-in-environment-variables
	containerAppNameEnvVar        = "CONTAINER_APP_NAME"
	containerAppReplicaNameEnvVar = "CONTAINER_APP_REPLICA_NAME"
)

var _ internal.Detector = (*Detector)(nil)

type Detector struct {
	resourceAttributes metadata.ResourceAttributesConfig
}

// NewDetector creates a new Azure Container Apps detector.
func NewDetector(_ processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)
	return &Detector{resourceAttributes: cfg.ResourceAttributes}, nil
}

// Detect returns Azure Container Apps resource attributes when the
// CONTAINER_APP_NAME environment variable is set. Otherwise it returns an
// empty resource and no error.
func (d *Detector) Detect(_ context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	appName := os.Getenv(containerAppNameEnvVar)
	if appName == "" {
		return pcommon.NewResource(), "", nil
	}

	rb := metadata.NewResourceBuilder(d.resourceAttributes)
	rb.SetCloudProvider(conventions.CloudProviderAzure.Value.AsString())
	rb.SetCloudPlatform(conventions.CloudPlatformAzureContainerApps.Value.AsString())
	rb.SetServiceName(appName)
	if replicaName := os.Getenv(containerAppReplicaNameEnvVar); replicaName != "" {
		rb.SetServiceInstanceID(replicaName)
	}

	return rb.Emit(), conventions.SchemaURL, nil
}
