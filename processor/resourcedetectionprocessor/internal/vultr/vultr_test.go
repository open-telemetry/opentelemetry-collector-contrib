// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vultr

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

const (
	hostName = "vultr-guest"
	v2ID     = "36e9cf60-5d93-4e31-8ebf-613b3d2874fb"
	region   = "ewr"
)

// ---- test stub + hook ----

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

func withFakeDetector(t *testing.T, res *sdkresource.Resource, err error) {
	t.Helper()

	orig := newResourceDetector
	newResourceDetector = func() sdkresource.Detector {
		return fakeDetector{res: res, err: err}
	}
	t.Cleanup(func() { newResourceDetector = orig })
}

// fullResource mirrors what the SDK detector reports for a healthy instance. It
// already prefers the v2 UUID for host.id and lower-cases the region code.
func fullResource() *sdkresource.Resource {
	return sdkresource.NewSchemaless(
		attribute.String("cloud.provider", "vultr"),
		attribute.String("cloud.platform", "vultr.cloud_compute"),
		attribute.String("cloud.region", region),
		attribute.String("host.id", v2ID),
		attribute.String("host.name", hostName),
	)
}

// ---- tests ----
func TestNewDetector(t *testing.T) {
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)
	require.NotNil(t, d)
}

func TestVultrDetector_Detect_OK(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.CloudPlatform.Enabled = true
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)

	want := map[string]any{
		"cloud.provider": TypeStr,
		"cloud.platform": TypeStr + ".cloud_compute",
		"cloud.region":   region,
		"host.id":        v2ID,
		"host.name":      hostName,
	}
	require.Equal(t, want, res.Attributes().AsRaw())
}

// cloud.platform is disabled by default, so the detector must drop it even
// though the SDK detector always reports it.
func TestVultrDetector_Detect_DefaultConfig(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)

	want := map[string]any{
		"cloud.provider": TypeStr,
		"cloud.region":   region,
		"host.id":        v2ID,
		"host.name":      hostName,
	}
	require.Equal(t, want, res.Attributes().AsRaw())
}

// All attributes disabled is not the same as not being on the platform: the
// instance was still detected, so the schema URL survives an empty attribute set.
func TestVultrDetector_Detect_AllAttributesDisabled_FailOnMissingMetadata(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.CloudProvider.Enabled = false
	cfg.ResourceAttributes.CloudPlatform.Enabled = false
	cfg.ResourceAttributes.CloudRegion.Enabled = false
	cfg.ResourceAttributes.HostID.Enabled = false
	cfg.ResourceAttributes.HostName.Enabled = false
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, true)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.Equal(t, 0, res.Attributes().Len())
}

func TestVultrDetector_NotOnVultr(t *testing.T) {
	withFakeDetector(t, sdkresource.Empty(), nil)

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, schemaURL)
}

// The metadata service answered but the response was unusable. Without
// fail_on_missing_metadata this stays a debug log, not an error.
func TestVultrDetector_MetadataError(t *testing.T) {
	withFakeDetector(t, nil, errors.New("no metadata"))

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, schemaURL)
}

func TestVultrDetector_MetadataError_FailOnMissingMetadata(t *testing.T) {
	errNoMetadata := errors.New("no metadata")
	withFakeDetector(t, nil, errNoMetadata)

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), true)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.ErrorIs(t, err, errNoMetadata)
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, schemaURL)
}

func TestVultrDetector_FailOnMissingMetadata(t *testing.T) {
	withFakeDetector(t, sdkresource.Empty(), nil)

	cfg := CreateDefaultConfig()
	cfg.FailOnMissingMetadata = true

	// Inject top-level false: the deprecated per-detector flag alone must still
	// trigger fail-on-missing for this detector (backward compatibility).
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.Error(t, err)
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, schemaURL)
}

// A partial result is kept as-is when fail_on_missing_metadata is false: the
// attributes absent from the metadata response are omitted rather than emitted
// with an empty value.
func TestVultrDetector_PartialMetadata(t *testing.T) {
	partial := sdkresource.NewSchemaless(
		attribute.String("cloud.provider", "vultr"),
		attribute.String("cloud.platform", "vultr.cloud_compute"),
		attribute.String("host.name", hostName),
	)
	withFakeDetector(t, partial, fmt.Errorf("%w: region: not present in metadata", sdkresource.ErrPartialResource))

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)

	want := map[string]any{
		"cloud.provider": TypeStr,
		"host.name":      hostName,
	}
	require.Equal(t, want, res.Attributes().AsRaw())
}

// fail_on_missing_metadata covers an unusable metadata service, not a field
// absent from an otherwise good response, so a partial result still succeeds.
func TestVultrDetector_PartialMetadata_FailOnMissingMetadata(t *testing.T) {
	partial := sdkresource.NewSchemaless(
		attribute.String("cloud.provider", "vultr"),
		attribute.String("host.name", hostName),
	)
	withFakeDetector(t, partial, fmt.Errorf("%w: region: not present in metadata", sdkresource.ErrPartialResource))

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), true)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)

	want := map[string]any{
		"cloud.provider": TypeStr,
		"host.name":      hostName,
	}
	require.Equal(t, want, res.Attributes().AsRaw())
}
