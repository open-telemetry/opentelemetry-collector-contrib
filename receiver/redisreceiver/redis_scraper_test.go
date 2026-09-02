// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/pdata/pcommon"
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
	// -22 because that many recorders are disabled by default: maxmemory, tracking_total_keys,
	// used_memory_overhead, used_memory_startup, slave_repl_offset, all 13 redis.cluster.*
	// recorders (populated from info.txt/cluster_info.txt but disabled by default), and
	// pubsub_channels, pubsub_shardchannels, pubsub_clients, pubsub_patterns.
	assert.Equal(t, len(rs.dataPointRecorders())+9-22, md.DataPointCount())
	rm := md.ResourceMetrics().At(0)
	ilm := rm.ScopeMetrics().At(0)
	il := ilm.Scope()
	assert.Equal(t, "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver", il.Name())
}

// TestRedisRunnable_ClusterMetrics verifies that, once enabled, the redis.cluster.* metrics
// are actually populated from the CLUSTER INFO command's output (testdata/cluster_info.txt),
// using the real Redis CLUSTER INFO field names.
func TestRedisRunnable_ClusterMetrics(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/38955")
	}
	logger, _ := zap.NewDevelopment()
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = logger
	cfg := createDefaultConfig().(*Config)
	cfg.AddrConfig.Endpoint = "localhost:6379"
	m := &cfg.MetricsBuilderConfig.Metrics
	m.RedisClusterClusterEnabled.Enabled = true
	m.RedisClusterKnownNodes.Enabled = true
	m.RedisClusterLinksBufferLimitExceededCount.Enabled = true
	m.RedisClusterNodeCount.Enabled = true
	m.RedisClusterNodeUptime.Enabled = true
	m.RedisClusterSlotsAssigned.Enabled = true
	m.RedisClusterSlotsFail.Enabled = true
	m.RedisClusterSlotsOk.Enabled = true
	m.RedisClusterSlotsPfail.Enabled = true
	m.RedisClusterState.Enabled = true
	m.RedisClusterStatsMessagesReceived.Enabled = true
	m.RedisClusterStatsMessagesSent.Enabled = true
	m.RedisClusterUptime.Enabled = true

	runner, err := newRedisScraperWithClient(newFakeClient(), settings, cfg)
	require.NoError(t, err)
	md, err := runner.ScrapeMetrics(t.Context())
	require.NoError(t, err)

	metricsByName := map[string]pmetric.Metric{}
	ms := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := range ms.Len() {
		metricsByName[ms.At(i).Name()] = ms.At(i)
	}

	requireGaugeInt := func(name string, want int64) {
		metric, ok := metricsByName[name]
		require.True(t, ok, "missing metric %s", name)
		require.Equal(t, want, metric.Gauge().DataPoints().At(0).IntValue(), name)
	}
	requireSumInt := func(name string, want int64) {
		metric, ok := metricsByName[name]
		require.True(t, ok, "missing metric %s", name)
		require.Equal(t, want, metric.Sum().DataPoints().At(0).IntValue(), name)
	}

	requireGaugeInt("redis.cluster.cluster_enabled", 0)
	requireGaugeInt("redis.cluster.known_nodes", 6)
	requireSumInt("redis.cluster.links_buffer_limit_exceeded.count", 0)
	requireGaugeInt("redis.cluster.node.count", 3)
	requireGaugeInt("redis.cluster.node.uptime", 2)
	requireGaugeInt("redis.cluster.slots_assigned", 16384)
	requireGaugeInt("redis.cluster.slots_fail", 0)
	requireGaugeInt("redis.cluster.slots_ok", 16384)
	requireGaugeInt("redis.cluster.slots_pfail", 0)
	requireGaugeInt("redis.cluster.uptime", 6)
	requireSumInt("redis.cluster.stats_messages_received", 1483968)
	requireSumInt("redis.cluster.stats_messages_sent", 1483972)

	stateMetric, ok := metricsByName["redis.cluster.state"]
	require.True(t, ok, "missing metric redis.cluster.state")
	dp := stateMetric.Gauge().DataPoints().At(0)
	require.Equal(t, int64(1), dp.IntValue())
	clusterState, ok := dp.Attributes().Get("cluster_state")
	require.True(t, ok)
	require.Equal(t, "ok", clusterState.Str())
}

// TestRecordCommonMetrics_ClusterStateFail covers the branch of the cluster_state handling
// where the cluster is not healthy (any value other than "ok" is reported as fail per the
// CLUSTER INFO documentation).
func TestRecordCommonMetrics_ClusterStateFail(t *testing.T) {
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = zap.NewNop()
	cfg := createDefaultConfig().(*Config)
	cfg.MetricsBuilderConfig.Metrics.RedisClusterState.Enabled = true
	rs := &redisScraper{
		settings: settings.TelemetrySettings,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}

	rs.recordCommonMetrics(pcommon.NewTimestampFromTime(time.Now()), info{"cluster_state": "fail"}, rs.dataPointRecorders())

	md := rs.mb.Emit()
	metric := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	require.Equal(t, "redis.cluster.state", metric.Name())
	dp := metric.Gauge().DataPoints().At(0)
	require.Equal(t, int64(1), dp.IntValue())
	clusterState, ok := dp.Attributes().Get("cluster_state")
	require.True(t, ok)
	require.Equal(t, "fail", clusterState.Str())
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
