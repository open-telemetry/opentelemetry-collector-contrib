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
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/azure"
)

func TestNewDetector(t *testing.T) {
	dcfg := CreateDefaultConfig()
	d, err := NewDetector(processortest.NewNopCreateSettings(), dcfg)
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
		// Cluster name will not be detected if resource grup is not set
		"k8s.cluster.name": "",
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
		name     string
		input    string
		expected string
	}{
		{
			name:     "parses cluster name",
			input:    "MC_myResourceGroup_myAKSCluster_eastus",
			expected: "myAKSCluster",
		},
		{
			name:     "returns resource group when cluster name has underscores",
			input:    "MC_myResourceGroup_my_AKS_Cluster_eastus",
			expected: "MC_myResourceGroup_my_AKS_Cluster_eastus",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual := parseClusterName(tc.input)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
