// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"

import (
	"database/sql"
	"sync"

	"github.com/lib/pq"
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/multierr"
)

const connectionPoolGateID = "receiver.postgresql.connectionPool"

var connectionPoolGate = featuregate.GlobalRegistry().MustRegister(
	connectionPoolGateID,
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("Use of connection pooling"),
	featuregate.WithRegisterFromVersion("0.96.0"),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/30831"),
)

type postgreSQLClientFactory interface {
	getClient(database string) (client, error)
	close() error
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

func (d *defaultClientFactory) getClient(database string) (client, error) {
	db, err := getDB(d.baseConfig, database)
	if err != nil {
		return nil, err
	}
	return &postgreSQLClient{client: db, closeFn: db.Close}, nil
}

func (d *defaultClientFactory) close() error {
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

func (p *poolClientFactory) getClient(database string) (client, error) {
	p.Lock()
	defer p.Unlock()
	db, ok := p.pool[database]
	if !ok {
		var err error
		db, err = getDB(p.baseConfig, database)
		p.setPoolSettings(db)
		if err != nil {
			return nil, err
		}
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

func getDB(cfg postgreSQLConfig, database string) (*sql.DB, error) {
	if database != "" {
		cfg.database = database
	}
	connectionString, err := cfg.ConnectionString()
	if err != nil {
		return nil, err
	}
	conn, err := pq.NewConnector(connectionString)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(conn), nil
}
