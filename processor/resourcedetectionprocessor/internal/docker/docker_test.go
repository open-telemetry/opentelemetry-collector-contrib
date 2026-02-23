// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types/container"
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

func (m *mockMetadata) ContainerInfo(context.Context) (container.InspectResponse, error) {
	args := m.MethodCalled("ContainerInfo")
	return args.Get(0).(container.InspectResponse), args.Error(1)
}

func TestDetect(t *testing.T) {
	md := &mockMetadata{}
	md.On("Hostname").Return("hostname", nil)
	md.On("OSType").Return("darwin", nil)
	md.On("ContainerInfo").Return(container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			Name:  "foo",
			Image: "bar:1.0",
		},
	}, nil)

	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.ContainerImageName.Enabled = true
	cfg.ResourceAttributes.ContainerName.Enabled = true

	detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg)
	require.NoError(t, err)
	detector.(*Detector).provider = md
	res, schemaURL, err := detector.Detect(t.Context())
	require.NoError(t, err)
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
	md.AssertExpectations(t)

	expected := map[string]any{
		"host.name":            "hostname",
		"os.type":              "darwin",
		"container.image.name": "bar:1.0",
		"container.name":       "foo",
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}
