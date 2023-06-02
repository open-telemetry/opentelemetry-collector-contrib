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
	d, err := NewDetector(processortest.NewNopCreateSettings(), nil)
	require.NoError(t, err)
	assert.NotNil(t, d)
}

func TestDetector_Detect_K8s_Azure(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "localhost")
	detector := &Detector{provider: mockProvider()}
	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	assert.Equal(t, map[string]interface{}{
		"cloud.provider": "azure",
		"cloud.platform": "azure_aks",
	}, res.Attributes().AsRaw(), "Resource attrs returned are incorrect")
}

func TestDetector_Detect_K8s_NonAzure(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "localhost")
	mp := &azure.MockProvider{}
	mp.On("Metadata").Return(nil, errors.New(""))
	detector := &Detector{provider: mp}
	res, _, err := detector.Detect(context.Background())
	require.NoError(t, err)
	attrs := res.Attributes()
	assert.Equal(t, 0, attrs.Len())
}

func TestDetector_Detect_NonK8s(t *testing.T) {
	os.Clearenv()
	detector := &Detector{provider: mockProvider()}
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
