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

// benchTopicCase describes a topic-scraper benchmark workload.
type benchTopicCase struct {
	topics     int
	partitions int
	brokers    int
}

func (c benchTopicCase) name() string {
	return fmt.Sprintf("topics=%d/parts=%d/brokers=%d", c.topics, c.partitions, c.brokers)
}

// benchTopicCases is the fixed workload matrix. Keep this stable across the
// before/after measurements.
var benchTopicCases = []benchTopicCase{
	{topics: 10, partitions: 1, brokers: 1},
	{topics: 50, partitions: 10, brokers: 3},
	{topics: 100, partitions: 10, brokers: 3},
}

func Benchmark_TopicScraperFranz_Scrape(b *testing.B) {
	for _, tc := range benchTopicCases {
		b.Run(tc.name(), func(b *testing.B) {
			topicNames := make([]string, tc.topics)
			for i := range topicNames {
				topicNames[i] = fmt.Sprintf("bench-topic-%d", i)
			}

			cluster, clientCfg := kafkatest.NewCluster(b,
				kfake.SeedTopics(int32(tc.partitions), topicNames...),
				kfake.NumBrokers(tc.brokers),
			)
			counter := newKafkaRequestCounter(cluster)

			cfg := Config{
				ClientConfig:         clientCfg,
				MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
				ClusterAlias:         "bench-cluster",
				TopicMatch:           ".*",
			}
			cfg.ResourceAttributes.KafkaClusterAlias.Enabled = true
			// Enable the config-derived metrics so DescribeTopicConfigs runs.
			cfg.Metrics.KafkaTopicLogRetentionPeriod.Enabled = true
			cfg.Metrics.KafkaTopicLogRetentionSize.Enabled = true
			cfg.Metrics.KafkaTopicMinInsyncReplicas.Enabled = true
			cfg.Metrics.KafkaTopicReplicationFactor.Enabled = true

			s, err := createTopicsScraperFranz(b.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
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
			b.ReportMetric(float64(counter.counts[keyListOffsets])/float64(n), "listOffsets/op")
			b.ReportMetric(float64(counter.counts[keyDescribeCfgs])/float64(n), "describeCfgs/op")
			b.ReportMetric(float64(datapoints), "datapoints/scrape")
		})
	}
}
