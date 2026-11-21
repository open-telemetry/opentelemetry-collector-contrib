// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_docker) && !omit_detector_docker

package docker

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/docker/internal/metadata"
)

const schemaURLWant = "https://opentelemetry.io/schemas/1.43.0"

// ---- test stub + hook ----

// fakeDetector stands in for the upstream SDK detector. Its Docker client is not
// injectable, so the adapter is exercised against canned SDK results instead of a
// fake daemon.
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

// setEmitSemconvContainerAttributes toggles the gate for one test. The registry is
// process-global, so these tests must not run in parallel.
func setEmitSemconvContainerAttributes(t *testing.T, enabled bool) {
	t.Helper()

	gate := metadata.ProcessorResourcedetectionDockerEmitSemconvContainerAttributesFeatureGate
	original := gate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), enabled))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), original))
	})
}

// fullResource mirrors what the SDK detector reports for a container it could fully
// inspect. It already trims the leading slash from the container name.
func fullResource() *sdkresource.Resource {
	return sdkresource.NewWithAttributes(schemaURLWant,
		attribute.String("container.name", "foo"),
		attribute.String("container.image.name", "bar"),
		attribute.StringSlice("container.image.tags", []string{"1.0"}),
		attribute.String("container.image.id", "sha256:abc"),
		attribute.String("host.name", "hostname"),
		attribute.String("os.type", "darwin"),
	)
}

// ---- tests ----

func TestNewDetector(t *testing.T) {
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)
	require.NotNil(t, d)
}

// With the gate off, the detector reports the shapes it always has: container.name with the
// leading slash, and the image ID under container.image.name.
func TestDetect_DefaultGate(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.ContainerImageID.Enabled = true
	cfg.ResourceAttributes.ContainerImageName.Enabled = true
	cfg.ResourceAttributes.ContainerImageTags.Enabled = true
	cfg.ResourceAttributes.ContainerName.Enabled = true

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.Equal(t, schemaURLWant, schemaURL)

	want := map[string]any{
		"container.name":       "/foo",
		"container.image.name": "sha256:abc",
		"container.image.tags": []any{"1.0"},
		"container.image.id":   "sha256:abc",
		"host.name":            "hostname",
		"os.type":              "darwin",
	}
	require.Equal(t, want, res.Attributes().AsRaw())
}

// The gate reports container.name without the slash and the real image name.
func TestDetect(t *testing.T) {
	setEmitSemconvContainerAttributes(t, true)
	withFakeDetector(t, fullResource(), nil)

	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.ContainerImageID.Enabled = true
	cfg.ResourceAttributes.ContainerImageName.Enabled = true
	cfg.ResourceAttributes.ContainerImageTags.Enabled = true
	cfg.ResourceAttributes.ContainerName.Enabled = true

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.Equal(t, schemaURLWant, schemaURL)

	want := map[string]any{
		"container.name":       "foo",
		"container.image.name": "bar",
		"container.image.tags": []any{"1.0"},
		"container.image.id":   "sha256:abc",
		"host.name":            "hostname",
		"os.type":              "darwin",
	}
	require.Equal(t, want, res.Attributes().AsRaw())
}

// With the gate off, container.image.name carries the image ID even though
// container.image.id itself is disabled, as it is by default.
func TestDetect_DefaultGateImageIDDisabled(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.ContainerImageName.Enabled = true

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)

	want := map[string]any{
		"container.image.name": "sha256:abc",
		"host.name":            "hostname",
		"os.type":              "darwin",
	}
	require.Equal(t, want, res.Attributes().AsRaw())
}

// The container attributes are disabled by default, so they are filtered out even
// though the SDK detector always reports them. Unaffected by the gate.
func TestDetectFiltersContainerInfoByDefault(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)

	want := map[string]any{
		"host.name": "hostname",
		"os.type":   "darwin",
	}
	require.Equal(t, want, res.Attributes().AsRaw())
}

// All attributes disabled is not the same as not running in a container: the
// container was still detected, so this must not be reported as missing metadata.
func TestDetectSkipsDisabledResourceAttributes(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.HostName.Enabled = false
	cfg.ResourceAttributes.OsType.Enabled = false

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, true)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.Equal(t, 0, res.Attributes().Len())
}

func TestDetectNotInContainer(t *testing.T) {
	withFakeDetector(t, sdkresource.Empty(), nil)

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.Equal(t, 0, res.Attributes().Len())
	require.Empty(t, schemaURL)
}

func TestDetectNotInContainer_FailOnMissingMetadata(t *testing.T) {
	withFakeDetector(t, sdkresource.Empty(), nil)

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), true)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.ErrorContains(t, err, "docker metadata unavailable")
	require.Equal(t, 0, res.Attributes().Len())
	require.Empty(t, schemaURL)
}

// A daemon that could not be reached is only a hard failure when the operator asked for one.
func TestDetectError(t *testing.T) {
	withFakeDetector(t, nil, errors.New("boom"))

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.Equal(t, 0, res.Attributes().Len())
	require.Empty(t, schemaURL)
}

func TestDetectError_FailOnMissingMetadata(t *testing.T) {
	errBoom := errors.New("boom")
	withFakeDetector(t, nil, errBoom)

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), true)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.ErrorIs(t, err, errBoom)
	require.Equal(t, 0, res.Attributes().Len())
	require.Empty(t, schemaURL)
}

// The SDK detector returns a partial result when the daemon's Info call fails, which leaves
// out host.name and os.type. That is only a failure when fail_on_missing_metadata is set and
// one of them is enabled.
func TestDetectPartialMetadata(t *testing.T) {
	tests := []struct {
		name                  string
		hostAttrsEnabled      bool
		failOnMissingMetadata bool
		wantErr               bool
		want                  map[string]any
	}{
		{
			name:                  "host attributes disabled",
			failOnMissingMetadata: true,
			// gate off, so the legacy leading slash is restored
			want: map[string]any{"container.name": "/foo"},
		},
		{
			name:                  "host attributes enabled",
			hostAttrsEnabled:      true,
			failOnMissingMetadata: true,
			wantErr:               true,
			want:                  map[string]any{},
		},
		{
			name:             "host attributes enabled without fail_on_missing_metadata",
			hostAttrsEnabled: true,
			want:             map[string]any{"container.name": "/foo"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			partial := sdkresource.NewSchemaless(
				attribute.String("container.name", "foo"),
			)
			withFakeDetector(t, partial, fmt.Errorf("%w: docker info: boom", sdkresource.ErrPartialResource))

			cfg := CreateDefaultConfig()
			cfg.ResourceAttributes.ContainerName.Enabled = true
			cfg.ResourceAttributes.HostName.Enabled = tt.hostAttrsEnabled
			cfg.ResourceAttributes.OsType.Enabled = tt.hostAttrsEnabled

			d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, tt.failOnMissingMetadata)
			require.NoError(t, err)

			res, _, err := d.Detect(t.Context())
			if tt.wantErr {
				require.ErrorContains(t, err, "docker metadata incomplete")
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.want, res.Attributes().AsRaw())
		})
	}
}
