// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientauth // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension/internal/clientauth"

import (
	"crypto/tls"
	"testing"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials/idtoken"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/credentials"
)

func TestPerRPCCredentials(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds.json")
	ca := clientAuthenticator{config: &Config{
		Project:      "my-project",
		QuotaProject: "other-project",
		TokenType:    accessToken,
	}}
	err := ca.Start(t.Context(), nil)
	assert.NoError(t, err)

	perrpc, err := ca.PerRPCCredentials()
	assert.NotNil(t, perrpc)
	assert.NoError(t, err)
}

func TestPerRPCCredentialsWithIDToken(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_isa_creds.json")
	ca := clientAuthenticator{
		config: &Config{
			Project:      "my-project",
			QuotaProject: "other-project",
			TokenType:    idToken,
			Audience:     "http://example.com",
		},
		newIDTokenCredentials: func(_ *idtoken.Options) (*auth.Credentials, error) {
			return auth.NewCredentials(&auth.CredentialsOptions{
				TokenProvider: &mockTokenProvider{token: "dummy token"},
			}), nil
		},
	}
	err := ca.Start(t.Context(), nil)
	assert.NoError(t, err)

	perrpc, err := ca.PerRPCCredentials()
	assert.NotNil(t, perrpc)
	assert.NoError(t, err)

	ctx := credentials.NewContextWithRequestInfo(t.Context(), credentials.RequestInfo{
		AuthInfo: credentials.TLSInfo{State: tls.ConnectionState{Version: tls.VersionTLS13}},
	})
	md, err := perrpc.GetRequestMetadata(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "other-project", md["X-goog-user-project"])
	assert.Equal(t, "my-project", md["X-goog-project-id"])
	assert.Equal(t, "Bearer dummy token", md["authorization"])
}

func TestPerRPCCredentialsNotStarted(t *testing.T) {
	ca := clientAuthenticator{config: &Config{
		Project:      "my-project",
		QuotaProject: "other-project",
		TokenType:    accessToken,
	}}
	perrpc, err := ca.PerRPCCredentials()
	assert.Nil(t, perrpc)
	assert.Error(t, err)
}
