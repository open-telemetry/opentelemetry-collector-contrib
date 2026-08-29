// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hetzner

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

// fakeDetector stands in for the upstream SDK detector. The SDK's hcloud
// metadata client is not injectable, so the adapter is exercised against canned
// SDK results instead of a fake metadata server.
type fakeDetector struct {
	res *sdkresource.Resource
	err error
}

func (f fakeDetector) Detect(context.Context) (*sdkresource.Resource, error) {
	return f.res, f.err
}

func withFakeDetector(t *testing.T, res *sdkresource.Resource, err error) {
	t.Helper()

	orig := newResourceDetector
	newResourceDetector = func() sdkresource.Detector {
		return fakeDetector{res: res, err: err}
	}
	t.Cleanup(func() { newResourceDetector = orig })
}

func fullResource() *sdkresource.Resource {
	return sdkresource.NewSchemaless(
		attribute.String("cloud.provider", "hetzner"),
		attribute.String("cloud.platform", "hetzner.cloud_server"),
		attribute.String("host.id", "987654321"),
		attribute.String("host.name", "srv-123"),
		attribute.String("cloud.region", "nbg1"),
		attribute.String("cloud.availability_zone", "nbg1-dc3"),
	)
}

func TestNewDetector(t *testing.T) {
	dcfg := CreateDefaultConfig()
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), dcfg, false)
	require.NoError(t, err)
	require.NotNil(t, d)
}

func TestHetznerDetector_Detect_OK(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.CloudPlatform.Enabled = true
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)

	want := map[string]any{
		"cloud.provider":          TypeStr,
		"cloud.platform":          TypeStr + ".cloud_server",
		"host.id":                 "987654321",
		"host.name":               "srv-123",
		"cloud.region":            "nbg1",
		"cloud.availability_zone": "nbg1-dc3",
	}
	require.Equal(t, want, res.Attributes().AsRaw())
}

// cloud.platform is disabled by default, so the detector must drop it even
// though the SDK detector always reports it.
func TestHetznerDetector_Detect_DefaultConfig(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)

	want := map[string]any{
		"cloud.provider":          TypeStr,
		"host.id":                 "987654321",
		"host.name":               "srv-123",
		"cloud.region":            "nbg1",
		"cloud.availability_zone": "nbg1-dc3",
	}
	require.Equal(t, want, res.Attributes().AsRaw())
}

func TestHetznerDetector_NotOnHetzner(t *testing.T) {
	withFakeDetector(t, sdkresource.Empty(), nil)

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, schemaURL)
}

// A partial result is kept as-is when fail_on_missing_metadata is false: the
// attributes that could not be read are omitted rather than emitted empty.
func TestHetznerDetector_PartialMetadata(t *testing.T) {
	partial := sdkresource.NewSchemaless(
		attribute.String("cloud.provider", "hetzner"),
		attribute.String("cloud.platform", "hetzner.cloud_server"),
		attribute.String("host.name", "srv-123"),
		attribute.String("cloud.region", "nbg1"),
	)
	withFakeDetector(t, partial, fmt.Errorf("%w: instance ID: boom", sdkresource.ErrPartialResource))

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)

	want := map[string]any{
		"cloud.provider": TypeStr,
		"host.name":      "srv-123",
		"cloud.region":   "nbg1",
	}
	require.Equal(t, want, res.Attributes().AsRaw())
}

// fail_on_missing_metadata covers an unusable metadata service, not a field
// absent from an otherwise good response, so a partial result still succeeds.
func TestHetznerDetector_PartialMetadata_FailOnMissingMetadata(t *testing.T) {
	partial := sdkresource.NewSchemaless(
		attribute.String("cloud.provider", "hetzner"),
		attribute.String("cloud.platform", "hetzner.cloud_server"),
		attribute.String("host.name", "srv-123"),
	)
	withFakeDetector(t, partial, fmt.Errorf("%w: instance ID: boom", sdkresource.ErrPartialResource))

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), true)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)

	want := map[string]any{
		"cloud.provider": TypeStr,
		"host.name":      "srv-123",
	}
	require.Equal(t, want, res.Attributes().AsRaw())
}

func TestHetznerDetector_Error(t *testing.T) {
	errBoom := errors.New("boom")
	withFakeDetector(t, nil, errBoom)

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.ErrorIs(t, err, errBoom)
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, schemaURL)
}
