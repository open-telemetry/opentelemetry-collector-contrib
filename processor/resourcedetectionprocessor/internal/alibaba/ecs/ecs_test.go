// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecs

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

const schemaURLForTest = "https://opentelemetry.io/schemas/1.43.0"

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
		"cloud.provider":          "alibaba_cloud",
		"cloud.platform":          "alibaba_cloud_ecs",
		"cloud.account.id":        "1234567890123456",
		"cloud.region":            "cn-hangzhou",
		"cloud.availability_zone": "cn-hangzhou-a",
		"host.id":                 "i-abcdef123456",
		"host.name":               "ecs-instance-01",
		"host.image.id":           "m-abcdef123456",
		"host.type":               "ecs.g6.large",
	}
}

func fullResource() *sdkresource.Resource {
	attrs := make([]attribute.KeyValue, 0, len(fullAttributes()))
	for k, v := range fullAttributes() {
		attrs = append(attrs, attribute.String(k, v.(string)))
	}
	return sdkresource.NewWithAttributes(schemaURLForTest, attrs...)
}

func TestNewDetector(t *testing.T) {
	for name, cfg := range map[string]Config{
		"empty config":   {},
		"default config": CreateDefaultConfig(),
	} {
		t.Run(name, func(t *testing.T) {
			d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
			require.NoError(t, err)
			require.NotNil(t, d)
		})
	}
}

func TestDetect_Success(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.Equal(t, schemaURLForTest, schemaURL)
	require.Equal(t, fullAttributes(), res.Attributes().AsRaw())
}

func TestDetect_DisabledAttributeRemoved(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.HostName.Enabled = false
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)

	want := fullAttributes()
	delete(want, "host.name")
	require.Equal(t, want, res.Attributes().AsRaw())
}

func TestDetect_EmptyResult(t *testing.T) {
	withFakeDetector(t, sdkresource.Empty(), nil)

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, schemaURL)
}

func TestDetect_EmptyResult_FailOnMissingMetadata(t *testing.T) {
	withFakeDetector(t, sdkresource.Empty(), nil)

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), true)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.EqualError(t, err, "alibaba_ecs metadata unavailable")
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, schemaURL)
}

// The deprecated per-detector flag alone must still trigger fail-on-missing.
func TestDetect_DeprecatedFailOnMissingMetadata(t *testing.T) {
	withFakeDetector(t, sdkresource.Empty(), nil)

	cfg := CreateDefaultConfig()
	cfg.FailOnMissingMetadata = true
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)

	_, _, err = d.Detect(t.Context())
	require.Error(t, err)
}

func TestDetect_Error(t *testing.T) {
	withFakeDetector(t, nil, errors.New("no metadata"))

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, schemaURL)
}

func TestDetect_Error_FailOnMissingMetadata(t *testing.T) {
	errNoMetadata := errors.New("no metadata")
	withFakeDetector(t, nil, errNoMetadata)

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), true)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.ErrorIs(t, err, errNoMetadata)
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, schemaURL)
}

// fail_on_missing_metadata covers an unusable metadata service, not a field absent
// from an otherwise good response, so a partial result succeeds either way.
func TestDetect_PartialResultKept(t *testing.T) {
	for _, failOnMissing := range []bool{false, true} {
		t.Run(fmt.Sprintf("fail_on_missing_metadata=%t", failOnMissing), func(t *testing.T) {
			partial := sdkresource.NewWithAttributes(schemaURLForTest,
				attribute.String("cloud.provider", "alibaba_cloud"),
				attribute.String("host.id", "i-abcdef123456"),
			)
			withFakeDetector(t, partial, fmt.Errorf("%w: hostname: not present in metadata", sdkresource.ErrPartialResource))

			d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), failOnMissing)
			require.NoError(t, err)

			res, schemaURL, err := d.Detect(t.Context())
			require.NoError(t, err)
			require.Equal(t, schemaURLForTest, schemaURL)
			require.Equal(t, map[string]any{
				"cloud.provider": "alibaba_cloud",
				"host.id":        "i-abcdef123456",
			}, res.Attributes().AsRaw())
		})
	}
}
