// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googleclientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension/internal/googleclientauthextension"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/api/idtoken"
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
			Project:   "my-project",
			TokenType: idToken,
			Audience:  "http://example.com",
		},
		newIDTokenSource: idtoken.NewTokenSource,
	}
	err := ca.Start(t.Context(), nil)
	assert.NoError(t, err)

	perrpc, err := ca.PerRPCCredentials()
	assert.NotNil(t, perrpc)
	assert.NoError(t, err)
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
