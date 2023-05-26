// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lokireceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/lokireceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, "defaults"),
			expected: &Config{
				Protocols: Protocols{
					GRPC: &configgrpc.GRPCServerSettings{
						NetAddr: confignet.NetAddr{
							Endpoint:  "0.0.0.0:3600",
							Transport: "tcp",
						},
					},
					HTTP: &confighttp.HTTPServerSettings{
						Endpoint: "0.0.0.0:3500",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "mixed"),
			expected: &Config{
				Protocols: Protocols{
					GRPC: &configgrpc.GRPCServerSettings{
						NetAddr: confignet.NetAddr{
							Endpoint:  "localhost:4600",
							Transport: "tcp",
						},
					},
					HTTP: &confighttp.HTTPServerSettings{
						Endpoint: "localhost:4500",
					},
				},
				KeepTimestamp: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestInvalidConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id  component.ID
		err string
	}{
		{
			id:  component.NewIDWithName(metadata.Type, "empty"),
			err: "must specify at least one protocol when using the Loki receiver",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			err = component.ValidateConfig(cfg)
			assert.Error(t, err, tt.err)
		})
	}
}

func TestConfigWithUnknownKeysConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id  component.ID
		err string
	}{
		{
			id:  component.NewIDWithName(metadata.Type, "extra_keys"),
			err: "'' has invalid keys: foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			assert.Contains(t, component.UnmarshalConfig(sub, cfg).Error(), tt.err)
		})
	}
}
