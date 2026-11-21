// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
)

type staticCredentialProvider struct {
	credential *dbauth.Credential
	err        error
	request    dbauth.Request
}

func (p *staticCredentialProvider) GetCredential(_ context.Context, request dbauth.Request) (*dbauth.Credential, error) {
	p.request = request
	return p.credential, p.err
}

func TestPostgresDataSourceUsesFreshProviderCredential(t *testing.T) {
	provider := &staticCredentialProvider{
		credential: &dbauth.Credential{Secret: "short-lived-token"},
	}
	cfg := sqlquery.Config{
		Driver:   sqlquery.DriverPostgres,
		Host:     "pg.example.postgres.database.azure.com",
		Port:     5432,
		Database: "metrics",
		Username: "collector@example.com",
	}

	dsn, err := postgresDataSource(t.Context(), cfg, func() dbauth.Provider { return provider })
	require.NoError(t, err)
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	password, hasPassword := parsed.User.Password()
	require.True(t, hasPassword)
	require.Equal(t, "short-lived-token", password)
	require.Equal(t, "collector@example.com", parsed.User.Username())
	require.Equal(t, "pg.example.postgres.database.azure.com:5432", provider.request.Endpoint)
	require.Equal(t, cfg.Username, provider.request.Username)
}

func TestPostgresDataSourceUsesProviderUsername(t *testing.T) {
	providerUsername := "generated@example.com"
	provider := &staticCredentialProvider{
		credential: &dbauth.Credential{Username: &providerUsername, Secret: "token"},
	}
	cfg := sqlquery.Config{
		Driver:   sqlquery.DriverPostgres,
		Host:     "localhost",
		Port:     5432,
		Database: "metrics",
		Username: "configured-user",
	}

	dsn, err := postgresDataSource(t.Context(), cfg, func() dbauth.Provider { return provider })
	require.NoError(t, err)
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	require.Equal(t, providerUsername, parsed.User.Username())
}

func TestPostgresDataSourceReturnsProviderErrors(t *testing.T) {
	provider := &staticCredentialProvider{err: errors.New("token acquisition failed")}
	_, err := postgresDataSource(t.Context(), sqlquery.Config{
		Driver: sqlquery.DriverPostgres, Host: "localhost", Port: 5432,
		Database: "metrics", Username: "collector",
	}, func() dbauth.Provider { return provider })
	require.ErrorContains(t, err, "token acquisition failed")
}
