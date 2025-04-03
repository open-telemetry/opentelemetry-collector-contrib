// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bearertokenauthextension

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.uber.org/zap/zaptest"
)

func TestPerRPCAuth(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.BearerToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."

	// test meta data is properly
	bauth := newBearerTokenAuth(cfg, nil)
	assert.NotNil(t, bauth)
	perRPCAuth := &PerRPCAuth{auth: bauth}
	md, err := perRPCAuth.GetRequestMetadata(context.Background())
	assert.NoError(t, err)
	expectedMetadata := map[string]string{
		"authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
	}
	assert.Equal(t, expectedMetadata, md)

	// always true
	ok := perRPCAuth.RequireTransportSecurity()
	assert.True(t, ok)
}

type mockRoundTripper struct{}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := &http.Response{StatusCode: http.StatusOK, Header: map[string][]string{}}
	for k, v := range req.Header {
		resp.Header.Set(k, v[0])
	}
	return resp, nil
}

func TestBearerAuthenticatorHttp(t *testing.T) {
	scheme := "TestScheme"
	cfg := createDefaultConfig().(*Config)
	cfg.BearerToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
	cfg.Scheme = scheme

	bauth := newBearerTokenAuth(cfg, nil)
	assert.NotNil(t, bauth)

	base := &mockRoundTripper{}
	c, err := bauth.RoundTripper(base)
	assert.NoError(t, err)
	assert.NotNil(t, c)

	request := &http.Request{Method: http.MethodGet}
	resp, err := c.RoundTrip(request)
	assert.NoError(t, err)
	authHeaderValue := resp.Header.Get("Authorization")
	assert.Equal(t, authHeaderValue, fmt.Sprintf("%s %s", scheme, string(cfg.BearerToken)))
}

func TestBearerAuthenticator(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.BearerToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."

	bauth := newBearerTokenAuth(cfg, nil)
	assert.NotNil(t, bauth)

	assert.NoError(t, bauth.Start(context.Background(), componenttest.NewNopHost()))
	credential, err := bauth.PerRPCCredentials()

	assert.NoError(t, err)
	assert.NotNil(t, credential)

	md, err := credential.GetRequestMetadata(context.Background())
	expectedMd := map[string]string{
		"authorization": fmt.Sprintf("Bearer %s", string(cfg.BearerToken)),
	}
	assert.Equal(t, expectedMd, md)
	assert.NoError(t, err)
	assert.True(t, credential.RequireTransportSecurity())

	roundTripper, _ := bauth.RoundTripper(&mockRoundTripper{})
	orgHeaders := http.Header{
		"Foo": {"bar"},
	}
	expectedHeaders := http.Header{
		"Foo":           {"bar"},
		"Authorization": {"Bearer " + string(cfg.BearerToken)},
	}

	resp, err := roundTripper.RoundTrip(&http.Request{Header: orgHeaders})
	assert.NoError(t, err)
	assert.Equal(t, expectedHeaders, resp.Header)
	assert.NoError(t, bauth.Shutdown(context.Background()))
}

