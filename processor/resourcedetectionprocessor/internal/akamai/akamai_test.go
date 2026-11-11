// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package akamai

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

const schemaURL = "https://opentelemetry.io/schemas/1.43.0"

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

func fullAttributes() map[string]any {
	return map[string]any{
		"cloud.account.id": "acc-eeee-uuuu-iiiii-dddd",
		"cloud.platform":   "akamai_cloud.compute",
		"cloud.provider":   "akamai_cloud",
		"cloud.region":     "us-southeast",
		"host.id":          "4242",
		"host.image.id":    "linode/ubuntu24.04",
		"host.image.name":  "Ubuntu 24.04 LTS",
		"host.name":        "linode-4242",
		"host.type":        "g6-standard-4",
	}
}

// fullResource mirrors what the SDK detector reports for a healthy instance.
func fullResource() *sdkresource.Resource {
	attrs := make([]attribute.KeyValue, 0, 9)
	for k, v := range fullAttributes() {
		attrs = append(attrs, attribute.String(k, v.(string)))
	}
	return sdkresource.NewWithAttributes(schemaURL, attrs...)
}

// partialResource mirrors an instance document without a region or account.
func partialResource() *sdkresource.Resource {
	return sdkresource.NewWithAttributes(schemaURL,
		attribute.String("cloud.provider", "akamai_cloud"),
		attribute.String("cloud.platform", "akamai_cloud.compute"),
		attribute.String("host.name", "linode-4242"),
	)
}

func TestNewDetector(t *testing.T) {
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)
	require.NotNil(t, d)
}

func TestAkamaiDetector_Detect_OK(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, gotSchemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.Equal(t, schemaURL, gotSchemaURL)
	require.Equal(t, fullAttributes(), res.Attributes().AsRaw())
}

func TestAkamaiDetector_Detect_DisabledAttribute(t *testing.T) {
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

// The SDK detector reports an empty resource when the process is not on Akamai.
func TestAkamaiDetector_NotOnAkamai(t *testing.T) {
	withFakeDetector(t, sdkresource.Empty(), nil)

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, gotSchemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, gotSchemaURL)
}

func TestAkamaiDetector_NotOnAkamai_FailOnMissingMetadata(t *testing.T) {
	withFakeDetector(t, sdkresource.Empty(), nil)

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), true)
	require.NoError(t, err)

	res, gotSchemaURL, err := d.Detect(t.Context())
	require.EqualError(t, err, "akamai metadata unavailable")
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, gotSchemaURL)
}

// The metadata service answered but the response was unusable. Without
// fail_on_missing_metadata this stays a debug log, not an error.
func TestAkamaiDetector_MetadataError(t *testing.T) {
	withFakeDetector(t, nil, errors.New("no metadata"))

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, gotSchemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, gotSchemaURL)
}

func TestAkamaiDetector_MetadataError_FailOnMissingMetadata(t *testing.T) {
	errNoMetadata := errors.New("no metadata")
	withFakeDetector(t, nil, errNoMetadata)

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), true)
	require.NoError(t, err)

	res, gotSchemaURL, err := d.Detect(t.Context())
	require.ErrorIs(t, err, errNoMetadata)
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, gotSchemaURL)
}

// Attributes absent from the metadata response are omitted rather than emitted
// with an empty value. fail_on_missing_metadata covers an unusable metadata
// service, not a missing field, so a partial result succeeds either way.
func TestAkamaiDetector_PartialMetadata(t *testing.T) {
	for _, failOnMissingMetadata := range []bool{false, true} {
		t.Run(fmt.Sprintf("failOnMissingMetadata=%t", failOnMissingMetadata), func(t *testing.T) {
			withFakeDetector(t, partialResource(), fmt.Errorf("%w: region: not present in metadata", sdkresource.ErrPartialResource))

			d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), failOnMissingMetadata)
			require.NoError(t, err)

			res, gotSchemaURL, err := d.Detect(t.Context())
			require.NoError(t, err)
			require.Equal(t, schemaURL, gotSchemaURL)
			require.Equal(t, map[string]any{
				"cloud.provider": "akamai_cloud",
				"cloud.platform": "akamai_cloud.compute",
				"host.name":      "linode-4242",
			}, res.Attributes().AsRaw())
		})
	}
}
