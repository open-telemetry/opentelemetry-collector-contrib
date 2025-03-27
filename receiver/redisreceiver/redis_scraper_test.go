// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
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
	cfg.Endpoint = "localhost:6379"
	rs := &redisScraper{mb: metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings)}
	runner, err := newRedisScraperWithClient(newFakeClient(), settings, cfg)
	require.NoError(t, err)
	md, err := runner.ScrapeMetrics(context.Background())
	require.NoError(t, err)
	// + 9 because there are three keyspace entries each of which has three metrics
	// -2 because maxmemory and slave_repl_offset is by default disabled, so recorder is there, but there won't be data point
	assert.Equal(t, len(rs.dataPointRecorders())+9-2, md.DataPointCount())
	rm := md.ResourceMetrics().At(0)
	ilm := rm.ScopeMetrics().At(0)
	il := ilm.Scope()
	assert.Equal(t, "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver", il.Name())
}

func TestNewReceiver_invalid_endpoint(t *testing.T) {
	c := createDefaultConfig().(*Config)
	_, err := createMetricsReceiver(context.Background(), receivertest.NewNopSettings(metadata.Type), c, nil)
	assert.ErrorContains(t, err, "invalid endpoint")
}

func TestNewReceiver_invalid_auth_error(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.TLS = configtls.ClientConfig{
		Config: configtls.Config{
			CAFile: "/invalid",
		},
	}
	r, err := createMetricsReceiver(context.Background(), receivertest.NewNopSettings(metadata.Type), c, nil)
	assert.ErrorContains(t, err, "failed to load TLS config")
	assert.Nil(t, r)
}