func TestBearerStartWatchStop(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Filename = filepath.Join("testdata", t.Name()+".token")

	bauth := newBearerTokenAuth(cfg, zaptest.NewLogger(t))
	assert.NotNil(t, bauth)

	assert.NoError(t, bauth.Start(context.Background(), componenttest.NewNopHost()))
	assert.Error(t, bauth.Start(context.Background(), componenttest.NewNopHost()))

	credential, err := bauth.PerRPCCredentials()
	assert.NoError(t, err)
	assert.NotNil(t, credential)

	token, err := os.ReadFile(bauth.filename)
	assert.NoError(t, err)

	tokenStr := fmt.Sprintf("Bearer %s", token)
	md, err := credential.GetRequestMetadata(context.Background())
	expectedMd := map[string]string{
		"authorization": tokenStr,
	}
	assert.Equal(t, expectedMd, md)
	assert.NoError(t, err)
	assert.True(t, credential.RequireTransportSecurity())

	// change file content once
	assert.NoError(t, os.WriteFile(bauth.filename, []byte(fmt.Sprintf("%stest", token)), 0o600))
	time.Sleep(5 * time.Second)
	credential, _ = bauth.PerRPCCredentials()
	md, err = credential.GetRequestMetadata(context.Background())
	expectedMd["authorization"] = tokenStr + "test"
	assert.Equal(t, expectedMd, md)
	assert.NoError(t, err)

	// change file content back
	assert.NoError(t, os.WriteFile(bauth.filename, token, 0o600))
	time.Sleep(5 * time.Second)
	credential, _ = bauth.PerRPCCredentials()
	md, err = credential.GetRequestMetadata(context.Background())
	expectedMd["authorization"] = tokenStr
	time.Sleep(5 * time.Second)
	assert.Equal(t, expectedMd, md)
	assert.NoError(t, err)

	assert.NoError(t, bauth.Shutdown(context.Background()))
	assert.Nil(t, bauth.shutdownCH)
}

func TestBearerTokenFileContentUpdate(t *testing.T) {
	scheme := "TestScheme"
	cfg := createDefaultConfig().(*Config)
	cfg.Filename = filepath.Join("testdata", t.Name()+".token")
	cfg.Scheme = scheme

	bauth := newBearerTokenAuth(cfg, zaptest.NewLogger(t))
	assert.NotNil(t, bauth)

	assert.NoError(t, bauth.Start(context.Background(), componenttest.NewNopHost()))
	assert.Error(t, bauth.Start(context.Background(), componenttest.NewNopHost()))
	defer func() { assert.NoError(t, bauth.Shutdown(context.Background())) }()

	token, err := os.ReadFile(bauth.filename)
	assert.NoError(t, err)

	base := &mockRoundTripper{}
	rt, err := bauth.RoundTripper(base)
	assert.NoError(t, err)
	assert.NotNil(t, rt)

	request := &http.Request{Method: http.MethodGet}
	resp, err := rt.RoundTrip(request)
	assert.NoError(t, err)
	authHeaderValue := resp.Header.Get("Authorization")
	assert.Equal(t, authHeaderValue, fmt.Sprintf("%s %s", scheme, string(token)))

	// change file content once
	assert.NoError(t, os.WriteFile(bauth.filename, []byte(fmt.Sprintf("%stest", token)), 0o600))
	time.Sleep(5 * time.Second)

	tokenNew, err := os.ReadFile(bauth.filename)
	assert.NoError(t, err)

	// check if request is updated with the new token
	request = &http.Request{Method: http.MethodGet}
	resp, err = rt.RoundTrip(request)
	assert.NoError(t, err)
	authHeaderValue = resp.Header.Get("Authorization")
	assert.Equal(t, authHeaderValue, fmt.Sprintf("%s %s", scheme, string(tokenNew)))

	// change file content back
	assert.NoError(t, os.WriteFile(bauth.filename, token, 0o600))
	time.Sleep(5 * time.Second)

	// check if request is updated with the old token
	request = &http.Request{Method: http.MethodGet}
	resp, err = rt.RoundTrip(request)
	assert.NoError(t, err)
	authHeaderValue = resp.Header.Get("Authorization")
	assert.Equal(t, authHeaderValue, fmt.Sprintf("%s %s", scheme, string(token)))
}

func TestBearerTokenUpdateForGrpc(t *testing.T) {
	// prepare
	cfg := createDefaultConfig().(*Config)
	cfg.BearerToken = "1234"

	bauth := newBearerTokenAuth(cfg, zaptest.NewLogger(t))
	assert.NotNil(t, bauth)

	perRPCAuth, err := bauth.PerRPCCredentials()
	assert.NoError(t, err)

	ctx := context.Background()
	assert.NoError(t, bauth.Start(ctx, componenttest.NewNopHost()))

	// initial token, OK
	md, err := perRPCAuth.GetRequestMetadata(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"authorization": "Bearer " + "1234"}, md)

	// update the token
	bauth.setAuthorizationValues([]string{"5678"})
	md, err = perRPCAuth.GetRequestMetadata(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"authorization": "Bearer " + "5678"}, md)

	assert.NoError(t, bauth.Shutdown(context.Background()))
}

