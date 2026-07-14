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

func TestDetect_AllEnabled(t *testing.T) {
	sdkRes := sdkresource.NewWithAttributes("https://opentelemetry.io/schemas/1.0.0",
		attribute.String("foo", "a"),
		attribute.String("bar", "b"),
		attribute.String("baz", "c"),
	)
	res, schemaURL, err := Detect(t.Context(), &mockSDKDetector{res: sdkRes}, map[string]bool{
		"foo": true,
		"bar": true,
		"baz": true,
	})
	require.NoError(t, err)
	assert.Equal(t, "https://opentelemetry.io/schemas/1.0.0", schemaURL)
	assert.Equal(t, map[string]any{
		"foo": "a",
		"bar": "b",
		"baz": "c",
	}, res.Attributes().AsRaw())
}

func TestDetect_AttributeDisabled(t *testing.T) {
	sdkRes := sdkresource.NewWithAttributes("https://opentelemetry.io/schemas/1.0.0",
		attribute.String("foo", "a"),
		attribute.String("bar", "b"),
		attribute.String("baz", "c"),
	)
	res, _, err := Detect(t.Context(), &mockSDKDetector{res: sdkRes}, map[string]bool{
		"foo": true,
		"bar": false,
		"baz": true,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, res.Attributes().Len())
	_, hasBar := res.Attributes().Get("bar")
	assert.False(t, hasBar)
}

func TestDetect_ErrPartialResourceSuppressed(t *testing.T) {
	sdkRes := sdkresource.NewWithAttributes("https://opentelemetry.io/schemas/1.0.0",
		attribute.String("foo", "a"),
	)
	res, _, err := Detect(t.Context(), &mockSDKDetector{res: sdkRes, err: sdkresource.ErrPartialResource}, map[string]bool{
		"foo": true,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Attributes().Len())
}

func TestDetect_NonPartialErrorPropagated(t *testing.T) {
	res, schemaURL, err := Detect(t.Context(), &mockSDKDetector{res: sdkresource.Empty(), err: errors.New("unexpected error")}, map[string]bool{})
	require.ErrorContains(t, err, "unexpected error")
	assert.Empty(t, schemaURL)
	assert.Equal(t, 0, res.Attributes().Len())
}

func TestDetect_NotOnPlatform(t *testing.T) {
	res, schemaURL, err := Detect(t.Context(), &mockSDKDetector{res: sdkresource.Empty()}, map[string]bool{
		"cloud.provider": true,
	})
	require.NoError(t, err)
	assert.Empty(t, schemaURL)
	assert.Equal(t, 0, res.Attributes().Len())
}
