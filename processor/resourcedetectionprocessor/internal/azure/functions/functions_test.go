// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package functions

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

func setFunctionsEnv(t *testing.T) {
	t.Setenv("FUNCTIONS_WORKER_RUNTIME", "dotnet-isolated")
	t.Setenv("FUNCTIONS_EXTENSION_VERSION", "")
	t.Setenv("WEBSITE_SITE_NAME", "my-function-app")
	t.Setenv("WEBSITE_RESOURCE_GROUP", "my-rg")
	t.Setenv("WEBSITE_OWNER_NAME", "00000000-0000-0000-0000-000000000000+my-rg-CentralUSwebspace")
	t.Setenv("REGION_NAME", "Central US")
	t.Setenv("WEBSITE_SLOT_NAME", "staging")
	t.Setenv("WEBSITE_INSTANCE_ID", "instance-1")
}

func TestNewDetector(t *testing.T) {
	detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)
	assert.NotNil(t, detector)
}

func TestDetector_Detect_AzureFunctions(t *testing.T) {
	setFunctionsEnv(t)
	// deployment.environment.name is opt-in; enable it here to exercise the full attribute
	// mapping.
	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.DeploymentEnvironmentName.Enabled = true
	detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)

	res, schemaURL, err := detector.Detect(t.Context())
	require.NoError(t, err)
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
	assert.Equal(t, map[string]any{
		"azure.resource_group.name":   "my-rg",
		"cloud.account.id":            "00000000-0000-0000-0000-000000000000",
		"cloud.platform":              "azure.functions",
		"cloud.provider":              "azure",
		"cloud.region":                "Central US",
		"cloud.resource_id":           "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-rg/providers/Microsoft.Web/sites/my-function-app",
		"deployment.environment.name": "staging",
		"faas.instance":               "instance-1",
		"service.name":                "my-function-app",
	}, res.Attributes().AsRaw())
}

func TestDetector_Detect_FlexConsumption(t *testing.T) {
	setFunctionsEnv(t)
	t.Setenv("WEBSITE_RESOURCE_GROUP", "")
	t.Setenv("WEBSITE_INSTANCE_ID", "")
	t.Setenv("WEBSITE_POD_NAME", "pod-1")
	detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), true)
	require.NoError(t, err)

	res, _, err := detector.Detect(t.Context())
	require.NoError(t, err)
	_, hasResourceGroup := res.Attributes().Get("azure.resource_group.name")
	_, hasResourceID := res.Attributes().Get("cloud.resource_id")
	assert.False(t, hasResourceGroup)
	assert.False(t, hasResourceID)
	instance, ok := res.Attributes().Get("faas.instance")
	require.True(t, ok)
	assert.Equal(t, "pod-1", instance.Str())
}

func TestDetector_Detect_NotAzureFunctions(t *testing.T) {
	t.Setenv("FUNCTIONS_WORKER_RUNTIME", "")
	t.Setenv("FUNCTIONS_EXTENSION_VERSION", "")
	t.Setenv("WEBSITE_SITE_NAME", "my-app-service")
	detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := detector.Detect(t.Context())
	require.NoError(t, err)
	assert.Empty(t, schemaURL)
	assert.Equal(t, 0, res.Attributes().Len())
}

func TestDetector_Detect_NotAzureFunctions_FailOnMissingMetadata(t *testing.T) {
	t.Setenv("FUNCTIONS_WORKER_RUNTIME", "")
	t.Setenv("FUNCTIONS_EXTENSION_VERSION", "")
	detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), true)
	require.NoError(t, err)

	res, schemaURL, err := detector.Detect(t.Context())
	require.ErrorContains(t, err, "azure functions metadata unavailable")
	assert.Empty(t, schemaURL)
	assert.Equal(t, 0, res.Attributes().Len())
}

func TestDetector_Detect_AllAttributesDisabled_FailOnMissingMetadata(t *testing.T) {
	setFunctionsEnv(t)
	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.AzureResourceGroupName.Enabled = false
	cfg.ResourceAttributes.CloudAccountID.Enabled = false
	cfg.ResourceAttributes.CloudPlatform.Enabled = false
	cfg.ResourceAttributes.CloudProvider.Enabled = false
	cfg.ResourceAttributes.CloudRegion.Enabled = false
	cfg.ResourceAttributes.CloudResourceID.Enabled = false
	cfg.ResourceAttributes.DeploymentEnvironmentName.Enabled = false
	cfg.ResourceAttributes.FaasInstance.Enabled = false
	cfg.ResourceAttributes.ServiceName.Enabled = false
	detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, true)
	require.NoError(t, err)

	res, _, err := detector.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 0, res.Attributes().Len())
}

func TestDetector_Detect_SDKError(t *testing.T) {
	orig := newResourceDetector
	t.Cleanup(func() { newResourceDetector = orig })
	newResourceDetector = func() sdkresource.Detector {
		return errSDKDetector{err: errors.New("sdk detector failed")}
	}

	detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)
	res, schemaURL, err := detector.Detect(t.Context())
	require.ErrorContains(t, err, "sdk detector failed")
	assert.Empty(t, schemaURL)
	assert.Equal(t, 0, res.Attributes().Len())
}

func TestDetect_ResourceAttributesDisabled(t *testing.T) {
	setFunctionsEnv(t)
	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.ServiceName.Enabled = false
	detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)

	res, _, err := detector.Detect(t.Context())
	require.NoError(t, err)
	_, hasServiceName := res.Attributes().Get("service.name")
	assert.False(t, hasServiceName)
	assert.Equal(t, 7, res.Attributes().Len())
}