func TestBearerServerAuthenticateWithScheme(t *testing.T) {
	const token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." // #nosec
	cfg := createDefaultConfig().(*Config)
	cfg.Scheme = "Bearer"
	cfg.BearerToken = token

	bauth := newBearerTokenAuth(cfg, nil)
	assert.NotNil(t, bauth)

	ctx := context.Background()
	assert.NoError(t, bauth.Start(ctx, componenttest.NewNopHost()))

	_, err := bauth.Authenticate(ctx, map[string][]string{"authorization": {"Bearer " + token}})
	assert.NoError(t, err)

	_, err = bauth.Authenticate(ctx, map[string][]string{"authorization": {"Bearer " + "1234"}})
	assert.Error(t, err)

	_, err = bauth.Authenticate(ctx, map[string][]string{"authorization": {"" + token}})
	assert.Error(t, err)

	assert.NoError(t, bauth.Shutdown(context.Background()))
}

func TestBearerServerAuthenticate(t *testing.T) {
	const token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." // #nosec
	cfg := createDefaultConfig().(*Config)
	cfg.Scheme = ""
	cfg.BearerToken = token

	bauth := newBearerTokenAuth(cfg, nil)
	assert.NotNil(t, bauth)

	ctx := context.Background()
	assert.NoError(t, bauth.Start(ctx, componenttest.NewNopHost()))

	_, err := bauth.Authenticate(ctx, map[string][]string{"authorization": {"Bearer " + token}})
	assert.Error(t, err)

	_, err = bauth.Authenticate(ctx, map[string][]string{"authorization": {"Bearer " + "1234"}})
	assert.Error(t, err)

	_, err = bauth.Authenticate(ctx, map[string][]string{"authorization": {"invalidtoken"}})
	assert.Error(t, err)

	_, err = bauth.Authenticate(ctx, map[string][]string{"authorization": {token}})
	assert.NoError(t, err)

	assert.NoError(t, bauth.Shutdown(context.Background()))
}

func TestBearerTokenMultipleTokens(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Scheme = "Bearer"
	cfg.Tokens = []configopaque.String{"token1", "token2"}

	bauth := newBearerTokenAuth(cfg, zaptest.NewLogger(t))
	assert.NotNil(t, bauth)

	assert.NoError(t, bauth.Start(context.Background(), componenttest.NewNopHost()))
	credential, err := bauth.PerRPCCredentials()
	assert.NoError(t, err)
	assert.NotNil(t, credential)

	md, err := credential.GetRequestMetadata(context.Background())
	expectedMd := map[string]string{
		"authorization": "Bearer token1",
	}
	assert.Equal(t, expectedMd, md)
	assert.NoError(t, err)
	assert.True(t, credential.RequireTransportSecurity())

	// Test Authenticate with multiple tokens
	headers := map[string][]string{
		"authorization": {"Bearer token1"},
	}
	ctx := context.Background()
	newCtx, err := bauth.Authenticate(ctx, headers)
	assert.NoError(t, err)
	assert.Equal(t, ctx, newCtx)

	headers = map[string][]string{
		"authorization": {"Bearer token2"},
	}
	newCtx, err = bauth.Authenticate(ctx, headers)
	assert.NoError(t, err)
	assert.Equal(t, ctx, newCtx)

	headers = map[string][]string{
		"authorization": {"Bearer invalidtoken"},
	}
	_, err = bauth.Authenticate(ctx, headers)
	assert.Error(t, err)

	assert.NoError(t, bauth.Shutdown(context.Background()))
}

