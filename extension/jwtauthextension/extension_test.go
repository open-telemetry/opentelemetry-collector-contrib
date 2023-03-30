// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jwtauthextension

import (
	"context"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
	"testing"
	"time"
)

func TestJWTAuthenticationSucceeded(t *testing.T) {
	mySigningKey := []byte("secret")
	config := &Config{
		JWTSecret: string(mySigningKey),
	}
	p, err := newExtension(config, zap.NewNop())
	require.NoError(t, err)

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		// A usual scenario is to set the expiration time relative to the current time
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		NotBefore: jwt.NewNumericDate(time.Now()),
		Issuer:    "test",
		Subject:   "somebody",
		Audience:  []string{"somebody_else"},
	})

	signingString, err := token.SignedString(mySigningKey)
	assert.NoError(t, err)

	// test
	ctx, err := p.Authenticate(context.Background(), map[string][]string{"authorization": {fmt.Sprintf("Bearer %s", signingString)}})

	// verify
	assert.NoError(t, err)
	assert.NotNil(t, ctx)

	// test, upper-case header
	ctx, err = p.Authenticate(context.Background(), map[string][]string{"Authorization": {fmt.Sprintf("Bearer %s", signingString)}})

	// verify
	assert.NoError(t, err)
	assert.NotNil(t, ctx)

	clientInfo := client.FromContext(ctx)

	assert.Equal(t, "test", clientInfo.Auth.GetAttribute("issuer"))
	assert.Equal(t, "somebody", clientInfo.Auth.GetAttribute("subject"))
	assert.Equal(t, []string{"somebody_else"}, clientInfo.Auth.GetAttribute("audience"))
}

func TestJWTExpired(t *testing.T) {
	mySigningKey := []byte("secret")
	config := &Config{
		JWTSecret: string(mySigningKey),
	}
	p, err := newExtension(config, zap.NewNop())
	require.NoError(t, err)

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// Expired token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		NotBefore: jwt.NewNumericDate(time.Now()),
		Issuer:    "test",
		Subject:   "somebody",
		Audience:  []string{"somebody_else"},
	})

	signingString, err := token.SignedString(mySigningKey)
	assert.NoError(t, err)

	// test
	ctx, err := p.Authenticate(context.Background(), map[string][]string{"authorization": {fmt.Sprintf("Bearer %s", signingString)}})

	// verify
	assert.Errorf(t, err, "failed to parse token: token has invalid claims: token is expired")
	assert.NotNil(t, ctx)
}

func TestJWTEmptyClaims(t *testing.T) {
	mySigningKey := []byte("secret")
	config := &Config{
		JWTSecret: string(mySigningKey),
	}
	p, err := newExtension(config, zap.NewNop())
	require.NoError(t, err)

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// Expired token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{})

	signingString, err := token.SignedString(mySigningKey)
	assert.NoError(t, err)

	// test
	ctx, err := p.Authenticate(context.Background(), map[string][]string{"authorization": {fmt.Sprintf("Bearer %s", signingString)}})

	// verify
	assert.NoError(t, err)
	assert.NotNil(t, ctx)
}

func TestJWTInvalidAuthHeader(t *testing.T) {
	// prepare
	p, err := newExtension(&Config{
		JWTSecret: "secret",
	}, zap.NewNop())
	require.NoError(t, err)

	// test
	ctx, err := p.Authenticate(context.Background(), map[string][]string{"authorization": {"some-value"}})

	// verify
	assert.Equal(t, errInvalidAuthenticationHeaderFormat, err)
	assert.NotNil(t, ctx)
}

func TestJWTNotAuthenticated(t *testing.T) {
	// prepare
	p, err := newExtension(&Config{
		JWTSecret: "secret",
	}, zap.NewNop())
	require.NoError(t, err)

	// test
	ctx, err := p.Authenticate(context.Background(), make(map[string][]string))

	// verify
	assert.Equal(t, errNotAuthenticated, err)
	assert.NotNil(t, ctx)
}

func TestFailedToVerifyToken(t *testing.T) {
	p, err := newExtension(&Config{
		JWTSecret: "secret",
	}, zap.NewNop())
	require.NoError(t, err)

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// test
	ctx, err := p.Authenticate(context.Background(), map[string][]string{"authorization": {"Bearer some-token"}})

	// verify
	assert.Error(t, err)
	assert.NotNil(t, ctx)
}

func TestMissingSecret(t *testing.T) {
	// prepare
	config := &Config{
		JWTSecret: "",
	}

	// test
	p, err := newExtension(config, zap.NewNop())

	// verify
	assert.Nil(t, p)
	assert.Equal(t, errNoJWTSecretProvided, err)
}

func TestShutdown(t *testing.T) {
	// prepare
	config := &Config{
		JWTSecret: "secret",
	}
	p, err := newExtension(config, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, p)

	// test
	err = p.Shutdown(context.Background()) // for now, we never fail

	// verify
	assert.NoError(t, err)
}
