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
				HTTPServerSettings: &confighttp.HTTPServerSettings{Endpoint: ":5778"},
				GRPCServerSettings: &configgrpc.GRPCServerSettings{NetAddr: confignet.NetAddr{
					Endpoint:  ":14250",
					Transport: "tcp",
				}},
				Source: Source{
					Remote: &configgrpc.GRPCClientSettings{
						Endpoint: "jaeger-collector:14250",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "1"),
			expected: &Config{
				HTTPServerSettings: &confighttp.HTTPServerSettings{Endpoint: ":5778"},
				GRPCServerSettings: &configgrpc.GRPCServerSettings{NetAddr: confignet.NetAddr{
					Endpoint:  ":14250",
					Transport: "tcp",
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
			require.NoError(t, component.UnmarshalConfig(sub, cfg))
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
				GRPCServerSettings: &configgrpc.GRPCServerSettings{},
			},
			expected: errNoSources,
		},
		{
			desc: "too many sources",
			cfg: Config{
				GRPCServerSettings: &configgrpc.GRPCServerSettings{},
				Source: Source{
					Remote: &configgrpc.GRPCClientSettings{},
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
