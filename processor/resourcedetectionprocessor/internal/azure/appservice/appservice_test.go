// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package appservice

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
)

type errSDKDetector struct {
	err error
}

func (d errSDKDetector) Detect(context.Context) (*sdkresource.Resource, error) {
	return nil, d.err
}

func setAppServiceEnv(t *testing.T) {
	t.Setenv("WEBSITE_SITE_NAME", "my-site")
	t.Setenv("WEBSITE_RESOURCE_GROUP", "my-rg")
	t.Setenv("WEBSITE_OWNER_NAME", "00000000-0000-0000-0000-000000000000+my-rg-CentralUSwebspace")
	t.Setenv("REGION_NAME", "Central US")
	t.Setenv("WEBSITE_SLOT_NAME", "staging")
	t.Setenv("WEBSITE_INSTANCE_ID", "instance-1")
	t.Setenv("FUNCTIONS_WORKER_RUNTIME", "")
}

func TestNewDetector(t *testing.T) {
	dcfg := CreateDefaultConfig()
	appServiceDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), dcfg, false)
	require.NoError(t, err)
	assert.NotNil(t, appServiceDetector)
}

func TestDetector_Detect_AppService(t *testing.T) {
	setAppServiceEnv(t)
	appServiceDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := appServiceDetector.Detect(t.Context())
	require.NoError(t, err)
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
	assert.Equal(t, map[string]any{
		"cloud.provider":                "azure",
		"cloud.platform":                "azure.app_service",
		"cloud.account.id":              "00000000-0000-0000-0000-000000000000",
		"cloud.region":                  "Central US",
		"cloud.resource_id":             "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-rg/providers/Microsoft.Web/sites/my-site",
		"deployment.environment.name":   "staging",
		"service.name":                  "my-site",
		"azure.resource_group.name":     "my-rg",
		"azure.app_service.instance.id": "instance-1",
	}, res.Attributes().AsRaw(), "Resource attributes returned are incorrect")
}

// WEBSITE_INSTANCE_ID is optional; its absence is a partial resource, not an error.
func TestDetector_Detect_AppService_NoInstanceID(t *testing.T) {
	setAppServiceEnv(t)
	t.Setenv("WEBSITE_INSTANCE_ID", "")
	appServiceDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, _, err := appServiceDetector.Detect(t.Context())
	require.NoError(t, err)
	_, hasInstanceID := res.Attributes().Get("azure.app_service.instance.id")
	assert.False(t, hasInstanceID, "azure.app_service.instance.id should be absent when WEBSITE_INSTANCE_ID is not set")
	assert.Equal(t, 8, res.Attributes().Len())
}

func TestDetector_Detect_NotAppService(t *testing.T) {
	t.Setenv("WEBSITE_SITE_NAME", "")
	t.Setenv("WEBSITE_RESOURCE_GROUP", "")
	t.Setenv("WEBSITE_OWNER_NAME", "")
	appServiceDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := appServiceDetector.Detect(t.Context())
	require.NoError(t, err)
	assert.Empty(t, schemaURL)
	assert.Equal(t, 0, res.Attributes().Len(), "Resource object should be empty")
}

func TestDetector_Detect_NotAppService_FailOnMissingMetadata(t *testing.T) {
	t.Setenv("WEBSITE_SITE_NAME", "")
	t.Setenv("WEBSITE_RESOURCE_GROUP", "")
	t.Setenv("WEBSITE_OWNER_NAME", "")
	appServiceDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), true)
	require.NoError(t, err)

	res, schemaURL, err := appServiceDetector.Detect(t.Context())
	require.ErrorContains(t, err, "azure app service metadata unavailable")
	assert.Empty(t, schemaURL)
	assert.Equal(t, 0, res.Attributes().Len())
}

// Azure Functions sets the same site env vars but belongs to the Functions detector.
func TestDetector_Detect_AzureFunctions(t *testing.T) {
	setAppServiceEnv(t)
	t.Setenv("FUNCTIONS_WORKER_RUNTIME", "dotnet-isolated")
	appServiceDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := appServiceDetector.Detect(t.Context())
	require.NoError(t, err)
	assert.Empty(t, schemaURL)
	assert.Equal(t, 0, res.Attributes().Len(), "Azure Functions apps should not be detected as App Service")
}

func TestDetector_Detect_AzureFunctions_FailOnMissingMetadata(t *testing.T) {
	setAppServiceEnv(t)
	t.Setenv("FUNCTIONS_WORKER_RUNTIME", "dotnet-isolated")
	appServiceDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), true)
	require.NoError(t, err)

	res, _, err := appServiceDetector.Detect(t.Context())
	require.ErrorContains(t, err, "azure app service metadata unavailable")
	assert.Equal(t, 0, res.Attributes().Len())
}

func TestDetector_Detect_AppService_AllAttributesDisabled_FailOnMissingMetadata(t *testing.T) {
	// All attributes disabled is not the same as not being on the platform.
	setAppServiceEnv(t)
	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.AzureAppServiceInstanceID.Enabled = false
	cfg.ResourceAttributes.AzureResourceGroupName.Enabled = false
	cfg.ResourceAttributes.CloudAccountID.Enabled = false
	cfg.ResourceAttributes.CloudPlatform.Enabled = false
	cfg.ResourceAttributes.CloudProvider.Enabled = false
	cfg.ResourceAttributes.CloudRegion.Enabled = false
	cfg.ResourceAttributes.CloudResourceID.Enabled = false
	cfg.ResourceAttributes.DeploymentEnvironmentName.Enabled = false
	cfg.ResourceAttributes.ServiceName.Enabled = false
	appServiceDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, true)
	require.NoError(t, err)

	res, _, err := appServiceDetector.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 0, res.Attributes().Len())
}

func TestDetector_Detect_SDKError(t *testing.T) {
	setAppServiceEnv(t)
	orig := newResourceDetector
	t.Cleanup(func() { newResourceDetector = orig })
	newResourceDetector = func() sdkresource.Detector {
		return errSDKDetector{err: errors.New("sdk detector failed")}
	}

	for _, failOnMissingMetadata := range []bool{false, true} {
		appServiceDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), failOnMissingMetadata)
		require.NoError(t, err)

		res, schemaURL, err := appServiceDetector.Detect(t.Context())
		require.ErrorContains(t, err, "sdk detector failed")
		assert.Empty(t, schemaURL)
		assert.Equal(t, 0, res.Attributes().Len())
	}
}

func TestDetect_ResourceAttributesDisabled(t *testing.T) {
	setAppServiceEnv(t)
	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.ServiceName.Enabled = false
	appServiceDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)

	res, _, err := appServiceDetector.Detect(t.Context())
	require.NoError(t, err)
	_, hasServiceName := res.Attributes().Get("service.name")
	assert.False(t, hasServiceName, "service.name should be absent when disabled in config")
	assert.Equal(t, 8, res.Attributes().Len())
}
