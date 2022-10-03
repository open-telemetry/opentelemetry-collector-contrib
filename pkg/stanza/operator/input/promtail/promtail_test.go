package promtail

import (
	"testing"
	"time"

	"github.com/grafana/loki/clients/pkg/promtail/api"
	"github.com/grafana/loki/clients/pkg/promtail/positions"
	"github.com/grafana/loki/clients/pkg/promtail/scrapeconfig"
	"github.com/grafana/loki/clients/pkg/promtail/targets/file"
	"github.com/grafana/loki/pkg/logproto"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

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
								"job":    "varlogs",
								"__path": "/var/log/example.log",
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

func TestPromtailInput_parsePromtailEntry(t *testing.T) {
	basicConfig := func() *Config {
		cfg := NewConfigWithID("testfile")
		cfg.ScrapeConfig = []scrapeconfig.Config{
			{
				JobName: "testjob",
				ServiceDiscoveryConfig: scrapeconfig.ServiceDiscoveryConfig{
					StaticConfigs: []*targetgroup.Group{
						{
							Labels: model.LabelSet{
								"job":    "varlogs",
								"__path": "/var/log/example.log",
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

	cfg := basicConfig()
	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)

	promtailInput := op.(*PromtailInput)

	cases := []struct {
		name        string
		inputEntry  api.Entry
		outputEntry entry.Entry
	}{
		{
			name: "Success",
			inputEntry: api.Entry{
				Labels: model.LabelSet{
					"filename": "/var/log/example.log",
					"job":      "varlogs",
				},
				Entry: logproto.Entry{
					Timestamp: time.Now(),
					Line:      "test message",
				},
			},
			outputEntry: entry.Entry{
				Body: "test message",
				Attributes: map[string]interface{}{
					"log.file.name": "example.log",
					"log.file.path": "/var/log/example.log",
					"job":           "varlogs",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(
			tc.name, func(t *testing.T) {
				outputEntry, err := promtailInput.parsePromtailEntry(tc.inputEntry)
				require.NoError(t, err)
				require.Equal(t, tc.outputEntry.Body, outputEntry.Body)
				require.Equal(t, tc.outputEntry.Attributes, outputEntry.Attributes)
			},
		)
	}
}
