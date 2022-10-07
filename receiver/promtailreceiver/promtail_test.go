package promtailreceiver

import (
	"testing"
	"time"

	"github.com/grafana/loki/clients/pkg/promtail/api"
	"github.com/grafana/loki/clients/pkg/promtail/scrapeconfig"
	"github.com/grafana/loki/clients/pkg/promtail/targets/file"
	"github.com/grafana/loki/pkg/logproto"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/stretchr/testify/require"
)

func TestPromtailInput_parsePromtailEntry(t *testing.T) {
	basicConfig := func() *Config {
		cfg := NewConfigWithID("testfile")
		cfg.Input.ScrapeConfig = []scrapeconfig.Config{
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
		cfg.Input.TargetConfig = file.Config{
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
