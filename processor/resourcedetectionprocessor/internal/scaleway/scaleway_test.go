// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scaleway

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

const testSchemaURL = "https://opentelemetry.io/schemas/1.43.0"

// fakeDetector stands in for the upstream SDK detector. Its metadata endpoints are
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

// fullResource mirrors what the SDK detector reports for a healthy instance.
func fullResource() *sdkresource.Resource {
	return sdkresource.NewWithAttributes(testSchemaURL,
		attribute.String("cloud.provider", "scaleway_cloud"),
		attribute.String("cloud.platform", "scaleway_cloud_compute"),
		attribute.String("cloud.account.id", "10542306-0c75-4265-9c2c-1fbfe4ea0bf0"),
		attribute.String("cloud.availability_zone", "nl-ams-1"),
		attribute.String("cloud.region", "nl-ams"),
		attribute.String("host.id", "daa2ea5a-0ee6-4cdc-9f1a-e0d1cb4e6d86"),
		attribute.String("host.image.id", "01fe25a9-2e95-41e2-9f3f-f7d3a00604be"),
		attribute.String("host.image.name", "Ubuntu 24.04 Noble Numbat"),
		attribute.String("host.name", "scw-interesting-bassi"),
		attribute.String("host.type", "STARDUST1-S"),
	)
}

func fullAttributes() map[string]any {
	return map[string]any{
		"cloud.provider":          "scaleway_cloud",
		"cloud.platform":          "scaleway_cloud_compute",
		"cloud.account.id":        "10542306-0c75-4265-9c2c-1fbfe4ea0bf0",
		"cloud.availability_zone": "nl-ams-1",
		"cloud.region":            "nl-ams",
		"host.id":                 "daa2ea5a-0ee6-4cdc-9f1a-e0d1cb4e6d86",
		"host.image.id":           "01fe25a9-2e95-41e2-9f3f-f7d3a00604be",
		"host.image.name":         "Ubuntu 24.04 Noble Numbat",
		"host.name":               "scw-interesting-bassi",
		"host.type":               "STARDUST1-S",
	}
}

func TestNewDetector(t *testing.T) {
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)
	require.NotNil(t, d)
}

func TestScalewayDetector_Detect_OK(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.Equal(t, testSchemaURL, schemaURL)
	require.Equal(t, fullAttributes(), res.Attributes().AsRaw())
}

// The SDK detector always reports every attribute, so the detector must drop the
// ones disabled in the configuration.
func TestScalewayDetector_Detect_DisabledAttribute(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.HostImageName.Enabled = false
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)

	want := fullAttributes()
	delete(want, "host.image.name")
	require.Equal(t, want, res.Attributes().AsRaw())
}

func TestScalewayDetector_NoMetadata(t *testing.T) {
	errNoMetadata := errors.New("no metadata")

	tests := []struct {
		name                  string
		res                   *sdkresource.Resource
		detectErr             error
		failOnMissingMetadata bool
		wantErr               string
	}{
		{
			name: "not on Scaleway",
			res:  sdkresource.Empty(),
		},
		{
			name:                  "not on Scaleway, fail on missing metadata",
			res:                   sdkresource.Empty(),
			failOnMissingMetadata: true,
			wantErr:               "scaleway metadata unavailable",
		},
		{
			name:      "metadata service error",
			detectErr: errNoMetadata,
		},
		{
			name:                  "metadata service error, fail on missing metadata",
			detectErr:             errNoMetadata,
			failOnMissingMetadata: true,
			wantErr:               errNoMetadata.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withFakeDetector(t, tt.res, tt.detectErr)

			d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), tt.failOnMissingMetadata)
			require.NoError(t, err)

			res, schemaURL, err := d.Detect(t.Context())
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.True(t, internal.IsEmptyResource(res))
			require.Empty(t, schemaURL)
		})
	}
}

// A partial result is kept with and without fail_on_missing_metadata: that flag
// covers an unusable metadata service, not a field absent from a good response.
func TestScalewayDetector_PartialMetadata(t *testing.T) {
	for _, failOnMissingMetadata := range []bool{false, true} {
		t.Run(fmt.Sprintf("failOnMissingMetadata=%t", failOnMissingMetadata), func(t *testing.T) {
			partial := sdkresource.NewWithAttributes(testSchemaURL,
				attribute.String("cloud.provider", "scaleway_cloud"),
				attribute.String("host.name", "scw-interesting-bassi"),
			)
			withFakeDetector(t, partial, fmt.Errorf("%w: id: not present in metadata", sdkresource.ErrPartialResource))

			d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), failOnMissingMetadata)
			require.NoError(t, err)

			res, schemaURL, err := d.Detect(t.Context())
			require.NoError(t, err)
			require.Equal(t, testSchemaURL, schemaURL)
			require.Equal(t, map[string]any{
				"cloud.provider": "scaleway_cloud",
				"host.name":      "scw-interesting-bassi",
			}, res.Attributes().AsRaw())
		})
	}
}
