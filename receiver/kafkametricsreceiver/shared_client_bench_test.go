// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kmsg"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/kafkatest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

// ApiVersions is issued once per broker connection, so it is a proxy for the
// number of distinct client connections established against the cluster.
const (
	rcvKeyAPIVersions int16 = 18
	rcvKeyMetadata    int16 = 3
)

// receiverRequestCounter tallies requests handled by the fake cluster by API key.
type receiverRequestCounter struct {
	counts map[int16]int64
}

func newReceiverRequestCounter(c *kfake.Cluster) *receiverRequestCounter {
	rc := &receiverRequestCounter{counts: make(map[int16]int64)}
	c.Control(func(req kmsg.Request) (kmsg.Response, error, bool) {
		c.KeepControl()
		rc.counts[req.Key()]++
		return nil, nil, false
	})
	return rc
}

func (rc *receiverRequestCounter) total() int64 {
	var t int64
	for _, v := range rc.counts {
		t += v
	}
	return t
}

func (rc *receiverRequestCounter) reset() {
	for k := range rc.counts {
		rc.counts[k] = 0
	}
}

// Benchmark_Receiver_AllScrapers measures one full scrape cycle of a receiver
// running the brokers, topics and consumers scrapers together. It drives the
// receiver through newMetricsReceiver and is self-contained, so the same
// benchmark compiles before and after the shared-client change for benchstat.
func Benchmark_Receiver_AllScrapers(b *testing.B) {
	topicNames := make([]string, 20)
	for i := range topicNames {
		topicNames[i] = fmt.Sprintf("bench-topic-%d", i)
	}

	cluster, clientCfg := kafkatest.NewCluster(b,
		kfake.SeedTopics(5, topicNames...),
		kfake.NumBrokers(3),
	)
	counter := newReceiverRequestCounter(cluster)

	cfg := createDefaultConfig().(*Config)
	cfg.ClientConfig = clientCfg
	cfg.ClusterAlias = "bench-cluster"
	cfg.TopicMatch = ".*"
	cfg.GroupMatch = ".*"
	cfg.Scrapers = []string{"brokers", "topics", "consumers"}
	cfg.Metrics.KafkaBrokerLogRetentionPeriod.Enabled = true
	// Long interval + no initial delay: measure one scrape cycle per iteration.
	cfg.CollectionInterval = time.Hour
	cfg.InitialDelay = 0

	sink := new(consumertest.MetricsSink)
	runCycle := func() {
		sink.Reset()
		r, err := newMetricsReceiver(b.Context(), *cfg, receivertest.NewNopSettings(metadata.Type), sink)
		require.NoError(b, err)
		require.NoError(b, r.Start(b.Context(), componenttest.NewNopHost()))
		// Initial scrape runs asynchronously; wait for it before shutting down.
		require.Eventually(b, func() bool {
			return sink.DataPointCount() > 0
		}, 10*time.Second, time.Millisecond)
		require.NoError(b, r.Shutdown(b.Context()))
	}

	// Warm up (kfake topic/group setup) outside the measured counters.
	runCycle()

	counter.reset()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runCycle()
	}
	b.StopTimer()

	n := int64(b.N)
	b.ReportMetric(float64(counter.counts[rcvKeyAPIVersions])/float64(n), "apiVersions/op")
	b.ReportMetric(float64(counter.counts[rcvKeyMetadata])/float64(n), "metadata/op")
	b.ReportMetric(float64(counter.total())/float64(n), "reqs/op")
}
