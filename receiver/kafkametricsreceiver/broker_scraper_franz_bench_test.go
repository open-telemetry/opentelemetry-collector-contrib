// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/kafkatest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

// benchBrokerCase describes a broker-scraper benchmark workload.
type benchBrokerCase struct {
	brokers int
}

func (c benchBrokerCase) name() string {
	return fmt.Sprintf("brokers=%d", c.brokers)
}

// benchBrokerCases is the fixed workload matrix. Keep this stable across the
// before/after measurements.
var benchBrokerCases = []benchBrokerCase{
	{brokers: 1},
	{brokers: 3},
	{brokers: 5},
}

func Benchmark_BrokerScraperFranz_Scrape(b *testing.B) {
	for _, tc := range benchBrokerCases {
		b.Run(tc.name(), func(b *testing.B) {
			cluster, clientCfg := kafkatest.NewCluster(b,
				kfake.SeedTopics(1, "bench-meta-topic"),
				kfake.NumBrokers(tc.brokers),
				kfake.BrokerConfigs(map[string]string{
					logRetentionHours: "168",
				}),
			)
			counter := newKafkaRequestCounter(cluster)

			cfg := Config{
				ClientConfig:         clientCfg,
				MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
				ClusterAlias:         "bench-cluster",
			}
			cfg.ResourceAttributes.KafkaClusterAlias.Enabled = true
			// Enable the config-derived metric so DescribeBrokerConfigs runs.
			cfg.Metrics.KafkaBrokerLogRetentionPeriod.Enabled = true

			s, err := createBrokerScraperFranz(b.Context(), cfg, receivertest.NewNopSettings(metadata.Type), nil)
			require.NoError(b, err)
			require.NoError(b, s.Start(b.Context(), componenttest.NewNopHost()))
			b.Cleanup(func() { require.NoError(b, s.Shutdown(b.Context())) })

			md, err := s.ScrapeMetrics(b.Context())
			require.NoError(b, err)
			datapoints := md.DataPointCount()

			counter.reset()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := s.ScrapeMetrics(b.Context())
				if err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()

			n := int64(b.N)
			b.ReportMetric(float64(counter.total())/float64(n), "reqs/op")
			b.ReportMetric(float64(counter.counts[keyMetadata])/float64(n), "metadata/op")
			b.ReportMetric(float64(counter.counts[keyDescribeCfgs])/float64(n), "describeCfgs/op")
			b.ReportMetric(float64(datapoints), "datapoints/scrape")
		})
	}
}
