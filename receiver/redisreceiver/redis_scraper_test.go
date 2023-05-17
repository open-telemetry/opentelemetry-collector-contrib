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
	rs := &redisScraper{mb: metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings)}
	runner, err := newRedisScraperWithClient(newFakeClient(), settings, cfg)
	require.NoError(t, err)
	md, err := runner.Scrape(context.Background())
	require.NoError(t, err)
	// + 6 because there are two keyspace entries each of which has three metrics
	// -1 because maxmemory is by default disabled, so recorder is there, but there won't be data point
	assert.Equal(t, len(rs.dataPointRecorders())+6-1, md.DataPointCount())
	rm := md.ResourceMetrics().At(0)
	ilm := rm.ScopeMetrics().At(0)
	il := ilm.Scope()
	assert.Equal(t, "otelcol/redisreceiver", il.Name())
	// Only version should be enabled by default at this moment
	assert.Equal(t, 1, rm.Resource().Attributes().Len())
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
