// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/kafkatest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

// kafkaRequestCounter installs a kfake control hook that observes (without
// intercepting) every request handled by the cluster and tallies them by Kafka
// API key. Returning (nil, nil, false) marks the request as "not handled" so
// the cluster processes it normally; KeepControl keeps the hook installed for
// the lifetime of the cluster.
//
// This helper, and the benchmarks that use it, are intentionally kept stable so
// that the same benchmark code can be used to capture a baseline (before the
// fix) and an after measurement (post fix) and compared with `benchstat`.
type kafkaRequestCounter struct {
	counts map[int16]int64
}

func newKafkaRequestCounter(c *kfake.Cluster) *kafkaRequestCounter {
	rc := &kafkaRequestCounter{counts: make(map[int16]int64)}
	c.Control(func(req kmsg.Request) (kmsg.Response, error, bool) {
		c.KeepControl()
		rc.counts[req.Key()]++
		return nil, nil, false
	})
	return rc
}

// total returns the sum of all observed requests.
func (rc *kafkaRequestCounter) total() int64 {
	var t int64
	for _, v := range rc.counts {
		t += v
	}
	return t
}

// reset zeroes the counters (used to ignore client warm-up requests issued
// before the timed loop).
func (rc *kafkaRequestCounter) reset() {
	for k := range rc.counts {
		rc.counts[k] = 0
	}
}

// Kafka protocol API keys we care about for these benchmarks.
const (
	keyMetadata     int16 = 3
	keyListOffsets  int16 = 2
	keyOffsetFetch  int16 = 9
	keyListGroups   int16 = 16
	keyDescribeCfgs int16 = 32
	keyDescribeGrps int16 = 15
)

// seedConsumerGroups commits offsets for nGroups consumer groups. Each group
// only commits to topicsPerGroup of the available topics (assigned round-robin),
// which mirrors real deployments where a consumer group consumes a small subset
// of the cluster's topics rather than every topic. It returns the group names so
// the caller can clean them up.
func seedConsumerGroups(tb testing.TB, cl *kgo.Client, nGroups, partitions, topicsPerGroup int, topics []string) []string {
	tb.Helper()
	adm := kadm.NewClient(cl)
	groups := make([]string, 0, nGroups)
	for g := range nGroups {
		group := fmt.Sprintf("bench-group-%d", g)
		groups = append(groups, group)
		var os kadm.Offsets
		for i := range topicsPerGroup {
			t := topics[(g+i)%len(topics)]
			for p := range partitions {
				os.AddOffset(t, int32(p), int64(g+1), -1)
			}
		}
		_, err := adm.CommitOffsets(tb.Context(), group, os)
		require.NoError(tb, err)
	}
	return groups
}

// benchConsumerCase describes a consumer-scraper benchmark workload.
//
// topicsPerGroup controls how many of the cluster's topics each consumer group
// actually commits offsets to. This is the key dimension for #48755: the
// original implementation records offset/lag data points for every matched
// topic-partition for every group (emitting -1 placeholders for topics a group
// never consumed), so its data point count scales with groups*topics*partitions
// regardless of how many topics each group truly consumes.
type benchConsumerCase struct {
	groups         int
	topics         int
	partitions     int
	topicsPerGroup int
}

func (c benchConsumerCase) name() string {
	return fmt.Sprintf("groups=%d/topics=%d/parts=%d/perGroup=%d", c.groups, c.topics, c.partitions, c.topicsPerGroup)
}

// benchConsumerCases is the fixed workload matrix. Keep this stable across the
// before/after measurements so results can be compared with benchstat.
var benchConsumerCases = []benchConsumerCase{
	// Each group consumes a single topic (realistic).
	{groups: 1, topics: 10, partitions: 1, topicsPerGroup: 1},
	{groups: 10, topics: 10, partitions: 1, topicsPerGroup: 1},
	{groups: 50, topics: 10, partitions: 1, topicsPerGroup: 1},
	{groups: 10, topics: 50, partitions: 10, topicsPerGroup: 1},
	{groups: 50, topics: 50, partitions: 10, topicsPerGroup: 2},
}

func Benchmark_ConsumerScraperFranz_Scrape(b *testing.B) {
	for _, tc := range benchConsumerCases {
		b.Run(tc.name(), func(b *testing.B) {
			topicNames := make([]string, tc.topics)
			for i := range topicNames {
				topicNames[i] = fmt.Sprintf("bench-topic-%d", i)
			}

			cluster, clientCfg := kafkatest.NewCluster(b,
				kfake.SeedTopics(int32(tc.partitions), topicNames...),
			)
			counter := newKafkaRequestCounter(cluster)

			cl, err := kgo.NewClient(kgo.SeedBrokers(cluster.ListenAddrs()...))
			require.NoError(b, err)
			b.Cleanup(cl.Close)

			groups := seedConsumerGroups(b, cl, tc.groups, tc.partitions, tc.topicsPerGroup, topicNames)

			cfg := Config{
				ClientConfig:         clientCfg,
				MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
				ClusterAlias:         "bench-cluster",
				TopicMatch:           ".*",
				GroupMatch:           ".*",
			}
			cfg.ResourceAttributes.KafkaClusterAlias.Enabled = true

			s, err := createConsumerScraperFranz(b.Context(), cfg, receivertest.NewNopSettings(metadata.Type), nil)
			require.NoError(b, err)
			require.NoError(b, s.Start(b.Context(), componenttest.NewNopHost()))
			b.Cleanup(func() { require.NoError(b, s.Shutdown(b.Context())) })

			// Warm up the scraper/client (lazy client creation, metadata cache)
			// so the timed loop measures steady-state scrape cost.
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
			b.ReportMetric(float64(counter.counts[keyOffsetFetch])/float64(n), "offsetFetch/op")
			b.ReportMetric(float64(counter.counts[keyMetadata])/float64(n), "metadata/op")
			b.ReportMetric(float64(counter.counts[keyListOffsets])/float64(n), "listOffsets/op")
			b.ReportMetric(float64(counter.counts[keyDescribeGrps])/float64(n), "describeGrps/op")
			b.ReportMetric(float64(datapoints), "datapoints/scrape")

			// Clean up groups so kfake goroutines exit promptly.
			adm := kadm.NewClient(cl)
			_, _ = adm.DeleteGroups(b.Context(), groups...)
		})
	}
}
