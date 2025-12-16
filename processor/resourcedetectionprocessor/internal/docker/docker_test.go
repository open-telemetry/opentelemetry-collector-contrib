// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/docker"
)

var _ docker.Provider = (*mockMetadata)(nil)

type mockMetadata struct {
	mock.Mock
}

func (m *mockMetadata) Hostname(context.Context) (string, error) {
	args := m.MethodCalled("Hostname")
	return args.String(0), args.Error(1)
}

func (m *mockMetadata) OSType(context.Context) (string, error) {
	args := m.MethodCalled("OSType")
	return args.String(0), args.Error(1)
}

func TestDetect(t *testing.T) {
	md := &mockMetadata{}
	md.On("Hostname").Return("hostname", nil)
	md.On("OSType").Return("darwin", nil)

	detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)
	detector.(*Detector).provider = md
	res, schemaURL, err := detector.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "https://opentelemetry.io/schemas/1.37.0", schemaURL)
	md.AssertExpectations(t)

	expected := map[string]any{
		"host.name": "hostname",
		"os.type":   "darwin",
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}
