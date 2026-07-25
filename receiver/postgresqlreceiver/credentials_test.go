// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/config/configdbauth"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth"
)

// staticProvider is a test credentials provider that returns a fixed credential.
// It re-reads from pointers so a test can mutate the returned secret between calls
// to simulate rotation.
type staticProvider struct {
	username *string
	secret   string
}

func (p *staticProvider) GetCredential(context.Context, dbauth.Request) (*dbauth.Credential, error) {
	return &dbauth.Credential{Username: p.username, Secret: p.secret}, nil
}

func baseConfigWithProvider(p dbauth.Provider) postgreSQLConfig {
	return postgreSQLConfig{
		username:           "configured_user",
		address:            confignet.AddrConfig{Endpoint: "localhost:5432", Transport: confignet.TransportTypeTCP},
		credentialProvider: p,
	}
}

func TestConnectionString_ProviderSuppliesSecret(t *testing.T) {
	cfg := baseConfigWithProvider(&staticProvider{secret: "minted-token"})
	cs, err := cfg.ConnectionString(t.Context())
	require.NoError(t, err)

	assert.Contains(t, cs, "password='minted-token'", "provider secret goes into the password slot")
	assert.Contains(t, cs, "user='configured_user'", "nil provider username falls back to the configured username")
}

func TestConnectionString_ProviderOverridesUsername(t *testing.T) {
	dynUser := "v-vault-generated"
	cfg := baseConfigWithProvider(&staticProvider{username: &dynUser, secret: "pw"})
	cs, err := cfg.ConnectionString(t.Context())
	require.NoError(t, err)

	assert.Contains(t, cs, "user='v-vault-generated'", "non-nil provider username overrides the configured one")
	assert.Contains(t, cs, "password='pw'")
}

func TestConnectionString_NoProviderUsesStaticPassword(t *testing.T) {
	cfg := postgreSQLConfig{
		username: "u",
		password: "static-pw",
		address:  confignet.AddrConfig{Endpoint: "localhost:5432", Transport: confignet.TransportTypeTCP},
	}
	cs, err := cfg.ConnectionString(t.Context())
	require.NoError(t, err)
	assert.Contains(t, cs, "password='static-pw'")
}

func TestConnectionString_PullRefreshOnRebuild(t *testing.T) {
	p := &staticProvider{secret: "token-v1"}
	cfg := baseConfigWithProvider(p)

	cs1, err := cfg.ConnectionString(t.Context())
	require.NoError(t, err)
	assert.Contains(t, cs1, "password='token-v1'")

	// Simulate the provider rotating its token; a fresh ConnectionString (as built
	// per *sql.DB by getDB) must pick up the new value without any restart.
	p.secret = "token-v2"
	cs2, err := cfg.ConnectionString(t.Context())
	require.NoError(t, err)
	assert.Contains(t, cs2, "password='token-v2'")
}

func TestConnectionString_QuotesConninfoValues(t *testing.T) {
	dynUser := `generated user\role`
	cfg := baseConfigWithProvider(&staticProvider{
		username: &dynUser,
		secret:   `pa ss\word' host=attacker`,
	})
	cfg.database = `postgres db\primary`
	cfg.tls.CAFile = `ca dir\root's.pem`

	cs, err := cfg.ConnectionString(t.Context())
	require.NoError(t, err)
	assert.Contains(t, cs, `port='5432'`)
	assert.Contains(t, cs, `host='localhost'`)
	assert.Contains(t, cs, `user='generated user\\role'`)
	assert.Contains(t, cs, `password='pa ss\\word\' host=attacker'`)
	assert.Contains(t, cs, `dbname='postgres db\\primary'`)
	assert.Contains(t, cs, `sslrootcert='ca dir\\root\'s.pem'`)

	_, err = pq.NewConnector(cs)
	require.NoError(t, err, "the escaped credentials must remain valid lib/pq conninfo")
}

func TestConfigValidate_PasswordAndDBAuthMutuallyExclusive(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:5432"
	cfg.Username = "u"
	cfg.Password = "static"
	cfg.DBAuth = configdbauth.ID(component.MustNewID("aws_iam_dbauth"))

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), ErrPasswordAndDBAuth)
}

