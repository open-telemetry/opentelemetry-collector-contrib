// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerremotesampling

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewID(metadata.Type),
			expected: &Config{
				HTTPServerConfig: &confighttp.ServerConfig{Endpoint: "localhost:5778"},
				GRPCServerConfig: &configgrpc.ServerConfig{NetAddr: confignet.AddrConfig{
					Endpoint:  "localhost:14250",
					Transport: confignet.TransportTypeTCP,
				}},
				Source: Source{
					Remote: &configgrpc.ClientConfig{
						Endpoint: "jaeger-collector:14250",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "1"),
			expected: &Config{
				HTTPServerConfig: &confighttp.ServerConfig{Endpoint: "localhost:5778"},
				GRPCServerConfig: &configgrpc.ServerConfig{NetAddr: confignet.AddrConfig{
					Endpoint:  "localhost:14250",
					Transport: confignet.TransportTypeTCP,
				}},
				Source: Source{
					ReloadInterval: time.Second,
					File:           "/etc/otelcol/sampling_strategies.json",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidate(t *testing.T) {
	testCases := []struct {
		desc     string
		cfg      Config
		expected error
	}{
		{
			desc:     "no receiving protocols",
			cfg:      Config{},
			expected: errAtLeastOneProtocol,
		},
		{
			desc: "no sources",
			cfg: Config{
				GRPCServerConfig: &configgrpc.ServerConfig{},
			},
			expected: errNoSources,
		},
		{
			desc: "too many sources",
			cfg: Config{
				GRPCServerConfig: &configgrpc.ServerConfig{},
				Source: Source{
					Remote: &configgrpc.ClientConfig{},
					File:   "/tmp/some-file",
				},
			},
			expected: errTooManySources,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			res := tC.cfg.Validate()
			assert.Equal(t, tC.expected, res)
		})
	}
}
