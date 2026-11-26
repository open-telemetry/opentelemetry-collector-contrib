// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_upcloud) && !omit_detector_upcloud

package upcloud

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
)

const testSchemaURL = "https://opentelemetry.io/schemas/1.45.0"

// fakeDetector stands in for the upstream SDK detector, whose metadata endpoint is
// not injectable.
type fakeDetector struct {
	attrs []attribute.KeyValue
	err   error
}

func (f fakeDetector) Detect(context.Context) (*sdkresource.Resource, error) {
	return sdkresource.NewWithAttributes(testSchemaURL, f.attrs...), f.err
}

func withFakeDetector(t *testing.T, f fakeDetector) {
	t.Helper()

	orig := newResourceDetector
	newResourceDetector = func() sdkresource.Detector { return f }
	t.Cleanup(func() { newResourceDetector = orig })
}

func fullAttributes() []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("cloud.provider", "upcloud"),
		attribute.String("cloud.region", "fi-hel1"),
		attribute.String("host.id", "00123456-1111-2222-3333-123456789012"),
		attribute.String("host.name", "my-server"),
	}
}

func fullWant() map[string]any {
	return map[string]any{
		"cloud.provider": "upcloud",
		"cloud.region":   "fi-hel1",
		"host.id":        "00123456-1111-2222-3333-123456789012",
		"host.name":      "my-server",
	}
}

func TestNewDetector(t *testing.T) {
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)
	assert.NotNil(t, d)
}

func TestDetect_OK(t *testing.T) {
	withFakeDetector(t, fakeDetector{attrs: fullAttributes()})

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, testSchemaURL, schemaURL)
	assert.Equal(t, fullWant(), res.Attributes().AsRaw())
}

func TestDetect_DisabledAttributeRemoved(t *testing.T) {
	withFakeDetector(t, fakeDetector{attrs: fullAttributes()})

	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.HostName.Enabled = false
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)

	want := fullWant()
	delete(want, "host.name")
	assert.Equal(t, want, res.Attributes().AsRaw())
}

// A partial result still came from a reachable metadata service, so it is kept even
// with fail_on_missing_metadata.
func TestDetect_PartialResult(t *testing.T) {
	withFakeDetector(t, fakeDetector{
		attrs: fullAttributes()[:2],
		err:   fmt.Errorf("%w: [instance ID: not present in metadata]", sdkresource.ErrPartialResource),
	})

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), true)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"cloud.provider": "upcloud",
		"cloud.region":   "fi-hel1",
	}, res.Attributes().AsRaw())
}

func TestDetect_EmptyResult(t *testing.T) {
	tests := []struct {
		name                     string
		failOnMissingMetadata    bool
		cfgFailOnMissingMetadata bool
		wantErr                  bool
	}{
		{name: "default"},
		{name: "fail_on_missing_metadata", failOnMissingMetadata: true, wantErr: true},
		{name: "deprecated detector fail_on_missing_metadata", cfgFailOnMissingMetadata: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withFakeDetector(t, fakeDetector{})

			cfg := CreateDefaultConfig()
			cfg.FailOnMissingMetadata = tt.cfgFailOnMissingMetadata
			d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, tt.failOnMissingMetadata)
			require.NoError(t, err)

			res, schemaURL, err := d.Detect(t.Context())
			if tt.wantErr {
				require.EqualError(t, err, "upcloud metadata unavailable")
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, 0, res.Attributes().Len())
			assert.Empty(t, schemaURL)
		})
	}
}

func TestDetect_Error(t *testing.T) {
	errBoom := errors.New("metadata request returned status 500")

	tests := []struct {
		name                  string
		failOnMissingMetadata bool
	}{
		{name: "default"},
		{name: "fail_on_missing_metadata", failOnMissingMetadata: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withFakeDetector(t, fakeDetector{err: errBoom})

			d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), tt.failOnMissingMetadata)
			require.NoError(t, err)

			res, schemaURL, err := d.Detect(t.Context())
			if tt.failOnMissingMetadata {
				require.ErrorIs(t, err, errBoom)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, 0, res.Attributes().Len())
			assert.Empty(t, schemaURL)
		})
	}
}