func TestConfigValidate_DBAuthWithoutPasswordIsValid(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:5432"
	cfg.Username = "u"
	cfg.DBAuth = configdbauth.ID(component.MustNewID("aws_iam_dbauth"))

	require.NoError(t, cfg.Validate(), "a db_auth block satisfies the credential requirement without a password")
}

// fakeCredExtension is a minimal credentials-provider extension for tests: it
// lives in a host extension map and implements dbauth.Provider directly, without
// importing the real aws_iam_dbauth package.
type fakeCredExtension struct {
	secret string
}

func (fakeCredExtension) Start(context.Context, component.Host) error { return nil }
func (fakeCredExtension) Shutdown(context.Context) error              { return nil }

func (f fakeCredExtension) GetCredential(context.Context, dbauth.Request) (*dbauth.Credential, error) {
	return &dbauth.Credential{Secret: f.secret}, nil
}

func credExtMap() map[component.ID]component.Component {
	return map[component.ID]component.Component{
		component.MustNewID("aws_iam_dbauth"): fakeCredExtension{secret: "fake-token"},
	}
}

func TestResolveCredentialProvider_ResolvesFromHostExtension(t *testing.T) {
	// The configured provider ID matches a declared extension in the host map, and
	// that extension implements dbauth.Provider, so it resolves.
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "db.example.com:5432"
	cfg.Username = "monitor"
	cfg.DBAuth = configdbauth.ID(component.MustNewID("aws_iam_dbauth"))

	p, err := cfg.resolveCredentialProvider(credExtMap())
	require.NoError(t, err)
	require.NotNil(t, p, "a matching declared provider extension resolves")
}

func TestResolveCredentialProvider_NoMatchingExtension(t *testing.T) {
	// The configured provider ID names an extension that is not declared in the host map.
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "db.example.com:5432"
	cfg.Username = "monitor"
	cfg.DBAuth = configdbauth.ID(component.MustNewID("aws_iam_dbauth"))

	_, err := cfg.resolveCredentialProvider(map[component.ID]component.Component{})
	require.Error(t, err, "no declared extension matches the provider ID")
}

func TestResolveCredentialProvider_NoAuthReturnsNil(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:5432"
	cfg.Username = "u"
	cfg.Password = "pw"

	p, err := cfg.resolveCredentialProvider(credExtMap())
	require.NoError(t, err)
	assert.Nil(t, p, "no db_auth block means no provider; the static password is used")
}

func TestNewPoolClientFactory_AcceptsDBAuth(t *testing.T) {
	// The connection pool now composes with a db_auth block: the pool re-mints
	// per physical connection via credentialConnector, so an expiring token no
	// longer goes stale. The pool accepts an injected provider and still caches one
	// *sql.DB per database.
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:5432"
	cfg.Username = "u"
	cfg.DBAuth = configdbauth.ID(component.MustNewID("aws_iam_dbauth"))

	f := newPoolClientFactory(cfg)
	t.Cleanup(func() { require.NoError(t, f.close()) }) // close pooled *sql.DBs so goleak stays clean

	// With a provider injected, getClient builds a *sql.DB backed by the
	// credential-resolving connector (sql.OpenDB is lazy, so no real dial here) and
	// caches one per database.
	f.setCredentialProvider(&staticProvider{secret: "minted-token"})
	c1, err := f.getClient(t.Context(), "db1")
	require.NoError(t, err)
	require.NotNil(t, c1)
	c2, err := f.getClient(t.Context(), "db1")
	require.NoError(t, err)
	assert.Same(t, c1.(*postgreSQLClient).client, c2.(*postgreSQLClient).client, "the pool caches one *sql.DB per database")
}

// countingProvider counts GetCredential calls and always errors, so the
// credentialConnector short-circuits before dialing — letting a test assert how
// many times the credential was resolved without a live database.
type countingProvider struct {
	calls int
}

func (p *countingProvider) GetCredential(context.Context, dbauth.Request) (*dbauth.Credential, error) {
	p.calls++
	return nil, errors.New("mint failed")
}

