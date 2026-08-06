// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

// newTestHealthScraper builds a connectionHealthScraper wired to the supplied
// DB client, bypassing Start so tests can inject a fake client directly.
func newTestHealthScraper(t *testing.T, cfg *Config, client sqlquery.DbClient) *connectionHealthScraper {
	t.Helper()
	params := receivertest.NewNopSettings(metadata.Type)
	id := component.NewIDWithName(metadata.Type, "connection-health")
	s := newConnectionHealthScraper(id, sqlquery.TelemetryConfig{}, nil, sqlquery.NewDbClient, params, cfg)
	s.client = client
	return s
}

func TestBoolToStatus(t *testing.T) {
	assert.Equal(t, int64(1), boolToStatus(true))
	assert.Equal(t, int64(0), boolToStatus(false))
}

func TestProbeReachable(t *testing.T) {
	// A live listener on loopback: the endpoint accepts a TCP connection.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	reachableCfg := &Config{DataSource: "server=127.0.0.1;port=" + portStr}
	host, port, err := resolveConfiguredHostPort(reachableCfg)
	require.NoError(t, err)
	s := newTestHealthScraper(t, reachableCfg, &sqlquery.FakeDBClient{})
	assert.True(t, s.probeReachable(t.Context(), host, port), "listener is up, endpoint should be reachable")

	// Close the listener to obtain a port that is guaranteed to refuse
	// connections, then probe it: reachable must be false.
	require.NoError(t, ln.Close())
	s = newTestHealthScraper(t, reachableCfg, &sqlquery.FakeDBClient{})
	assert.False(t, s.probeReachable(t.Context(), host, port), "closed port should not be reachable")
}

func TestProbeReachableDefaultPort(t *testing.T) {
	// When the config has no explicit port, resolveConfiguredHostPort falls back
	// to the SQL Server default (1433). We can't guarantee 1433 is closed on the
	// test host, so we only assert the probe returns without panicking.
	cfg := &Config{DataSource: "server=127.0.0.1"}
	host, port, err := resolveConfiguredHostPort(cfg)
	require.NoError(t, err)
	assert.Equal(t, 1433, port, "missing port should default to 1433")
	s := newTestHealthScraper(t, cfg, &sqlquery.FakeDBClient{})
	assert.NotPanics(t, func() { s.probeReachable(t.Context(), host, port) })
}

func TestProbeQueryable(t *testing.T) {
	cfg := &Config{DataSource: "server=127.0.0.1;port=1433"}

	// A client that returns a row for SELECT 1 => queryable.
	okClient := &sqlquery.FakeDBClient{
		StringMaps: [][]sqlquery.StringMap{{{"": "1"}}},
	}
	s := newTestHealthScraper(t, cfg, okClient)
	assert.True(t, s.probeQueryable(t.Context()))

	// A client that errors (e.g. auth failure) => not queryable.
	errClient := &sqlquery.FakeDBClient{Err: errors.New("login failed")}
	s = newTestHealthScraper(t, cfg, errClient)
	assert.False(t, s.probeQueryable(t.Context()))
}

// statusByCheck extracts the sqlserver.health datapoint values keyed by the
// check attribute from an emitted metrics batch.
func statusByCheck(t *testing.T, md pmetric.Metrics) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				if m.Name() != "sqlserver.health" {
					continue
				}
				dps := m.Gauge().DataPoints()
				for d := 0; d < dps.Len(); d++ {
					dp := dps.At(d)
					check, ok := dp.Attributes().Get("check")
					require.True(t, ok, "status datapoint missing check attribute")
					out[check.Str()] = dp.IntValue()
				}
			}
		}
	}
	return out
}

func TestScrapeMetricsReachableAndQueryable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	cfg := &Config{
		DataSource:           "server=127.0.0.1;port=" + portStr,
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
	}
	client := &sqlquery.FakeDBClient{StringMaps: [][]sqlquery.StringMap{{{"": "1"}}}}
	s := newTestHealthScraper(t, cfg, client)

	md, err := s.ScrapeMetrics(t.Context())
	require.NoError(t, err)

	got := statusByCheck(t, md)
	assert.Equal(t, int64(1), got["reachable"], "listener up => reachable=1")
	assert.Equal(t, int64(1), got["queryable"], "query succeeds => queryable=1")
}

