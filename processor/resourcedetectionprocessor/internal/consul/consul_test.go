// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package consul

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
	res, schemaURL, err := detector.Detect(t.Context())
	require.NoError(t, err)
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
	md.AssertExpectations(t)

	expected := map[string]any{
		"host.name":    "hostname",
		"cloud.region": "dc1",
		"host.id":      "00000000-0000-0000-0000-000000000000",
		"test":         "test",
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}

func TestBuildConsulAPIConfig_TokenFile(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(tokenFile, []byte("secret-token-value"), 0o600))

	userCfg := Config{
		TokenFile:          tokenFile,
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
	}

	apiCfg := buildConsulAPIConfig(userCfg)

	// TokenFile must be set on api.Config.TokenFile so the consul SDK reads the file.
	// It must NOT be set on api.Config.Token (which would use the path as a literal token).
	require.Equal(t, tokenFile, apiCfg.TokenFile,
		"TokenFile should be passed to api.Config.TokenFile")
	require.Empty(t, apiCfg.Token,
		"api.Config.Token should not be set when only TokenFile is configured")
}

func TestBuildConsulAPIConfig_Token(t *testing.T) {
	userCfg := Config{
		Token:              "my-secret-token",
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
	}

	apiCfg := buildConsulAPIConfig(userCfg)

	require.Equal(t, "my-secret-token", apiCfg.Token)
	require.Empty(t, apiCfg.TokenFile)
}

func TestBuildConsulAPIConfig_AllFields(t *testing.T) {
	userCfg := Config{
		Address:            "http://consul.local:8500",
		Datacenter:         "dc2",
		Namespace:          "team-a",
		Token:              "direct-token",
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
	}

	apiCfg := buildConsulAPIConfig(userCfg)

	require.Equal(t, "http://consul.local:8500", apiCfg.Address)
	require.Equal(t, "dc2", apiCfg.Datacenter)
	require.Equal(t, "team-a", apiCfg.Namespace)
	require.Equal(t, "direct-token", apiCfg.Token)
}

func TestBuildConsulAPIConfig_Defaults(t *testing.T) {
	userCfg := Config{
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
	}

	apiCfg := buildConsulAPIConfig(userCfg)

	// When no fields are set, api.DefaultConfig() defaults should be preserved
	defaultCfg := api.DefaultConfig()
	require.Equal(t, defaultCfg.Address, apiCfg.Address)
	require.Equal(t, defaultCfg.Datacenter, apiCfg.Datacenter)
}
