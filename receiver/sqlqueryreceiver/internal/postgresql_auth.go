// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver/internal"

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/lib/pq"
	"go.opentelemetry.io/collector/config/configopaque"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
)

// CredentialProviderFunc returns the provider resolved from the Collector host.
type CredentialProviderFunc func() dbauth.Provider

// NewPostgreSQLCredentialDB creates a SQL pool whose PostgreSQL credentials are
// requested from the configured provider every time a physical connection opens.
func NewPostgreSQLCredentialDB(cfg sqlquery.Config, maxOpenConns int, getProvider CredentialProviderFunc) *sql.DB {
	db := sql.OpenDB(&postgresCredentialConnector{config: cfg, getProvider: getProvider})
	db.SetMaxOpenConns(maxOpenConns)
	return db
}

// NewPostgreSQLCredentialPool returns a lazy, shared PostgreSQL database pool.
func NewPostgreSQLCredentialPool(cfg sqlquery.Config, maxOpenConns int, getProvider CredentialProviderFunc) interface {
	DB() (*sql.DB, error)
} {
	return &postgresCredentialPool{
		config:       cfg,
		maxOpenConns: maxOpenConns,
		getProvider:  getProvider,
	}
}

type postgresCredentialPool struct {
	mu           sync.Mutex
	config       sqlquery.Config
	maxOpenConns int
	getProvider  CredentialProviderFunc
	db           *sql.DB
}

func (p *postgresCredentialPool) DB() (*sql.DB, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.db == nil {
		p.db = NewPostgreSQLCredentialDB(p.config, p.maxOpenConns, p.getProvider)
	}
	return p.db, nil
}

type postgresCredentialConnector struct {
	config      sqlquery.Config
	getProvider CredentialProviderFunc
}

func (c *postgresCredentialConnector) Connect(ctx context.Context) (driver.Conn, error) {
	dataSource, err := postgresDataSource(ctx, c.config, c.getProvider)
	if err != nil {
		return nil, err
	}
	connector, err := pq.NewConnector(dataSource)
	if err != nil {
		return nil, err
	}
	return connector.Connect(ctx)
}

func postgresDataSource(ctx context.Context, config sqlquery.Config, getProvider CredentialProviderFunc) (string, error) {
	provider := getProvider()
	if provider == nil {
		return "", errors.New("db_auth provider is not resolved")
	}

	endpoint := net.JoinHostPort(config.Host, strconv.Itoa(config.Port))
	credential, err := provider.GetCredential(ctx, dbauth.Request{
		Endpoint: endpoint,
		Username: config.Username,
	})
	if err != nil {
		return "", fmt.Errorf("resolve PostgreSQL credential: %w", err)
	}
	if credential == nil {
		return "", errors.New("resolve PostgreSQL credential: provider returned a nil credential")
	}

	config.Password = configopaque.String(credential.Secret)
	if credential.Username != nil {
		config.Username = *credential.Username
	}
	dataSource, err := sqlquery.BuildDataSourceString(config)
	if err != nil {
		return "", err
	}
	return dataSource, nil
}

func (*postgresCredentialConnector) Driver() driver.Driver {
	return &pq.Driver{}
}