func TestCredentialConnector_ResolvesPerConnect(t *testing.T) {
	// Each database/sql connection-open calls Connect, which must re-resolve the
	// credential — that is what keeps a long-lived pool from dialing with a stale
	// token. A counting provider proves one resolution per Connect.
	p := &countingProvider{}
	cfg := baseConfigWithProvider(p)
	conn := &credentialConnector{cfg: cfg}

	_, err1 := conn.Connect(t.Context())
	require.Error(t, err1, "the provider errors, surfaced before any dial")
	_, err2 := conn.Connect(t.Context())
	require.Error(t, err2)

	assert.Equal(t, 2, p.calls, "the credential is resolved once per Connect, not once per pool")
}

func TestCredentialConnector_PerConnectionRefresh(t *testing.T) {
	// A rotated secret must reach the next connection's DSN without rebuilding the
	// pool — the per-connect resolution in ConnectionString(ctx) is what delivers it.
	p := &staticProvider{secret: "token-v1"}
	cfg := baseConfigWithProvider(p)

	cs1, err := cfg.ConnectionString(t.Context())
	require.NoError(t, err)
	assert.Contains(t, cs1, "password='token-v1'")

	p.secret = "token-v2"
	cs2, err := cfg.ConnectionString(t.Context())
	require.NoError(t, err)
	assert.Contains(t, cs2, "password='token-v2'", "the next connection picks up the rotated secret")
}

// nilCredentialProvider violates the dbauth.Provider contract by returning
// neither a credential nor an error, so a test can assert connectionString fails
// closed instead of dereferencing the nil credential.
type nilCredentialProvider struct{}

func (nilCredentialProvider) GetCredential(context.Context, dbauth.Request) (*dbauth.Credential, error) {
	return nil, nil
}

func TestConnectionString_NilCredentialFailsClosed(t *testing.T) {
	// A contract-violating provider that returns (nil, nil) must not panic the
	// collector on the credential dereference; ConnectionString returns an error so
	// only this scrape fails.
	cfg := baseConfigWithProvider(nilCredentialProvider{})
	_, err := cfg.ConnectionString(t.Context())
	require.Error(t, err, "a nil credential from the provider is surfaced as an error, not a panic")
	assert.Contains(t, err.Error(), "nil credential")
}

func TestConfigUnmarshal_DBAuthScalarID(t *testing.T) {
	// The receiver config's db_auth block is a bare component-ID reference: the
	// scalar value names the provider extension. Confirm confmap decodes that scalar
	// into configdbauth.ID via its UnmarshalText hook (the same path a bare
	// component.ID field uses) and that resolveCredentialProvider resolves it.
	cfg := createDefaultConfig().(*Config)
	conf := confmap.NewFromStringMap(map[string]any{
		"endpoint": "db.example.com:5432",
		"username": "monitor",
		"db_auth":  "aws_iam_dbauth",
	})
	require.NoError(t, conf.Unmarshal(cfg))
	require.NoError(t, cfg.Validate())

	require.Equal(t, component.MustNewID("aws_iam_dbauth"), cfg.DBAuth.ComponentID(),
		"the scalar db_auth value decodes into the provider component ID")

	provider, err := cfg.resolveCredentialProvider(credExtMap())
	require.NoError(t, err)
	require.NotNil(t, provider, "the referenced extension resolves from the host map")
}

func TestConfigUnmarshal_DBAuthNamedInstance(t *testing.T) {
	// A named instance (aws_iam_dbauth/primary) decodes the same way and resolves against a
	// host map keyed by the full named ID.
	cfg := createDefaultConfig().(*Config)
	conf := confmap.NewFromStringMap(map[string]any{
		"endpoint": "db.example.com:5432",
		"username": "monitor",
		"db_auth":  "aws_iam_dbauth/primary",
	})
	require.NoError(t, conf.Unmarshal(cfg))
	require.NoError(t, cfg.Validate())

	id := component.MustNewIDWithName("aws_iam_dbauth", "primary")
	require.Equal(t, id, cfg.DBAuth.ComponentID())

	provider, err := cfg.resolveCredentialProvider(map[component.ID]component.Component{
		id: fakeCredExtension{secret: "fake-token"},
	})
	require.NoError(t, err)
	require.NotNil(t, provider)
}
