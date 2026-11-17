// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package classic

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
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/ibmcloud/classic/internal/metadata"
)

const testSchemaURL = "https://opentelemetry.io/schemas/1.43.0"

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

func newTestDetector(t *testing.T, cfg Config, failOnMissingMetadata bool) internal.Detector {
	t.Helper()

	d, err := NewDetector(processortest.NewNopSettings(metadata.Type), cfg, failOnMissingMetadata)
	require.NoError(t, err)
	return d
}

// fullResource mirrors what the SDK detector reports for a healthy instance.
func fullResource() *sdkresource.Resource {
	return sdkresource.NewWithAttributes(testSchemaURL,
		attribute.String("cloud.provider", "ibm_cloud"),
		attribute.String("cloud.platform", "ibm_cloud.classic"),
		attribute.String("cloud.account.id", "3186058"),
		attribute.String("cloud.availability_zone", "par01"),
		attribute.String("cloud.resource_id", "06220b70-9072-4f83-ba16-d62f03106c1c"),
		attribute.String("host.id", "156800198"),
		attribute.String("host.name", "otel-collector"),
	)
}

// partialResource mirrors what the SDK detector reports when some fields failed.
func partialResource() *sdkresource.Resource {
	return sdkresource.NewWithAttributes(testSchemaURL,
		attribute.String("cloud.provider", "ibm_cloud"),
		attribute.String("cloud.platform", "ibm_cloud.classic"),
		attribute.String("host.id", "156800198"),
	)
}

func TestNewDetector(t *testing.T) {
	require.NotNil(t, newTestDetector(t, CreateDefaultConfig(), false))
}

func TestDetect(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), false).Detect(t.Context())
	require.NoError(t, err)
	require.Equal(t, testSchemaURL, schemaURL)

	want := map[string]any{
		"cloud.provider":          "ibm_cloud",
		"cloud.platform":          "ibm_cloud.classic",
		"cloud.account.id":        "3186058",
		"cloud.availability_zone": "par01",
		"cloud.resource_id":       "06220b70-9072-4f83-ba16-d62f03106c1c",
		"host.id":                 "156800198",
		"host.name":               "otel-collector",
	}
	require.Equal(t, want, res.Attributes().AsRaw())
}

func TestDetectWithDisabledAttributes(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.CloudAvailabilityZone.Enabled = false
	cfg.ResourceAttributes.HostName.Enabled = false

	res, schemaURL, err := newTestDetector(t, cfg, false).Detect(t.Context())
	require.NoError(t, err)
	require.Equal(t, testSchemaURL, schemaURL)

	want := map[string]any{
		"cloud.provider":    "ibm_cloud",
		"cloud.platform":    "ibm_cloud.classic",
		"cloud.account.id":  "3186058",
		"cloud.resource_id": "06220b70-9072-4f83-ba16-d62f03106c1c",
		"host.id":           "156800198",
	}
	require.Equal(t, want, res.Attributes().AsRaw())
}

func TestDetectNotOnIBMCloudClassic(t *testing.T) {
	withFakeDetector(t, sdkresource.Empty(), nil)

	res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), false).Detect(t.Context())
	require.NoError(t, err)
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, schemaURL)
}

func TestDetectNotOnIBMCloudClassicWithFailOnMissingMetadata(t *testing.T) {
	withFakeDetector(t, sdkresource.Empty(), nil)

	res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), true).Detect(t.Context())
	require.EqualError(t, err, "ibmcloud classic metadata unavailable")
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, schemaURL)
}

func TestDetectError(t *testing.T) {
	withFakeDetector(t, nil, errors.New("connection refused"))

	res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), false).Detect(t.Context())
	require.NoError(t, err)
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, schemaURL)
}

func TestDetectErrorWithFailOnMissingMetadata(t *testing.T) {
	errNoMetadata := errors.New("connection refused")
	withFakeDetector(t, nil, errNoMetadata)

	res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), true).Detect(t.Context())
	require.ErrorIs(t, err, errNoMetadata)
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, schemaURL)
}

// A partial result is kept with or without fail_on_missing_metadata: the flag covers
// an unusable metadata service, not a field absent from an otherwise good response.
func TestDetectPartialMetadata(t *testing.T) {
	for _, failOnMissingMetadata := range []bool{false, true} {
		t.Run(fmt.Sprintf("failOnMissingMetadata=%t", failOnMissingMetadata), func(t *testing.T) {
			withFakeDetector(t, partialResource(), fmt.Errorf("%w: hostname: not present in metadata", sdkresource.ErrPartialResource))

			res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), failOnMissingMetadata).Detect(t.Context())
			require.NoError(t, err)
			require.Equal(t, testSchemaURL, schemaURL)

			want := map[string]any{
				"cloud.provider": "ibm_cloud",
				"cloud.platform": "ibm_cloud.classic",
				"host.id":        "156800198",
			}
			require.Equal(t, want, res.Attributes().AsRaw())
		})
	}
}
