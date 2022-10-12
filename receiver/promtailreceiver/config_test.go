// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package promtailreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/grafana/loki/clients/pkg/promtail/positions"
	"github.com/grafana/loki/clients/pkg/promtail/scrapeconfig"
	"github.com/grafana/loki/clients/pkg/promtail/targets/file"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weaveworks/common/server"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(config.NewComponentID("promtail").String())
	require.NoError(t, err)
	require.NoError(t, config.UnmarshalReceiver(sub, cfg))

	assert.NoError(t, cfg.Validate())
	assert.Equal(t, testdataConfigYaml(), cfg)
}

func testdataConfigYaml() *PromtailConfig {
	return &PromtailConfig{
		BaseConfig: adapter.BaseConfig{
			ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
			Operators:        []operator.Config{},
		},
		InputConfig: func() Config {
			c := NewConfig()
			c.Input.PositionsConfig = positions.Config{PositionsFile: "/tmp/positions.yaml"}
			c.Input.ScrapeConfig = []scrapeconfig.Config{
				{
					JobName:        "system",
					PipelineStages: []interface{}{},
					ServiceDiscoveryConfig: scrapeconfig.ServiceDiscoveryConfig{
						StaticConfigs: discovery.StaticConfig{
							{
								Labels: model.LabelSet{
									"job":      "varlogs",
									"__path__": "testdata/simple.log",
								},
								Targets: []model.LabelSet{},
							},
						},
					},
				},
			}

			c.Input.TargetConfig = file.Config{
				SyncPeriod: time.Second * 10,
			}

			return *c
		}(),
	}
}

func TestBuild(t *testing.T) {
	basicConfig := func() *Config {
		cfg := NewConfigWithID("testfile")
		cfg.Input.PositionsConfig = positions.Config{}
		cfg.Input.ScrapeConfig = []scrapeconfig.Config{
			{
				JobName: "testjob",
				ServiceDiscoveryConfig: scrapeconfig.ServiceDiscoveryConfig{
					StaticConfigs: []*targetgroup.Group{
						{
							Labels: model.LabelSet{
								"job":      "varlogs",
								"__path__": "/var/log/example.log",
							},
						},
					},
				},
			},
		}
		cfg.Input.TargetConfig = file.Config{
			SyncPeriod: 10 * time.Second,
		}

		return cfg
	}

	cases := []struct {
		name             string
		modifyBaseConfig func(config *Config)
		errorRequirement require.ErrorAssertionFunc
		validate         func(*testing.T, *PromtailInput)
	}{
		{
			name:             "Basic",
			modifyBaseConfig: func(f *Config) {},
			errorRequirement: require.NoError,
			validate: func(t *testing.T, f *PromtailInput) {
				require.Equal(t, "/var/log/positions.yaml", f.config.Input.PositionsConfig.PositionsFile)
				require.Equal(t, 10*time.Second, f.config.Input.PositionsConfig.SyncPeriod)
				require.Len(t, f.config.Input.ScrapeConfig, 1)

				staticConfigs := f.config.Input.ScrapeConfig[0].ServiceDiscoveryConfig.StaticConfigs
				require.Len(t, staticConfigs, 1)

				require.Equal(t, 10*time.Second, f.config.Input.TargetConfig.SyncPeriod)
			},
		},
		{
			name: "ScrapeConfigMissing",
			modifyBaseConfig: func(f *Config) {
				f.Input.ScrapeConfig = nil
			},
			errorRequirement: require.Error,
			validate:         nil,
		},
		{
			name: "TargetConfigSyncPeriodMissing",
			modifyBaseConfig: func(f *Config) {
				f.Input.TargetConfig = file.Config{}
			},
			errorRequirement: require.Error,
			validate:         nil,
		},
	}

	cfg := basicConfig()

	for _, tc := range cases {
		t.Run(
			tc.name, func(t *testing.T) {
				tc.modifyBaseConfig(cfg)
				op, err := cfg.Build(testutil.Logger(t))
				tc.errorRequirement(t, err)
				if err != nil {
					return
				}

				promtailInput := op.(*PromtailInput)
				tc.validate(t, promtailInput)
			},
		)
	}
}

func TestUnmarshal(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config_stanza.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name:      "default",
				ExpectErr: false,
				Expect:    NewConfig(),
			},
			{
				Name:      "static_config",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Input.ScrapeConfig = []scrapeconfig.Config{
						{
							JobName:        "testjob",
							PipelineStages: []interface{}{},
							ServiceDiscoveryConfig: scrapeconfig.ServiceDiscoveryConfig{
								StaticConfigs: []*targetgroup.Group{
									{
										Labels: model.LabelSet{
											"job":      "varlogs",
											"__path__": "/var/log/example.log",
										},
										Targets: []model.LabelSet{},
									},
								},
							},
						},
					}
					return cfg
				}(),
			},
			{
				Name:      "loki_push_api",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Input.ScrapeConfig = []scrapeconfig.Config{
						{
							JobName:        "push",
							PipelineStages: []interface{}{},
							PushConfig: &scrapeconfig.PushTargetConfig{
								Server: server.Config{
									HTTPListenPort: 3101,
									GRPCListenPort: 3600,
								},
								Labels: model.LabelSet{
									"pushserver": "push1",
								},
								KeepTimestamp: true,
							},
						},
					}
					return cfg
				}(),
			},
		},
	}.Run(t)
}
