// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdkbridge

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
)

type mockSDKDetector struct {
	res *sdkresource.Resource
	err error
}

func (m *mockSDKDetector) Detect(_ context.Context) (*sdkresource.Resource, error) {
	return m.res, m.err
}

func TestDetect_CopiesAttributes(t *testing.T) {
	sdkRes := sdkresource.NewWithAttributes("https://opentelemetry.io/schemas/1.0.0",
		attribute.String("foo", "a"),
		attribute.String("bar", "b"),
	)
	res, schemaURL, err := Detect(t.Context(), &mockSDKDetector{res: sdkRes})
	require.NoError(t, err)
	assert.Equal(t, "https://opentelemetry.io/schemas/1.0.0", schemaURL)
	assert.Equal(t, map[string]any{
		"foo": "a",
		"bar": "b",
	}, res.Attributes().AsRaw())
}

func TestDetect_PreservesAttributeTypes(t *testing.T) {
	sdkRes := sdkresource.NewWithAttributes("https://opentelemetry.io/schemas/1.0.0",
		attribute.Bool("b", true),
		attribute.Int64("i", 42),
		attribute.Float64("f", 1.5),
		attribute.String("s", "str"),
		attribute.BoolSlice("bs", []bool{true, false}),
		attribute.Int64Slice("is", []int64{1, 2}),
		attribute.Float64Slice("fs", []float64{1.5, 2.5}),
		attribute.StringSlice("ss", []string{"x", "y"}),
	)

	res, _, err := Detect(t.Context(), &mockSDKDetector{res: sdkRes})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"b":  true,
		"i":  int64(42),
		"f":  1.5,
		"s":  "str",
		"bs": []any{true, false},
		"is": []any{int64(1), int64(2)},
		"fs": []any{1.5, 2.5},
		"ss": []any{"x", "y"},
	}, res.Attributes().AsRaw())
}

// Types without a direct pdata equivalent fall back to their string form.
func TestDetect_UnsupportedTypeStringified(t *testing.T) {
	sdkRes := sdkresource.NewWithAttributes("https://opentelemetry.io/schemas/1.0.0",
		attribute.KeyValue{Key: "bytes", Value: attribute.ByteSliceValue([]byte("raw"))},
	)

	res, _, err := Detect(t.Context(), &mockSDKDetector{res: sdkRes})
	require.NoError(t, err)
	got, ok := res.Attributes().Get("bytes")
	require.True(t, ok)
	assert.NotEmpty(t, got.Str())
}

func TestDetect_ErrPartialResourceSuppressed(t *testing.T) {
	sdkRes := sdkresource.NewWithAttributes("https://opentelemetry.io/schemas/1.0.0",
		attribute.String("foo", "a"),
	)
	res, schemaURL, err := Detect(t.Context(), &mockSDKDetector{res: sdkRes, err: sdkresource.ErrPartialResource})
	require.NoError(t, err)
	assert.Equal(t, "https://opentelemetry.io/schemas/1.0.0", schemaURL)
	assert.Equal(t, 1, res.Attributes().Len())
}

func TestDetect_NonPartialErrorPropagated(t *testing.T) {
	res, schemaURL, err := Detect(t.Context(), &mockSDKDetector{res: sdkresource.Empty(), err: errors.New("unexpected error")})
	require.ErrorContains(t, err, "unexpected error")
	assert.Empty(t, schemaURL)
	assert.Equal(t, 0, res.Attributes().Len())
}

func TestDetect_NotOnPlatform(t *testing.T) {
	res, schemaURL, err := Detect(t.Context(), &mockSDKDetector{res: sdkresource.Empty()})
	require.NoError(t, err)
	assert.Empty(t, schemaURL)
	assert.Equal(t, 0, res.Attributes().Len())
}

// A detector returning a nil resource with no error must not panic.
func TestDetect_NilResource(t *testing.T) {
	res, schemaURL, err := Detect(t.Context(), &mockSDKDetector{res: nil})
	require.NoError(t, err)
	assert.Empty(t, schemaURL)
	assert.Equal(t, 0, res.Attributes().Len())
}
