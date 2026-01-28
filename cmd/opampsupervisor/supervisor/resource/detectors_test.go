// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
)

func findAttr(attrs []attribute.KeyValue, key attribute.Key) (attribute.Value, bool) {
	for _, kv := range attrs {
		if kv.Key == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

func TestGetDetector(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	tests := []struct {
		name         string
		detectorName string
		wantErr      bool
	}{
		{name: "env", detectorName: "env"},
		{name: "host", detectorName: "host"},
		{name: "aws", detectorName: "aws"},
		{name: "aws/ec2", detectorName: "aws/ec2"},
		{name: "aws/ecs", detectorName: "aws/ecs"},
		{name: "aws/eks", detectorName: "aws/eks"},
		{name: "aws/lambda", detectorName: "aws/lambda"},
		{name: "gcp", detectorName: "gcp"},
		{name: "azure", detectorName: "azure"},
		{
			name:         "invalid",
			detectorName: "invalid",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			detector, err := GetDetector(ctx, tt.detectorName)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, detector)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, detector)
			}
		})
	}
}

func TestGetDetectors(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	tests := []struct {
		name          string
		detectorNames []string
		wantCount     int
		wantErr       bool
	}{
		{name: "empty", detectorNames: []string{}, wantCount: 0},
		{name: "nil", detectorNames: nil, wantCount: 0},
		{name: "single", detectorNames: []string{"env"}, wantCount: 1},
		{
			name:          "multiple",
			detectorNames: []string{"env", "host"},
			wantCount:     2,
		},
		{
			name:          "invalid entry",
			detectorNames: []string{"env", "invalid"},
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			detectors, err := GetDetectors(ctx, tt.detectorNames)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, detectors)
			} else {
				require.NoError(t, err)
				assert.Len(t, detectors, tt.wantCount)
			}
		})
	}
}

func TestEnvDetector(t *testing.T) {
	ctx := t.Context()
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "custom.key=value")

	det, err := newEnvDetector(ctx)
	require.NoError(t, err)

	res, err := det.Detect(ctx)
	require.NoError(t, err)

	attrs := res.Attributes()
	val, ok := findAttr(attrs, attribute.Key("custom.key"))
	require.True(t, ok)
	assert.Equal(t, "value", val.AsString())
}

func TestHostDetector(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	det, err := newHostDetector(ctx)
	require.NoError(t, err)

	res, err := det.Detect(ctx)
	require.NoError(t, err)

	assert.NotNil(t, res)
}

func TestMultiDetector(t *testing.T) {
	ctx := t.Context()
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "env.key=value")

	envDet, err := newEnvDetector(ctx)
	require.NoError(t, err)

	hostDet, err := newHostDetector(ctx)
	require.NoError(t, err)

	multi := &multiDetector{
		detectors: []resource.Detector{envDet, hostDet},
	}

	res, err := multi.Detect(ctx)
	require.NoError(t, err)

	attrs := res.Attributes()
	val, ok := findAttr(attrs, attribute.Key("env.key"))
	require.True(t, ok)
	assert.Equal(t, "value", val.AsString())
}
