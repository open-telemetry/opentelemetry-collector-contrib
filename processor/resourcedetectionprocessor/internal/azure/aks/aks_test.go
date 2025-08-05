// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aks

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/azure"
)

func TestNewDetector(t *testing.T) {
	dcfg := CreateDefaultConfig()
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), dcfg)
	require.NoError(t, err)
	assert.NotNil(t, d)
}

func TestDetector_Detect_K8s_Azure(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "localhost")
	resourceAttributes := CreateDefaultConfig().ResourceAttributes
	detector := &Detector{provider: mockProvider(), resourceAttributes: resourceAttributes}
	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	assert.Equal(t, map[string]any{
		"cloud.provider": "azure",
		"cloud.platform": "azure_aks",
	}, res.Attributes().AsRaw(), "Resource attrs returned are incorrect")
}

func TestDetector_Detect_K8s_NonAzure(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "localhost")
	mp := &azure.MockProvider{}
	mp.On("Metadata").Return(nil, errors.New(""))
	resourceAttributes := CreateDefaultConfig().ResourceAttributes
	detector := &Detector{provider: mp, resourceAttributes: resourceAttributes}
	res, _, err := detector.Detect(context.Background())
	require.NoError(t, err)
	attrs := res.Attributes()
	assert.Equal(t, 0, attrs.Len())
}

func TestDetector_Detect_NonK8s(t *testing.T) {
	os.Clearenv()
	resourceAttributes := CreateDefaultConfig().ResourceAttributes
	detector := &Detector{provider: mockProvider(), resourceAttributes: resourceAttributes}
	res, _, err := detector.Detect(context.Background())
	require.NoError(t, err)
	attrs := res.Attributes()
	assert.Equal(t, 0, attrs.Len())
}

func mockProvider() *azure.MockProvider {
	mp := &azure.MockProvider{}
	mp.On("Metadata").Return(&azure.ComputeMetadata{}, nil)
	return mp
}

func TestParseClusterName(t *testing.T) {
	cases := []struct {
		name          string
		resourceGroup string
		expected      string
	}{
		{
			name:          "Return cluster name",
			resourceGroup: "MC_myResourceGroup_AKSCluster_eastus",
			expected:      "AKSCluster",
		},
		{
			name:          "Return resource group name, resource group contains underscores",
			resourceGroup: "MC_Resource_Group_AKSCluster_eastus",
			expected:      "MC_Resource_Group_AKSCluster_eastus",
		},
		{
			name:          "Return resource group name, cluster name contains underscores",
			resourceGroup: "MC_myResourceGroup_AKS_Cluster_eastus",
			expected:      "MC_myResourceGroup_AKS_Cluster_eastus",
		},
		{
			name:          "Custom infrastructure resource group name, return resource group name",
			resourceGroup: "infra-group_name",
			expected:      "infra-group_name",
		},
		{
			name:          "Custom infrastructure resource group name with four underscores, return resource group name",
			resourceGroup: "dev_infra_group_name",
			expected:      "dev_infra_group_name",
		},
		// This case is unlikely because it would require the user to create
		// a custom infrastructure resource group with the MC prefix and the
		// correct number of underscores.
		{
			name:          "Custom infrastructure resource group name with MC prefix",
			resourceGroup: "MC_group_name_location",
			expected:      "name",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual := parseClusterName(tc.resourceGroup)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