func TestScrapeMetricsUnreachableSkipsQueryable(t *testing.T) {
	// Obtain a guaranteed-closed port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	require.NoError(t, ln.Close())

	cfg := &Config{
		DataSource:           "server=127.0.0.1;port=" + portStr,
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
	}
	// The client would succeed if called; ScrapeMetrics must not call it when
	// the endpoint is unreachable.
	client := &sqlquery.FakeDBClient{StringMaps: [][]sqlquery.StringMap{{{"": "1"}}}}
	s := newTestHealthScraper(t, cfg, client)

	md, err := s.ScrapeMetrics(t.Context())
	require.NoError(t, err, "an unreachable database is data, not a scrape error")

	got := statusByCheck(t, md)
	assert.Equal(t, int64(0), got["reachable"], "closed port => reachable=0")
	assert.Equal(t, int64(0), got["queryable"], "unreachable => queryable=0 without probing")
	assert.Equal(t, 0, client.RequestCounter, "queryable check must be skipped when unreachable")
}

func TestScrapeMetricsReachableNotQueryable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	cfg := &Config{
		DataSource:           "server=127.0.0.1;port=" + portStr,
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
	}
	// Reachable, but the query fails (e.g. bad credentials).
	client := &sqlquery.FakeDBClient{Err: errors.New("login failed")}
	s := newTestHealthScraper(t, cfg, client)

	md, err := s.ScrapeMetrics(t.Context())
	require.NoError(t, err)

	got := statusByCheck(t, md)
	assert.Equal(t, int64(1), got["reachable"], "listener up => reachable=1")
	assert.Equal(t, int64(0), got["queryable"], "query fails => queryable=0")
}

func TestScrapeMetricsUnresolvableConfigEmitsNothing(t *testing.T) {
	// The config cannot be resolved to a host:port, so the reachability state is
	// unknown. ScrapeMetrics must emit no datapoints (a gap), not a false 0.
	cfg := &Config{
		DataSource:           "sqlserver://host:notaport",
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
	}
	client := &sqlquery.FakeDBClient{StringMaps: [][]sqlquery.StringMap{{{"": "1"}}}}
	s := newTestHealthScraper(t, cfg, client)

	md, err := s.ScrapeMetrics(t.Context())
	require.NoError(t, err, "an unknown state is not a scrape error")

	assert.Equal(t, 0, md.DataPointCount(), "unknown state must emit no datapoints")
	assert.Equal(t, 0, client.RequestCounter, "queryable check must be skipped when the target is unresolvable")
}

func TestSetupConnectionHealthScraper(t *testing.T) {
	params := receivertest.NewNopSettings(metadata.Type)

	t.Run("nil when no direct connection", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.isDirectDBConnectionEnabled = false
		assert.Nil(t, setupConnectionHealthScraper(params, cfg))
	})

	t.Run("nil when status metric disabled", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.isDirectDBConnectionEnabled = true
		cfg.MetricsBuilderConfig.Metrics.SqlserverHealth.Enabled = false
		assert.Nil(t, setupConnectionHealthScraper(params, cfg))
	})

	t.Run("scraper when enabled and direct", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.isDirectDBConnectionEnabled = true
		cfg.Server = "127.0.0.1"
		cfg.Username = "sa"
		cfg.Password = "pw"
		cfg.Port = 1433
		require.True(t, cfg.MetricsBuilderConfig.Metrics.SqlserverHealth.Enabled, "health metric should be on by default")
		assert.NotNil(t, setupConnectionHealthScraper(params, cfg))
	})
}

func TestConnectionHealthScraperID(t *testing.T) {
	cfg := &Config{DataSource: "server=127.0.0.1;port=1433"}
	id := component.NewIDWithName(metadata.Type, "connection-health")
	params := receivertest.NewNopSettings(metadata.Type)
	s := newConnectionHealthScraper(id, sqlquery.TelemetryConfig{}, nil, sqlquery.NewDbClient, params, cfg)
	assert.Equal(t, id, s.ID())
	assert.Equal(t, "connection-health", s.ID().Name())
}
