// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"context"
	"net"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

// dialTimeout bounds the transport-level reachability probe (parse DSN -> dial).
// It is deliberately independent of the query timeout so a firewalled endpoint
// (dropped SYN) cannot hang the scrape past this budget.
const dialTimeout = 5 * time.Second

// queryTimeout bounds the queryable probe (connect -> authenticate -> SELECT 1).
// Without it a server that accepts the TCP connection but stalls during the
// login handshake or query (an overloaded or half-broken instance) could hang
// the scrape indefinitely. It is independent of dialTimeout so the two phases
// are bounded separately.
const queryTimeout = 5 * time.Second

// queryableProbeQuery is the trivial statement used for the queryable check.
// It forces the driver to establish a session, authenticate, and execute a
// statement end-to-end without touching any DMV or user object.
const queryableProbeQuery = "SELECT 1"

// connectionHealthScraper reports the connection health of the SQL Server target
// as the sqlserver.health metric, keyed by the check attribute:
//
//   - reachable: a transport-level (pre-auth) check that the endpoint accepts a
//     TCP connection, performed by parsing the connection string with the driver's
//     own parser and dialing the resolved host:port outside the connection pool.
//   - queryable: an end-to-end check that the receiver can authenticate and run a
//     trivial query (SELECT 1). A fresh connection is opened and closed for every
//     scrape rather than reusing a pooled one, so the probe tests whether a brand
//     new authenticated session can be established right now. Reusing a pooled
//     connection would report success as long as a previously-opened session
//     survived, masking failures that only affect new sessions (rotated
//     credentials, login-limit exhaustion, an auth path broken for new logins).
//
// Both checks are emitted every collection interval. ScrapeMetrics never returns
// an error for a failed probe: an unreachable or unauthenticated database is
// reported as data (a 0 datapoint), not as an absence of data.
type connectionHealthScraper struct {
	id                 component.ID
	config             *Config
	logger             *zap.Logger
	dbProviderFunc     sqlquery.DbProviderFunc
	clientProviderFunc sqlquery.ClientProviderFunc
	mb                 *metadata.MetricsBuilder
	serviceInstanceID  string
}

var _ scraper.Metrics = (*connectionHealthScraper)(nil)

func newConnectionHealthScraper(
	id component.ID,
	dbProvider sqlquery.DbProviderFunc,
	clientProvider sqlquery.ClientProviderFunc,
	params receiver.Settings,
	cfg *Config,
) *connectionHealthScraper {
	serviceInstanceID, err := computeServiceInstanceID(cfg)
	if err != nil {
		params.Logger.Warn("Failed to compute service.instance.id", zap.Error(err))
		serviceInstanceID = defaultServiceInstanceID
	}

	return &connectionHealthScraper{
		id:                 id,
		config:             cfg,
		logger:             params.Logger,
		dbProviderFunc:     dbProvider,
		clientProviderFunc: clientProvider,
		mb:                 metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, params),
		serviceInstanceID:  serviceInstanceID,
	}
}

func (s *connectionHealthScraper) ID() component.ID {
	return s.id
}

// Start is a no-op: unlike the metric/log scrapers, the health scraper holds no
// long-lived connection. Each scrape's queryable probe opens and closes its own
// connection so that it always tests a fresh authenticated session (see
// probeQueryable).
func (*connectionHealthScraper) Start(context.Context, component.Host) error {
	return nil
}

