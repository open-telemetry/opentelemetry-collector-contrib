// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dynatrace

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

const testSchemaURL = "https://opentelemetry.io/schemas/1.0.0"

// fakeDetector stands in for the upstream SDK detector, whose enrichment
// directory is not injectable.
type fakeDetector struct {
	res *sdkresource.Resource
	err error
}

func (f fakeDetector) Detect(context.Context) (*sdkresource.Resource, error) {
	return f.res, f.err
}

func newTestDetector(t *testing.T, res *sdkresource.Resource, err error, failOnMissingMetadata bool) internal.Detector {
	t.Helper()

	orig := newResourceDetector
	newResourceDetector = func() sdkresource.Detector {
		return fakeDetector{res: res, err: err}
	}
	t.Cleanup(func() { newResourceDetector = orig })

	d, newErr := NewDetector(processortest.NewNopSettings(processortest.NopType), nil, failOnMissingMetadata)
	require.NoError(t, newErr)
	return d
}

func TestNewDetector(t *testing.T) {
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), nil, false)
	require.NoError(t, err)
	require.NotNil(t, d)
}

func TestDetect(t *testing.T) {
	d := newTestDetector(t, sdkresource.NewWithAttributes(testSchemaURL,
		attribute.String("dt.entity.host", "HOST-1"),
		attribute.String("host.name", "my-host"),
		attribute.String("dt.smartscape.host", "HOST_SMARTSCAPE-1"),
	), nil, false)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.Equal(t, testSchemaURL, schemaURL)
	require.Equal(t, map[string]any{
		"dt.entity.host":     "HOST-1",
		"host.name":          "my-host",
		"dt.smartscape.host": "HOST_SMARTSCAPE-1",
	}, res.Attributes().AsRaw())
}

// The enrichment file holding only some attributes is a normal result.
func TestDetect_PartialResult(t *testing.T) {
	partial := sdkresource.NewWithAttributes(testSchemaURL, attribute.String("host.name", "my-host"))
	d := newTestDetector(t, partial, fmt.Errorf("%w: dt.entity.host", sdkresource.ErrPartialResource), false)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.Equal(t, testSchemaURL, schemaURL)
	require.Equal(t, map[string]any{"host.name": "my-host"}, res.Attributes().AsRaw())
}

// This detector ignores fail_on_missing_metadata: a missing enrichment file is
// never an error and an unreadable one always is.
func TestDetect_IgnoresFailOnMissingMetadata(t *testing.T) {
	for _, failOnMissingMetadata := range []bool{false, true} {
		t.Run(fmt.Sprintf("failOnMissingMetadata=%t", failOnMissingMetadata), func(t *testing.T) {
			t.Run("file missing", func(t *testing.T) {
				d := newTestDetector(t, sdkresource.Empty(), nil, failOnMissingMetadata)

				res, schemaURL, err := d.Detect(t.Context())
				require.NoError(t, err)
				require.Empty(t, schemaURL)
				require.True(t, internal.IsEmptyResource(res))
			})

			t.Run("file unreadable", func(t *testing.T) {
				errRead := errors.New("permission denied")
				d := newTestDetector(t, nil, errRead, failOnMissingMetadata)

				res, schemaURL, err := d.Detect(t.Context())
				require.ErrorIs(t, err, errRead)
				require.Empty(t, schemaURL)
				require.True(t, internal.IsEmptyResource(res))
			})
		})
	}
}
