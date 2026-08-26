// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"sync"

	"github.com/lib/pq"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver/internal/metadata"
)

type postgreSQLClientFactory interface {
	getClient(ctx context.Context, database string) (client, error)
	// setCredentialProvider injects the credential provider resolved from the host
	// extension map at Start. A nil provider means no db_auth block (static
	// password).
	setCredentialProvider(provider dbauth.Provider)
	close() error
}

// newClientFactory selects the pool or default client factory based on the
// connection-pool feature gate. The credential provider (if any) is resolved from
// the host extension map later, at scraper Start, and injected via
// setCredentialProvider — the host is not available at receiver-create time.
func newClientFactory(cfg *Config) postgreSQLClientFactory {
	if metadata.ReceiverPostgresqlConnectionPoolFeatureGate.IsEnabled() {
		return newPoolClientFactory(cfg)
	}
	return newDefaultClientFactory(cfg)
}

// defaultClientFactory creates one PG connection per call
type defaultClientFactory struct {
	baseConfig postgreSQLConfig
}

func newDefaultClientFactory(cfg *Config) *defaultClientFactory {
	return &defaultClientFactory{
		baseConfig: postgreSQLConfig{
			username: cfg.Username,
			password: string(cfg.Password),
			address:  cfg.AddrConfig,
			tls:      cfg.ClientConfig,
		},
	}
}

func (d *defaultClientFactory) setCredentialProvider(provider dbauth.Provider) {
	d.baseConfig.credentialProvider = provider
}

func (d *defaultClientFactory) getClient(ctx context.Context, database string) (client, error) {
	db, err := getDB(ctx, d.baseConfig, database)
	if err != nil {
		return nil, err
	}
	return &postgreSQLClient{client: db, closeFn: db.Close}, nil
}

func (*defaultClientFactory) close() error {
	return nil
}

// poolClientFactory creates one PG connection per database, keeping a pool of connections
type poolClientFactory struct {
	sync.Mutex
	baseConfig postgreSQLConfig
	poolConfig *ConnectionPool
	pool       map[string]*sql.DB
	closed     bool
}

func newPoolClientFactory(cfg *Config) *poolClientFactory {
	poolCfg := cfg.ConnectionPool
	return &poolClientFactory{
		baseConfig: postgreSQLConfig{
			username: cfg.Username,
			password: string(cfg.Password),
			address:  cfg.AddrConfig,
			tls:      cfg.ClientConfig,
		},
		poolConfig: &poolCfg,
		pool:       make(map[string]*sql.DB),
		closed:     false,
	}
}

func (p *poolClientFactory) setCredentialProvider(provider dbauth.Provider) {
	p.baseConfig.credentialProvider = provider
}

func (p *poolClientFactory) getClient(ctx context.Context, database string) (client, error) {
	p.Lock()
	defer p.Unlock()
	db, ok := p.pool[database]
	if !ok {
		var err error
		db, err = getDB(ctx, p.baseConfig, database)
		if err != nil {
			return nil, err
		}
		p.setPoolSettings(db)
		p.pool[database] = db
	}
	return &postgreSQLClient{client: db, closeFn: nil}, nil
}

func (p *poolClientFactory) close() error {
	p.Lock()
	defer p.Unlock()

	if p.closed {
		return nil
	}

	if p.pool != nil {
		var err error
		for _, db := range p.pool {
			if closeErr := db.Close(); closeErr != nil {
				err = multierr.Append(err, closeErr)
			}
		}
		if err != nil {
			return err
		}
	}

	p.closed = true
	return nil
}

func (p *poolClientFactory) setPoolSettings(db *sql.DB) {
	if p.poolConfig == nil {
		return
	}
	if p.poolConfig.MaxIdleTime != nil {
		db.SetConnMaxIdleTime(*p.poolConfig.MaxIdleTime)
	}
	if p.poolConfig.MaxLifetime != nil {
		db.SetConnMaxLifetime(*p.poolConfig.MaxLifetime)
	}
	if p.poolConfig.MaxIdle != nil {
		db.SetMaxIdleConns(*p.poolConfig.MaxIdle)
	}
	if p.poolConfig.MaxOpen != nil {
		db.SetMaxOpenConns(*p.poolConfig.MaxOpen)
	}
}

func getDB(ctx context.Context, cfg postgreSQLConfig, database string) (*sql.DB, error) {
	if database != "" {
		cfg.database = database
	}
	if cfg.credentialProvider != nil {
		// A credential provider mints a short-lived secret (e.g. an AWS IAM token).
		// Resolve it per physical connection rather than baking one secret into the
		// DSN, so a long-lived pool re-mints on every new connection it opens and
		// never dials with an expired token. credentialConnector does this inside
		// driver.Connector.Connect, which database/sql calls per new connection.
		return sql.OpenDB(&credentialConnector{cfg: cfg}), nil
	}
	connectionString, err := cfg.ConnectionString(ctx)
	if err != nil {
		return nil, err
	}
	conn, err := pq.NewConnector(connectionString)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(conn), nil
}

// credentialConnector is a driver.Connector that rebuilds the lib/pq DSN — and so
// re-resolves the credential provider — every time database/sql opens a new
// physical connection. This is what lets a pooled *sql.DB pick up a refreshed
// credential: each new connection mints a current secret, while connections
// already established stay valid for their lifetime (AWS RDS IAM authenticates
// only at connection open, not per query).
type credentialConnector struct {
	cfg postgreSQLConfig
}

func (c *credentialConnector) Connect(ctx context.Context) (driver.Conn, error) {
	connectionString, err := c.cfg.ConnectionString(ctx)
	if err != nil {
		return nil, err
	}
	conn, err := pq.NewConnector(connectionString)
	if err != nil {
		return nil, err
	}
	return conn.Connect(ctx)
}

func (*credentialConnector) Driver() driver.Driver { return &pq.Driver{} }
