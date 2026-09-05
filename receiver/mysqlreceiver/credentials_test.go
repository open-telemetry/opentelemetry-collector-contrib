// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mysqlreceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/config/configdbauth"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth"
)

type staticProvider struct {
	username *string
	secret   string
}

func (p *staticProvider) GetCredential(context.Context, dbauth.Request) (*dbauth.Credential, error) {
	return &dbauth.Credential{Username: p.username, Secret: p.secret}, nil
}

func baseConfigWithProvider(p dbauth.Provider) mySQLConfig {
	return mySQLConfig{
		username:           "configured_user",
		password:           "static",
		address:            confignet.AddrConfig{Endpoint: "localhost:3306", Transport: confignet.TransportTypeTCP},
		credentialProvider: p,
	}
}

func TestConnectionString_ProviderSuppliesSecret(t *testing.T) {
	cfg := baseConfigWithProvider(&staticProvider{secret: "minted-token"})
	cs, err := cfg.ConnectionString(t.Context())
	require.NoError(t, err)

	assert.Contains(t, cs, "minted-token")
	assert.Contains(t, cs, "configured_user")
	assert.Contains(t, cs, "allowCleartextPasswords=true")
}

func TestConnectionString_NoProviderOmitsCleartextPasswords(t *testing.T) {
	cfg := mySQLConfig{
		username: "u",
		password: "static-pw",
		address:  confignet.AddrConfig{Endpoint: "localhost:3306", Transport: confignet.TransportTypeTCP},
	}
	cs, err := cfg.ConnectionString(t.Context())
	require.NoError(t, err)
	assert.NotContains(t, cs, "allowCleartextPasswords=true")
}

func TestConnectionString_ProviderOverridesUsername(t *testing.T) {
	dynUser := "vault-generated"
	cfg := baseConfigWithProvider(&staticProvider{username: &dynUser, secret: "pw"})
	cs, err := cfg.ConnectionString(t.Context())
	require.NoError(t, err)

	assert.Contains(t, cs, "vault-generated")
	assert.Contains(t, cs, "pw")
}

func TestConnectionString_NoProviderUsesStaticPassword(t *testing.T) {
	cfg := mySQLConfig{
		username: "u",
		password: "static-pw",
		address:  confignet.AddrConfig{Endpoint: "localhost:3306", Transport: confignet.TransportTypeTCP},
	}
	cs, err := cfg.ConnectionString(t.Context())
	require.NoError(t, err)
	assert.Contains(t, cs, "static-pw")
}

func TestConnectionString_PullRefreshOnRebuild(t *testing.T) {
	p := &staticProvider{secret: "token-v1"}
	cfg := baseConfigWithProvider(p)

	cs1, err := cfg.ConnectionString(t.Context())
	require.NoError(t, err)
	assert.Contains(t, cs1, "token-v1")

	p.secret = "token-v2"
	cs2, err := cfg.ConnectionString(t.Context())
	require.NoError(t, err)
	assert.Contains(t, cs2, "token-v2")
}

func TestConnectionString_SpecialCharactersInCredential(t *testing.T) {
	cfg := baseConfigWithProvider(&staticProvider{
		secret: "pa:ss@word",
	})
	cs, err := cfg.ConnectionString(t.Context())
	require.NoError(t, err)

	driverConf, err := mysql.ParseDSN(cs)
	require.NoError(t, err)
	assert.Equal(t, "pa:ss@word", driverConf.Passwd)
}

func TestConfigValidate_PasswordAndDBAuthMutuallyExclusive(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.AddrConfig.Endpoint = "localhost:3306"
	cfg.Username = "u"
	cfg.Password = "static"
	cfg.DBAuth = configdbauth.ID(component.MustNewID("aws_iam_db_auth"))
	cfg.TLS.Insecure = false

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), ErrPasswordAndDBAuth)
}

func TestConfigValidate_DBAuthWithoutPasswordIsValid(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.AddrConfig.Endpoint = "localhost:3306"
	cfg.Username = "u"
	cfg.DBAuth = configdbauth.ID(component.MustNewID("aws_iam_db_auth"))
	cfg.TLS.Insecure = false

	require.NoError(t, cfg.Validate())
}

func TestConfigValidate_NoPasswordWithoutDBAuthIsValid(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.AddrConfig.Endpoint = "localhost:3306"
	cfg.Username = "root"
	cfg.Password = ""

	require.NoError(t, cfg.Validate())
}

func TestConfigValidate_DBAuthRequiresTLS(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.AddrConfig.Endpoint = "localhost:3306"
	cfg.Username = "u"
	cfg.DBAuth = configdbauth.ID(component.MustNewID("aws_iam_db_auth"))
	cfg.TLS.Insecure = true

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), ErrDBAuthRequiresTLS)
}

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
		component.MustNewID("aws_iam_db_auth"): fakeCredExtension{secret: "fake-token"},
	}
}

