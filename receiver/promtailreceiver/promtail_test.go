package promtailreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/grafana/loki/clients/pkg/promtail/targets/file"

	"github.com/grafana/loki/clients/pkg/promtail/scrapeconfig"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery"

	"github.com/grafana/loki/clients/pkg/promtail/positions"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/promtail"
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
	bla := testdataConfigYaml()
	assert.Equal(t, bla, cfg)
}

func testdataConfigYaml() *PromtailConfig {
	return &PromtailConfig{
		BaseConfig: adapter.BaseConfig{
			ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
			Operators:        []operator.Config{},
		},
		InputConfig: func() promtail.Config {
			c := promtail.NewConfig()
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
