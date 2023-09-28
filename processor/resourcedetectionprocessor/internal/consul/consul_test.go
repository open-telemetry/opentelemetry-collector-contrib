// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package consul

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/consul"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/consul/internal/metadata"
)

var _ consul.Provider = (*mockMetadata)(nil)

type mockMetadata struct {
	mock.Mock
}

func (m *mockMetadata) Metadata(context.Context) (*consul.Metadata, error) {
	args := m.MethodCalled("Metadata")

	return args.Get(0).(*consul.Metadata), args.Error(1)
}

func TestDetect(t *testing.T) {
	md := &mockMetadata{}
	md.On("Metadata").Return(
		&consul.Metadata{
			Hostname:     "hostname",
			Datacenter:   "dc1",
			NodeID:       "00000000-0000-0000-0000-000000000000",
			HostMetadata: map[string]string{"test": "test"},
		},
		nil,
	)
	detector := &Detector{
		provider: md,
		logger:   zap.NewNop(),
		rb:       metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig()),
	}
	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	md.AssertExpectations(t)

	expected := map[string]any{
		conventions.AttributeHostName:    "hostname",
		conventions.AttributeCloudRegion: "dc1",
		conventions.AttributeHostID:      "00000000-0000-0000-0000-000000000000",
		"test":                           "test",
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}