func TestResolveCredentialProvider_ResolvesFromHostExtension(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.AddrConfig.Endpoint = "db.example.com:3306"
	cfg.Username = "monitor"
	cfg.DBAuth = configdbauth.ID(component.MustNewID("aws_iam_db_auth"))
	cfg.TLS.Insecure = false

	p, err := cfg.resolveCredentialProvider(credExtMap())
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestResolveCredentialProvider_NoMatchingExtension(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.AddrConfig.Endpoint = "db.example.com:3306"
	cfg.Username = "monitor"
	cfg.DBAuth = configdbauth.ID(component.MustNewID("aws_iam_db_auth"))
	cfg.TLS.Insecure = false

	_, err := cfg.resolveCredentialProvider(map[component.ID]component.Component{})
	require.Error(t, err)
}

func TestResolveCredentialProvider_NoAuthReturnsNil(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.AddrConfig.Endpoint = "localhost:3306"
	cfg.Username = "u"
	cfg.Password = "pw"

	p, err := cfg.resolveCredentialProvider(credExtMap())
	require.NoError(t, err)
	assert.Nil(t, p)
}

type countingProvider struct {
	calls int
}

func (p *countingProvider) GetCredential(context.Context, dbauth.Request) (*dbauth.Credential, error) {
	p.calls++
	return nil, errors.New("mint failed")
}

func TestCredentialConnector_ResolvesPerConnect(t *testing.T) {
	p := &countingProvider{}
	cfg := baseConfigWithProvider(p)
	conn := &credentialConnector{cfg: cfg}

	_, err1 := conn.Connect(t.Context())
	require.Error(t, err1)
	_, err2 := conn.Connect(t.Context())
	require.Error(t, err2)

	assert.Equal(t, 2, p.calls)
}

func TestCredentialConnector_PerConnectionRefresh(t *testing.T) {
	p := &staticProvider{secret: "token-v1"}
	cfg := baseConfigWithProvider(p)

	cs1, err := cfg.ConnectionString(t.Context())
	require.NoError(t, err)
	assert.Contains(t, cs1, "token-v1")

	p.secret = "token-v2"
	cs2, err := cfg.ConnectionString(t.Context())
	require.NoError(t, err)
	assert.Contains(t, cs2, "token-v2")
}

type nilCredentialProvider struct{}

func (nilCredentialProvider) GetCredential(context.Context, dbauth.Request) (*dbauth.Credential, error) {
	return nil, nil
}

func TestConnectionString_NilCredentialFailsClosed(t *testing.T) {
	cfg := baseConfigWithProvider(nilCredentialProvider{})
	_, err := cfg.ConnectionString(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil credential")
}

func TestConfigUnmarshal_DBAuthScalarID(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	conf := confmap.NewFromStringMap(map[string]any{
		"endpoint": "db.example.com:3306",
		"username": "monitor",
		"db_auth":  "aws_iam_db_auth",
		"tls": map[string]any{
			"insecure": false,
		},
	})
	require.NoError(t, conf.Unmarshal(cfg))
	require.NoError(t, cfg.Validate())

	require.Equal(t, component.MustNewID("aws_iam_db_auth"), cfg.DBAuth.ComponentID())

	provider, err := cfg.resolveCredentialProvider(credExtMap())
	require.NoError(t, err)
	require.NotNil(t, provider)
}

func TestConfigUnmarshal_DBAuthNamedInstance(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	conf := confmap.NewFromStringMap(map[string]any{
		"endpoint": "db.example.com:3306",
		"username": "monitor",
		"db_auth":  "aws_iam_db_auth/primary",
		"tls": map[string]any{
			"insecure": false,
		},
	})
	require.NoError(t, conf.Unmarshal(cfg))
	require.NoError(t, cfg.Validate())

	id := component.MustNewIDWithName("aws_iam_db_auth", "primary")
	require.Equal(t, id, cfg.DBAuth.ComponentID())

	provider, err := cfg.resolveCredentialProvider(map[component.ID]component.Component{
		id: fakeCredExtension{secret: "fake-token"},
	})
	require.NoError(t, err)
	require.NotNil(t, provider)
}

func TestNewClientFactory_AcceptsDBAuth(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.AddrConfig.Endpoint = "localhost:3306"
	cfg.Username = "u"
	cfg.DBAuth = configdbauth.ID(component.MustNewID("aws_iam_db_auth"))
	cfg.TLS.Insecure = false

	f, err := newClientFactory(cfg, component.MustNewID("mysql"))
	require.NoError(t, err)

	factory := f.(*defaultClientFactory)
	factory.setCredentialProvider(&staticProvider{secret: "minted-token"})
	sqlClient, err := factory.connect(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlClient.Close()) })
}