func (s *connectionHealthScraper) ScrapeMetrics(ctx context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())

	// Resolve the dial target first. If the config cannot be resolved to a
	// host:port, we could not probe anything: the state is unknown, not down. We
	// emit no datapoints this scrape (a gap) rather than a false 0 for an endpoint
	// we never contacted. Do not log the parse error verbatim: it can echo the
	// DSN, which may contain credentials.
	host, port, err := resolveConfiguredHostPort(s.config)
	if err != nil {
		s.logger.Warn("Connection health: failed to parse connection string for reachability check; skipping datapoint")
		return pmetric.NewMetrics(), nil
	}

	// reachable is a transport-level pre-auth check. queryable requires
	// reachable, so if reachable is 0 we report queryable 0 without opening a
	// session. A failed probe is not a scrape error: a database that is down is
	// data (a 0 datapoint), not an absence of data.
	reachable := s.probeReachable(ctx, host, port)
	s.mb.RecordSqlserverHealthDataPoint(now, boolToStatus(reachable), metadata.AttributeCheckReachable)

	queryable := false
	if reachable {
		queryable = s.probeQueryable(ctx)
	}
	s.mb.RecordSqlserverHealthDataPoint(now, boolToStatus(queryable), metadata.AttributeCheckQueryable)

	rb := s.mb.NewResourceBuilder()
	s.setupResourceBuilder(rb)

	return s.mb.Emit(metadata.WithResource(rb.Emit())), nil
}

// probeReachable dials the resolved host:port, pre-auth and outside the pool. It
// returns true if the endpoint accepts a TCP connection within dialTimeout. The
// caller resolves host:port (see resolveConfiguredHostPort) so that an
// unresolvable config is handled as an unknown state before any probe runs.
func (s *connectionHealthScraper) probeReachable(ctx context.Context, host string, port int) bool {
	address := net.JoinHostPort(host, strconv.Itoa(port))

	var d net.Dialer
	dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	conn, err := d.DialContext(dialCtx, "tcp", address)
	if err != nil {
		s.logger.Debug("Connection health: reachability check failed",
			zap.String("address", address), zap.Error(err))
		return false
	}
	_ = conn.Close()
	return true
}

// probeQueryable opens a fresh connection, runs a trivial query (SELECT 1), and
// closes the connection. It returns true if the receiver can authenticate and
// execute the statement end-to-end. A new connection is opened for every scrape
// rather than reusing a pooled one: reuse would report success as long as a
// previously-established session survived, so it would not catch failures that
// only affect new logins (rotated credentials, login-limit exhaustion, an auth
// path that is broken for new sessions but fine for an existing one). Opening and
// closing per scrape makes queryable a true test of "can I connect right now".
func (s *connectionHealthScraper) probeQueryable(ctx context.Context) bool {
	db, err := s.dbProviderFunc()
	if err != nil {
		s.logger.Debug("Connection health: queryable check could not open a connection", zap.Error(err))
		return false
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			s.logger.Debug("Connection health: failed to close queryable-probe connection", zap.Error(closeErr))
		}
	}()

	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	client := s.clientProviderFunc(sqlquery.DbWrapper{Db: db}, queryableProbeQuery, s.logger, sqlquery.TelemetryConfig{})
	if _, err := client.QueryRows(queryCtx); err != nil {
		s.logger.Debug("Connection health: queryable check failed", zap.Error(err))
		return false
	}
	return true
}

func (s *connectionHealthScraper) setupResourceBuilder(rb *metadata.ResourceBuilder) {
	rb.SetSqlserverComputerName(s.config.ComputerName)
	rb.SetSqlserverInstanceName(s.config.InstanceName)

	hostName, port, err := resolveResourceHostPort(s.config)
	if err != nil {
		s.logger.Warn("Failed to resolve host/port for resource attributes, using fallback", zap.Error(err))
	}

	rb.SetHostName(hostName)
	rb.SetServiceInstanceID(s.serviceInstanceID)
	rb.SetServiceName(defaultServiceName)
	rb.SetServiceNamespace("")

	// SetServerAddress / SetServerPort are already gated on the server.address /
	// server.port resource-attribute config (disabled by default), so they are
	// called unconditionally here. The receiver.sqlserver.RemoveServerResourceAttribute
	// feature gate is being removed (see #49886); not referencing it keeps this
	// mergeable once that gate is deleted.
	rb.SetServerAddress(hostName)
	rb.SetServerPort(int64(port))
}

// Shutdown is a no-op: the queryable probe opens and closes its own connection
// each scrape, so there is no long-lived connection to release here.
func (*connectionHealthScraper) Shutdown(context.Context) error {
	return nil
}

func boolToStatus(ok bool) int64 {
	if ok {
		return 1
	}
	return 0
}
