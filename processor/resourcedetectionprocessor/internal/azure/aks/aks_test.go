// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aks

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

const schemaURLUnderTest = "https://opentelemetry.io/schemas/1.43.0"

// fakeDetector stands in for the upstream SDK detector. Its metadata endpoint is
// not injectable, so the adapter is exercised against canned SDK results instead
// of a fake metadata server.
type fakeDetector struct {
	res *sdkresource.Resource
	err error
}

func (f fakeDetector) Detect(context.Context) (*sdkresource.Resource, error) {
	return f.res, f.err
}

func newTestDetector(t *testing.T, cfg Config, failOnMissingMetadata bool, res *sdkresource.Resource, err error) internal.Detector {
	t.Helper()

	orig := newResourceDetector
	newResourceDetector = func() sdkresource.Detector {
		return fakeDetector{res: res, err: err}
	}
	t.Cleanup(func() { newResourceDetector = orig })

	d, newErr := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, failOnMissingMetadata)
	require.NoError(t, newErr)
	return d
}

func fullResource() *sdkresource.Resource {
	return sdkresource.NewWithAttributes(schemaURLUnderTest,
		attribute.String("cloud.provider", "azure"),
		attribute.String("cloud.platform", "azure.aks"),
		attribute.String("k8s.cluster.name", "my-cluster"),
	)
}

func clusterNameEnabled() Config {
	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.K8sClusterName.Enabled = true
	return cfg
}

func TestNewDetector(t *testing.T) {
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)
	assert.NotNil(t, d)
}

func TestDetect_ClusterNameEnabled(t *testing.T) {
	t.Setenv(kubernetesServiceHostEnvVar, "localhost")
	d := newTestDetector(t, clusterNameEnabled(), false, fullResource(), nil)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, schemaURLUnderTest, schemaURL)
	assert.Equal(t, map[string]any{
		"cloud.provider":   "azure",
		"cloud.platform":   "azure.aks",
		"k8s.cluster.name": "my-cluster",
	}, res.Attributes().AsRaw())
}

// k8s.cluster.name is disabled by default, so the detector must drop it even
// though the SDK detector always reports it.
func TestDetect_DefaultConfig(t *testing.T) {
	t.Setenv(kubernetesServiceHostEnvVar, "localhost")
	d := newTestDetector(t, CreateDefaultConfig(), false, fullResource(), nil)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, schemaURLUnderTest, schemaURL)
	assert.Equal(t, map[string]any{
		"cloud.provider": "azure",
		"cloud.platform": "azure.aks",
	}, res.Attributes().AsRaw())
}

func TestDetect_DisabledAttributeRemoved(t *testing.T) {
	t.Setenv(kubernetesServiceHostEnvVar, "localhost")
	cfg := clusterNameEnabled()
	cfg.ResourceAttributes.CloudPlatform.Enabled = false
	d := newTestDetector(t, cfg, false, fullResource(), nil)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"cloud.provider":   "azure",
		"k8s.cluster.name": "my-cluster",
	}, res.Attributes().AsRaw())
}

func TestDetect_Failures(t *testing.T) {
	errNoMetadata := errors.New("no metadata")

	tests := []struct {
		name    string
		res     *sdkresource.Resource
		err     error
		wantErr string
	}{
		{name: "empty result", res: sdkresource.Empty(), wantErr: "aks metadata unavailable"},
		{name: "detector error", err: errNoMetadata, wantErr: errNoMetadata.Error()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(kubernetesServiceHostEnvVar, "localhost")

			res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), false, tt.res, tt.err).Detect(t.Context())
			require.NoError(t, err)
			assert.True(t, internal.IsEmptyResource(res))
			assert.Empty(t, schemaURL)

			res, schemaURL, err = newTestDetector(t, CreateDefaultConfig(), true, tt.res, tt.err).Detect(t.Context())
			require.ErrorContains(t, err, tt.wantErr)
			assert.True(t, internal.IsEmptyResource(res))
			assert.Empty(t, schemaURL)
		})
	}
}

// Outside Kubernetes the SDK detector reports an empty resource, which must not
// be treated as missing metadata even with fail_on_missing_metadata.
func TestDetect_NotOnK8s(t *testing.T) {
	t.Setenv(kubernetesServiceHostEnvVar, "")

	for _, failOnMissingMetadata := range []bool{false, true} {
		res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), failOnMissingMetadata, sdkresource.Empty(), nil).Detect(t.Context())
		require.NoError(t, err)
		assert.True(t, internal.IsEmptyResource(res))
		assert.Empty(t, schemaURL)
	}
}

// A metadata response without a resource group is kept as a partial result, also
// with fail_on_missing_metadata, which covers an unusable metadata service and
// not a field absent from an otherwise good response.
func TestDetect_PartialMetadata(t *testing.T) {
	t.Setenv(kubernetesServiceHostEnvVar, "localhost")
	partial := sdkresource.NewWithAttributes(schemaURLUnderTest,
		attribute.String("cloud.provider", "azure"),
		attribute.String("cloud.platform", "azure.aks"),
	)
	partialErr := fmt.Errorf("%w: cluster name: resourceGroupName not present in metadata", sdkresource.ErrPartialResource)

	for _, failOnMissingMetadata := range []bool{false, true} {
		res, schemaURL, err := newTestDetector(t, clusterNameEnabled(), failOnMissingMetadata, partial, partialErr).Detect(t.Context())
		require.NoError(t, err)
		assert.Equal(t, schemaURLUnderTest, schemaURL)
		assert.Equal(t, map[string]any{
			"cloud.provider": "azure",
			"cloud.platform": "azure.aks",
		}, res.Attributes().AsRaw())
	}
}
