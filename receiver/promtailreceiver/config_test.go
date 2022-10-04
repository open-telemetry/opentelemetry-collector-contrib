package promtailreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"

	"github.com/prometheus/prometheus/discovery/targetgroup"

	"github.com/grafana/loki/clients/pkg/promtail/targets/file"

	"github.com/grafana/loki/clients/pkg/promtail/scrapeconfig"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery"

	"github.com/grafana/loki/clients/pkg/promtail/positions"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap/confmaptest"
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
			c.PositionsConfig = positions.Config{PositionsFile: "/tmp/positions.yaml"}
			c.ScrapeConfig = []scrapeconfig.Config{
				{
					JobName: "system",
					ServiceDiscoveryConfig: scrapeconfig.ServiceDiscoveryConfig{
						StaticConfigs: discovery.StaticConfig{
							{
								Labels: model.LabelSet{
									"job":      "varlogs",
									"__path__": "testdata/simple.log",
								},
							},
						},
					},
				},
			}

			syncPeriod, _ := time.ParseDuration("10s")

			c.TargetConfig = file.Config{
				SyncPeriod: syncPeriod,
			}

			return *c
		}(),
	}
}

func TestBuild(t *testing.T) {
	basicConfig := func() *Config {
		cfg := NewConfigWithID("testfile")
		cfg.PositionsConfig = positions.Config{}
		cfg.ScrapeConfig = []scrapeconfig.Config{
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
		cfg.TargetConfig = file.Config{
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
				require.Equal(t, "/var/log/positions.yaml", f.config.PositionsConfig.PositionsFile)
				require.Equal(t, 10*time.Second, f.config.PositionsConfig.SyncPeriod)
				require.Len(t, f.config.ScrapeConfig, 1)

				staticConfigs := f.config.ScrapeConfig[0].ServiceDiscoveryConfig.StaticConfigs
				require.Len(t, staticConfigs, 1)

				require.Equal(t, 10*time.Second, f.config.TargetConfig.SyncPeriod)
			},
		},
		{
			name: "ScrapeConfigMissing",
			modifyBaseConfig: func(f *Config) {
				f.ScrapeConfig = nil
			},
			errorRequirement: require.Error,
			validate:         nil,
		},
		{
			name: "TargetConfigSyncPeriodMissing",
			modifyBaseConfig: func(f *Config) {
				f.TargetConfig = file.Config{}
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
					cfg.ScrapeConfig = []scrapeconfig.Config{
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
					return cfg
				}(),
			},
		},
	}.Run(t)
}
