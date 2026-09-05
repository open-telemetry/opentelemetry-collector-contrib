// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"net/http"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/extension/extensionauth"
)

func TestApplyAuth_NoAuth(t *testing.T) {
	cfg := withDefaultConfig(func(c *Config) {
		c.Endpoint = "https://127.0.0.1:8443"
	})

	opt, err := cfg.buildClickHouseOptions()
	require.NoError(t, err)

	require.NoError(t, cfg.applyAuth(t.Context(), opt, nil))
	assert.Nil(t, opt.GetJWT)
	assert.Nil(t, opt.TransportFunc)
}

func TestApplyAuth_HTTPUsesTransportFunc(t *testing.T) {
	authID := component.MustNewID("authtest")
	cfg := withDefaultConfig(func(c *Config) {
		c.Endpoint = "https://127.0.0.1:8443"
		c.Auth = configoptional.Some(configauth.Config{AuthenticatorID: authID})
	})

	opt, err := cfg.buildClickHouseOptions()
	require.NoError(t, err)
	require.Equal(t, clickhouse.HTTP, opt.Protocol)
	require.NotNil(t, opt.TLS)

	require.NoError(t, cfg.applyAuth(t.Context(), opt, newAuthHost(authID, "test-jwt")))
	assert.Nil(t, opt.GetJWT)
	require.NotNil(t, opt.TransportFunc)

	rt, err := opt.TransportFunc(&http.Transport{})
	require.NoError(t, err)
	require.NotNil(t, rt)
}

func TestApplyAuth_NativeUsesGetJWT(t *testing.T) {
	authID := component.MustNewID("authtest")
	cfg := withDefaultConfig(func(c *Config) {
		c.Endpoint = "clickhouse://127.0.0.1:9440?secure=true"
		c.Auth = configoptional.Some(configauth.Config{AuthenticatorID: authID})
	})

	opt, err := cfg.buildClickHouseOptions()
	require.NoError(t, err)
	require.Equal(t, clickhouse.Native, opt.Protocol)
	require.NotNil(t, opt.TLS)

	require.NoError(t, cfg.applyAuth(t.Context(), opt, newAuthHost(authID, "native-jwt")))
	assert.Nil(t, opt.TransportFunc)
	require.NotNil(t, opt.GetJWT)

	token, err := opt.GetJWT(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "native-jwt", token)
}

func TestApplyAuth_RequiresTLS(t *testing.T) {
	authID := component.MustNewID("authtest")
	cfg := withDefaultConfig(func(c *Config) {
		c.Endpoint = "tcp://127.0.0.1:9000"
		c.Auth = configoptional.Some(configauth.Config{AuthenticatorID: authID})
	})

	opt, err := cfg.buildClickHouseOptions()
	require.NoError(t, err)
	require.Nil(t, opt.TLS)

	err = cfg.applyAuth(t.Context(), opt, newAuthHost(authID, "unused"))
	require.ErrorIs(t, err, errAuthRequiresTLS)
}

func TestApplyAuth_RequiresHost(t *testing.T) {
	cfg := withDefaultConfig(func(c *Config) {
		c.Endpoint = "https://127.0.0.1:8443"
		c.Auth = configoptional.Some(configauth.Config{AuthenticatorID: component.MustNewID("authtest")})
	})

	opt, err := cfg.buildClickHouseOptions()
	require.NoError(t, err)

	err = cfg.applyAuth(t.Context(), opt, nil)
	require.ErrorIs(t, err, errAuthRequiresHost)
}

func TestApplyAuth_MissingExtension(t *testing.T) {
	authID := component.MustNewID("authtest")
	cfg := withDefaultConfig(func(c *Config) {
		c.Endpoint = "https://127.0.0.1:8443"
		c.Auth = configoptional.Some(configauth.Config{AuthenticatorID: authID})
	})

	opt, err := cfg.buildClickHouseOptions()
	require.NoError(t, err)

	err = cfg.applyAuth(t.Context(), opt, &authHost{extensions: map[component.ID]component.Component{}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve auth extension")
}

func TestTokenFromHTTPAuth(t *testing.T) {
	token, err := tokenFromHTTPAuth(t.Context(), &mockAuthClient{token: "from-http"})
	require.NoError(t, err)
	assert.Equal(t, "from-http", token)
}

func TestJwtFromAuthorization(t *testing.T) {
	token, err := jwtFromAuthorization("Bearer abc.def.ghi")
	require.NoError(t, err)
	assert.Equal(t, "abc.def.ghi", token)

	token, err = jwtFromAuthorization("bearer xyz")
	require.NoError(t, err)
	assert.Equal(t, "xyz", token)

	_, err = jwtFromAuthorization("")
	require.Error(t, err)

	_, err = jwtFromAuthorization("Bearer ")
	require.Error(t, err)
}

type authHost struct {
	extensions map[component.ID]component.Component
}

func (h *authHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

func newAuthHost(id component.ID, token string) *authHost {
	return &authHost{
		extensions: map[component.ID]component.Component{
			id: &mockAuthClient{token: token},
		},
	}
}

var _ extensionauth.HTTPClient = (*mockAuthClient)(nil)

type mockAuthClient struct {
	component.StartFunc
	component.ShutdownFunc
	token string
}

func (m *mockAuthClient) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return &bearerRoundTripper{base: base, token: m.token}, nil
}

type bearerRoundTripper struct {
	base  http.RoundTripper
	token string
}

func (b *bearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(req2)
}
