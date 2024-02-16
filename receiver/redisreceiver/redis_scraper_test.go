// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/internal/metadata"
)

func TestRedisRunnable(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	settings := receivertest.NewNopCreateSettings()
	settings.Logger = logger
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:6379"
	rs := &redisScraper{mb: metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings)}
	runner, err := newRedisScraperWithClient(newFakeClient(), settings, cfg)
	require.NoError(t, err)
	md, err := runner.Scrape(context.Background())
	require.NoError(t, err)
	// + 6 because there are two keyspace entries each of which has three metrics
	// -2 because maxmemory and slave_repl_offset is by default disabled, so recorder is there, but there won't be data point
	assert.Equal(t, len(rs.dataPointRecorders())+6-2, md.DataPointCount())
	rm := md.ResourceMetrics().At(0)
	ilm := rm.ScopeMetrics().At(0)
	il := ilm.Scope()
	assert.Equal(t, "otelcol/redisreceiver", il.Name())
}

func TestNewReceiver_invalid_endpoint(t *testing.T) {
	c := createDefaultConfig().(*Config)
	_, err := createMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), c, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid endpoint")
}

func TestNewReceiver_invalid_auth_error(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.TLS = configtls.TLSClientSetting{
		TLSSetting: configtls.TLSSetting{
			CAFile: "/invalid",
		},
	}
	r, err := createMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), c, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load TLS config")
	assert.Nil(t, r)
}
