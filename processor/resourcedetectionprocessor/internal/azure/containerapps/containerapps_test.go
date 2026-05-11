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
	containerAppDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), dcfg)
	require.NoError(t, err)
	assert.NotNil(t, containerAppDetector)
}

func TestDetector_Detect_ContainerApp(t *testing.T) {
	t.Setenv("CONTAINER_APP_NAME", "my-app")
	t.Setenv("CONTAINER_APP_REPLICA_NAME", "my-app--abc123-7d9f8c5b6-xyz")
	containerAppDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)
	res, schemaURL, err := containerAppDetector.Detect(t.Context())
	require.NoError(t, err)
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
	assert.Equal(t, map[string]any{
		"cloud.provider":      "azure",
		"cloud.platform":      "azure.container_apps",
		"service.name":        "my-app",
		"service.instance.id": "my-app--abc123-7d9f8c5b6-xyz",
	}, res.Attributes().AsRaw(), "Resource attributes returned are incorrect")
}

func TestDetector_Detect_NotContainerApp(t *testing.T) {
	t.Setenv("CONTAINER_APP_NAME", "")
	t.Setenv("CONTAINER_APP_REPLICA_NAME", "")
	containerAppDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)
	res, schemaURL, err := containerAppDetector.Detect(t.Context())
	require.NoError(t, err)
	assert.Empty(t, schemaURL)
	assert.Equal(t, 0, res.Attributes().Len(), "Resource object should be empty")
}
