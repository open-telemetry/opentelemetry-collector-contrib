// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package containerapps

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestNewDetector(t *testing.T) {
	dcfg := CreateDefaultConfig()
	containerAppDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), dcfg, false)
	require.NoError(t, err)
	assert.NotNil(t, containerAppDetector)
}

func TestDetector_Detect_ContainerApp(t *testing.T) {
	t.Setenv("CONTAINER_APP_NAME", "my-app")
	t.Setenv("CONTAINER_APP_REPLICA_NAME", "my-app--abc123-7d9f8c5b6-xyz")
	containerAppDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := containerAppDetector.Detect(t.Context())
	require.NoError(t, err)
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
	assert.Equal(t, map[string]any{
		"cloud.provider":                  "azure",
		"cloud.platform":                  "azure.container_apps",
		"service.name":                    "my-app",
		"azure.container_app.instance.id": "my-app--abc123-7d9f8c5b6-xyz",
	}, res.Attributes().AsRaw(), "Resource attributes returned are incorrect")
}

func TestDetector_Detect_NotContainerApp(t *testing.T) {
	t.Setenv("CONTAINER_APP_NAME", "")
	t.Setenv("CONTAINER_APP_REPLICA_NAME", "")
	containerAppDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := containerAppDetector.Detect(t.Context())
	require.NoError(t, err)
	assert.Empty(t, schemaURL)
	assert.Equal(t, 0, res.Attributes().Len(), "Resource object should be empty")
}

func TestDetector_Detect_NotContainerApp_FailOnMissingMetadata(t *testing.T) {
	t.Setenv("CONTAINER_APP_NAME", "")
	t.Setenv("CONTAINER_APP_REPLICA_NAME", "")
	containerAppDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), true)
	require.NoError(t, err)

	res, schemaURL, err := containerAppDetector.Detect(t.Context())
	require.ErrorContains(t, err, "azure container apps metadata unavailable")
	assert.Empty(t, schemaURL)
	assert.Equal(t, 0, res.Attributes().Len())
}

func TestDetector_Detect_ContainerApp_AllAttributesDisabled_FailOnMissingMetadata(t *testing.T) {
	// Even if every resource attribute is disabled, the process is running on Azure Container Apps,
	// so this shouldn't be treated as missing metadata.
	t.Setenv("CONTAINER_APP_NAME", "my-app")
	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.CloudProvider.Enabled = false
	cfg.ResourceAttributes.CloudPlatform.Enabled = false
	cfg.ResourceAttributes.ServiceName.Enabled = false
	cfg.ResourceAttributes.AzureContainerAppInstanceID.Enabled = false
	containerAppDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, true)
	require.NoError(t, err)

	res, schemaURL, err := containerAppDetector.Detect(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, schemaURL)
	assert.Equal(t, 0, res.Attributes().Len())
}

func TestDetect_ResourceAttributesDisabled(t *testing.T) {
	t.Setenv("CONTAINER_APP_NAME", "my-app")
	t.Setenv("CONTAINER_APP_REPLICA_NAME", "my-replica")
	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.ServiceName.Enabled = false
	containerAppDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)

	res, schemaURL, err := containerAppDetector.Detect(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, schemaURL)
	_, hasServiceName := res.Attributes().Get("service.name")
	assert.False(t, hasServiceName, "service.name should be absent when disabled in config")
	assert.Equal(t, 3, res.Attributes().Len())
}
