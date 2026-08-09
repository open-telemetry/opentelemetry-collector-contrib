// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"database/sql"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

// newTestHealthScraper builds a connectionHealthScraper whose queryable probe
// resolves to the supplied fake DB client. The db provider returns a lazily
// opened *sql.DB (sql.Open does not dial until a query runs, and the fake client
// short-circuits the query, so the connection is never actually established); it
// is safe to Close. The client provider ignores its db argument and always hands
// back the injected fake, so RequestCounter reflects the probe's calls.
func newTestHealthScraper(t *testing.T, cfg *Config, client sqlquery.DbClient) *connectionHealthScraper {
	t.Helper()
	params := receivertest.NewNopSettings(metadata.Type)
	id := component.NewIDWithName(metadata.Type, "connection-health")
	dbProvider := func() (*sql.DB, error) {
		return sql.Open("sqlserver", "server=127.0.0.1")
	}
	clientProvider := func(sqlquery.Db, string, *zap.Logger, sqlquery.TelemetryConfig) sqlquery.DbClient {
		return client
	}
	return newConnectionHealthScraper(id, dbProvider, clientProvider, params, cfg)
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

// TestProbeQueryableOpensFreshConnectionEachScrape asserts the queryable probe
// opens a new connection (via dbProviderFunc) on every call rather than reusing
// one long-lived pooled connection. Reuse would let a stale but still-open
// session report queryable=1 after new logins have started failing.
func TestProbeQueryableOpensFreshConnectionEachScrape(t *testing.T) {
	cfg := &Config{DataSource: "server=127.0.0.1;port=1433"}
	params := receivertest.NewNopSettings(metadata.Type)
	id := component.NewIDWithName(metadata.Type, "connection-health")

	var opened int
	dbProvider := func() (*sql.DB, error) {
		opened++
		// sql.Open does not dial; the fake client below short-circuits the query,
		// so the connection is never actually established. Safe to Close.
		return sql.Open("sqlserver", "server=127.0.0.1")
	}
	client := &sqlquery.FakeDBClient{StringMaps: [][]sqlquery.StringMap{{{"": "1"}}, {{"": "1"}}}}
	clientProvider := func(sqlquery.Db, string, *zap.Logger, sqlquery.TelemetryConfig) sqlquery.DbClient {
		return client
	}
	s := newConnectionHealthScraper(id, dbProvider, clientProvider, params, cfg)

	require.True(t, s.probeQueryable(t.Context()))
	require.True(t, s.probeQueryable(t.Context()))

	assert.Equal(t, 2, opened, "each scrape must open its own connection")
	assert.Equal(t, 2, client.RequestCounter, "each scrape must run its own query")
}

// TestProbeQueryableOpenFailure asserts that when a connection cannot even be
// opened, the probe reports not-queryable rather than panicking.
func TestProbeQueryableOpenFailure(t *testing.T) {
	cfg := &Config{DataSource: "server=127.0.0.1;port=1433"}
	params := receivertest.NewNopSettings(metadata.Type)
	id := component.NewIDWithName(metadata.Type, "connection-health")

	dbProvider := func() (*sql.DB, error) {
		return nil, errors.New("cannot open connection")
	}
	clientProvider := func(sqlquery.Db, string, *zap.Logger, sqlquery.TelemetryConfig) sqlquery.DbClient {
		t.Fatal("client provider must not be called when the connection cannot be opened")
		return nil
	}
	s := newConnectionHealthScraper(id, dbProvider, clientProvider, params, cfg)

	assert.False(t, s.probeQueryable(t.Context()), "open failure => not queryable")
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
					check, ok := dp.Attributes().Get("sqlserver.health.check.type")
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
	s := newConnectionHealthScraper(id, nil, sqlquery.NewDbClient, params, cfg)
	assert.Equal(t, id, s.ID())
	assert.Equal(t, "connection-health", s.ID().Name())
}
