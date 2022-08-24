// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jaegerremotesampling

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       config.ComponentID
		expected config.Extension
	}{
		{
			id: config.NewComponentID(typeStr),
			expected: &Config{
				ExtensionSettings:  config.NewExtensionSettings(config.NewComponentID(typeStr)),
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
			id: config.NewComponentIDWithName(typeStr, "1"),
			expected: &Config{
				ExtensionSettings:  config.NewExtensionSettings(config.NewComponentID(typeStr)),
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
			require.NoError(t, config.UnmarshalExtension(sub, cfg))
			assert.NoError(t, cfg.Validate())
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