func TestBearerTokenMultipleTokensInFile(t *testing.T) {
	scheme := "Bearer"
	filename := filepath.Join("testdata", t.Name()+".tokens")
	fileContent := "token1\ntoken2"
	err := os.WriteFile(filename, []byte(fileContent), 0o600)
	assert.NoError(t, err)
	defer os.Remove(filename)

	cfg := createDefaultConfig().(*Config)
	cfg.Scheme = scheme
	cfg.Filename = filename

	bauth := newBearerTokenAuth(cfg, zaptest.NewLogger(t))
	assert.NotNil(t, bauth)

	assert.NoError(t, bauth.Start(context.Background(), componenttest.NewNopHost()))
	credential, err := bauth.PerRPCCredentials()
	assert.NoError(t, err)
	assert.NotNil(t, credential)

	md, err := credential.GetRequestMetadata(context.Background())
	expectedMd := map[string]string{
		"authorization": "Bearer token1",
	}
	assert.Equal(t, expectedMd, md)
	assert.NoError(t, err)
	assert.True(t, credential.RequireTransportSecurity())

	// Test Authenticate with multiple tokens
	headers := map[string][]string{
		"authorization": {"Bearer token1"},
	}
	ctx := context.Background()
	newCtx, err := bauth.Authenticate(ctx, headers)
	assert.NoError(t, err)
	assert.Equal(t, ctx, newCtx)

	headers = map[string][]string{
		"authorization": {"Bearer token2"},
	}
	newCtx, err = bauth.Authenticate(ctx, headers)
	assert.NoError(t, err)
	assert.Equal(t, ctx, newCtx)

	headers = map[string][]string{
		"authorization": {"Bearer invalidtoken"},
	}
	_, err = bauth.Authenticate(ctx, headers)
	assert.Error(t, err)

	assert.NoError(t, bauth.Shutdown(context.Background()))
}

func TestCustomHeaderRoundTrip(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Header = "X-Custom-Authorization"
	cfg.Scheme = ""
	cfg.BearerToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."

	bauth := newBearerTokenAuth(cfg, nil)
	assert.NotNil(t, bauth)

	base := &mockRoundTripper{}
	c, err := bauth.RoundTripper(base)
	assert.NoError(t, err)
	assert.NotNil(t, c)

	request := &http.Request{Method: http.MethodGet}
	resp, err := c.RoundTrip(request)
	assert.NoError(t, err)
	authHeaderValue := resp.Header.Get(cfg.Header)
	assert.Equal(t, authHeaderValue, string(cfg.BearerToken))
}

func TestCustomHeaderGetRequestMetadata(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Header = "X-Custom-Authorization"
	cfg.Scheme = ""
	cfg.BearerToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."

	bauth := newBearerTokenAuth(cfg, nil)
	assert.NotNil(t, bauth)

	assert.NoError(t, bauth.Start(context.Background(), componenttest.NewNopHost()))
	credential, err := bauth.PerRPCCredentials()

	assert.NoError(t, err)
	assert.NotNil(t, credential)

	md, err := credential.GetRequestMetadata(context.Background())
	expectedMd := map[string]string{
		strings.ToLower(cfg.Header): string(cfg.BearerToken),
	}
	assert.Equal(t, expectedMd, md)
	assert.NoError(t, err)
}

func TestCustomHeaderAuthenticate(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Header = "X-Custom-Authorization"
	cfg.Scheme = ""
	cfg.BearerToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."

	bauth := newBearerTokenAuth(cfg, nil)
	assert.NotNil(t, bauth)

	ctx := context.Background()
	assert.NoError(t, bauth.Start(ctx, componenttest.NewNopHost()))

	_, err := bauth.Authenticate(ctx, map[string][]string{cfg.Header: {string(cfg.BearerToken)}})
	assert.NoError(t, err)

	assert.NoError(t, bauth.Shutdown(context.Background()))
}
