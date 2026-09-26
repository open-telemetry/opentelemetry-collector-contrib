// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver

import (
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/internal/metadata"
)

func TestRedisRunnable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/38955")
	}
	logger, _ := zap.NewDevelopment()
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = logger
	cfg := createDefaultConfig().(*Config)
	cfg.AddrConfig.Endpoint = "localhost:6379"
	rs := &redisScraper{mb: metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings)}
	runner, err := newRedisScraperWithClient(newFakeClient(), settings, cfg)
	require.NoError(t, err)
	md, err := runner.ScrapeMetrics(t.Context())
	require.NoError(t, err)
	// + 9 because there are three keyspace entries each of which has three metrics
	// -22 because that many recorders either have disabled metrics or keys absent from the test info fixture
	// (includes cluster_enabled, maxmemory, tracking_total_keys, used_memory_overhead, used_memory_startup,
	//  slave_repl_offset, all other cluster_*/links_buffer_limit_exceeded.count fields not in info.txt,
	//  and pubsub_channels, pubsub_shardchannels, pubsub_clients, pubsub_patterns, which are all disabled
	//  by default)
	assert.Equal(t, len(rs.dataPointRecorders())+9-22, md.DataPointCount())
	rm := md.ResourceMetrics().At(0)
	ilm := rm.ScopeMetrics().At(0)
	il := ilm.Scope()
	assert.Equal(t, "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver", il.Name())
}

func TestRedisRunnableCluster(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/38955")
	}
	logger, _ := zap.NewDevelopment()
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = logger
	cfg := createDefaultConfig().(*Config)
	cfg.AddrConfig.Endpoint = "localhost:6379"
	m := &cfg.MetricsBuilderConfig.Metrics
	m.RedisClusterState.Enabled = true
	m.RedisClusterSlotsAssigned.Enabled = true
	m.RedisClusterSlotsOk.Enabled = true
	m.RedisClusterSlotsPfail.Enabled = true
	m.RedisClusterSlotsFail.Enabled = true
	m.RedisClusterKnownNodes.Enabled = true
	m.RedisClusterNodeCount.Enabled = true
	m.RedisClusterUptime.Enabled = true
	m.RedisClusterNodeUptime.Enabled = true
	m.RedisClusterStatsMessagesSent.Enabled = true
	m.RedisClusterStatsMessagesReceived.Enabled = true
	m.RedisClusterLinksBufferLimitExceededCount.Enabled = true

	runner, err := newRedisScraperWithClient(newFakeClusterClient(), settings, cfg)
	require.NoError(t, err)
	md, err := runner.ScrapeMetrics(t.Context())
	require.NoError(t, err)

	ms := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	got := map[string]int64{}
	for i := 0; i < ms.Len(); i++ {
		metric := ms.At(i)
		if !strings.HasPrefix(metric.Name(), "redis.cluster.") {
			continue
		}
		var dps pmetric.NumberDataPointSlice
		switch metric.Type() {
		case pmetric.MetricTypeGauge:
			dps = metric.Gauge().DataPoints()
		case pmetric.MetricTypeSum:
			dps = metric.Sum().DataPoints()
		default:
			continue
		}
		require.Equal(t, 1, dps.Len(), metric.Name())
		got[metric.Name()] = dps.At(0).IntValue()
		if metric.Name() == "redis.cluster.state" {
			state, ok := dps.At(0).Attributes().Get("cluster_state")
			require.True(t, ok)
			assert.Equal(t, "ok", state.Str())
		}
	}

	// Every field of CLUSTER INFO reaches its metric, including the four whose
	// Redis field names differ from the metric name (cluster_size,
	// cluster_current_epoch, cluster_my_epoch,
	// total_cluster_links_buffer_limit_exceeded).
	assert.Equal(t, map[string]int64{
		"redis.cluster.state":                             1,
		"redis.cluster.slots_assigned":                    16384,
		"redis.cluster.slots_ok":                          16384,
		"redis.cluster.slots_pfail":                       0,
		"redis.cluster.slots_fail":                        0,
		"redis.cluster.known_nodes":                       6,
		"redis.cluster.node.count":                        3,
		"redis.cluster.uptime":                            6,
		"redis.cluster.node.uptime":                       2,
		"redis.cluster.stats_messages_sent":               1483972,
		"redis.cluster.stats_messages_received":           1483968,
		"redis.cluster.links_buffer_limit_exceeded.count": 0,
	}, got)
}

// A standalone instance must not be asked for CLUSTER INFO at all, and must not
// emit cluster metrics even when they are enabled.
func TestRedisRunnableStandaloneSkipsClusterInfo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/38955")
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	cfg := createDefaultConfig().(*Config)
	cfg.AddrConfig.Endpoint = "localhost:6379"
	cfg.MetricsBuilderConfig.Metrics.RedisClusterState.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.RedisClusterSlotsAssigned.Enabled = true

	runner, err := newRedisScraperWithClient(newFakeClient(), settings, cfg)
	require.NoError(t, err)
	md, err := runner.ScrapeMetrics(t.Context())
	require.NoError(t, err)

	ms := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := 0; i < ms.Len(); i++ {
		assert.NotContains(t, ms.At(i).Name(), "redis.cluster.")
	}
}

func TestNewReceiver_invalid_endpoint(t *testing.T) {
	c := createDefaultConfig().(*Config)
	_, err := createMetricsReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), c, nil)
	assert.ErrorContains(t, err, "invalid endpoint")
}

func TestNewReceiver_invalid_auth_error(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.TLS = configtls.ClientConfig{
		Config: configtls.Config{
			CAFile: "/invalid",
		},
	}
	r, err := createMetricsReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), c, nil)
	assert.ErrorContains(t, err, "failed to load TLS config")
	assert.Nil(t, r)
}
