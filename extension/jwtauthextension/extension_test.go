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
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
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

	cc := jwt.MapClaims{
		// Expire in 24 hours
		"exp": jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		"iat": jwt.NewNumericDate(time.Now()),
		"nbf": jwt.NewNumericDate(time.Now()),
		"iss": "test",
		"sub": "somebody",
		"aud": []string{"somebody_else"},

		// This is a custom claim
		"projectID": "123456789",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, cc)

	signingString, err := token.SignedString(mySigningKey)
	assert.NoError(t, err)

	ctx, err := p.Authenticate(context.Background(), map[string][]string{"authorization": {fmt.Sprintf("Bearer %s", signingString)}})

	assert.NoError(t, err)
	assert.NotNil(t, ctx)

	clientInfo := client.FromContext(ctx)

	assert.Equal(t, "test", clientInfo.Auth.GetAttribute("iss"))
	assert.Equal(t, "somebody", clientInfo.Auth.GetAttribute("sub"))
	assert.Equal(t, []any{"somebody_else"}, clientInfo.Auth.GetAttribute("aud"))
	assert.Equal(t, "123456789", clientInfo.Auth.GetAttribute("projectID"))
	assert.Len(t, clientInfo.Auth.GetAttributeNames(), 7) // 6 standard claims + 1 custom claim

	// test, upper-case header
	ctx, err = p.Authenticate(context.Background(), map[string][]string{"Authorization": {fmt.Sprintf("Bearer %s", signingString)}})

	assert.NoError(t, err)
	assert.NotNil(t, ctx)

	clientInfo = client.FromContext(ctx)

	assert.Equal(t, "test", clientInfo.Auth.GetAttribute("iss"))
	assert.Equal(t, "somebody", clientInfo.Auth.GetAttribute("sub"))
	assert.Equal(t, []any{"somebody_else"}, clientInfo.Auth.GetAttribute("aud"))
	assert.Equal(t, "123456789", clientInfo.Auth.GetAttribute("projectID"))
	assert.Len(t, clientInfo.Auth.GetAttributeNames(), 7) // 6 standard claims + 1 custom claim
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

	ctx, err := p.Authenticate(context.Background(), map[string][]string{"authorization": {fmt.Sprintf("Bearer %s", signingString)}})

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

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{})

	signingString, err := token.SignedString(mySigningKey)
	assert.NoError(t, err)

	ctx, err := p.Authenticate(context.Background(), map[string][]string{"authorization": {fmt.Sprintf("Bearer %s", signingString)}})

	assert.NoError(t, err)
	assert.NotNil(t, ctx)
}

func TestJWTInvalidAuthHeader(t *testing.T) {

	p, err := newExtension(&Config{
		JWTSecret: "secret",
	}, zap.NewNop())
	require.NoError(t, err)

	ctx, err := p.Authenticate(context.Background(), map[string][]string{"authorization": {"some-value"}})

	assert.Equal(t, errInvalidAuthenticationHeaderFormat, err)
	assert.NotNil(t, ctx)
}

func TestJWTNotAuthenticated(t *testing.T) {

	p, err := newExtension(&Config{
		JWTSecret: "secret",
	}, zap.NewNop())
	require.NoError(t, err)

	// test
	ctx, err := p.Authenticate(context.Background(), make(map[string][]string))

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

	ctx, err := p.Authenticate(context.Background(), map[string][]string{"authorization": {"Bearer some-token"}})

	assert.Error(t, err)
	assert.NotNil(t, ctx)
}

func TestMissingSecret(t *testing.T) {

	config := &Config{
		JWTSecret: "",
	}

	p, err := newExtension(config, zap.NewNop())

	assert.Nil(t, p)
	assert.Equal(t, errNoJWTSecretProvided, err)
}

func TestShutdown(t *testing.T) {

	config := &Config{
		JWTSecret: "secret",
	}
	p, err := newExtension(config, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, p)

	err = p.Shutdown(context.Background()) // for now, we never fail

	assert.NoError(t, err)
}
