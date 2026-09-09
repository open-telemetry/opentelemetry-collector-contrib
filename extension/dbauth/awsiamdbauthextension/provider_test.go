// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsiamdbauthextension

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth"
)

func staticCredentials() aws.CredentialsProvider {
	return aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{
			AccessKeyID:     "test-access-key",
			SecretAccessKey: "test-secret-key",
			Source:          "test",
		}, nil
	})
}

func newTestExtension(region string, credentials aws.CredentialsProvider) *iamExtension {
	return &iamExtension{
		awsConfig: aws.Config{
			Region:      region,
			Credentials: credentials,
		},
	}
}

func parseToken(t *testing.T, token string) *url.URL {
	t.Helper()
	u, err := url.Parse("https://" + token)
	require.NoError(t, err)
	return u
}

// newProviderExtension creates the extension via the factory and asserts it
// implements dbauth.Provider — the dual role consumers depend on.
func newProviderExtension(t *testing.T, cfg *Config) dbauth.Provider {
	t.Helper()
	ext, err := createExtension(t.Context(), extension.Settings{}, cfg)
	require.NoError(t, err)
	p, ok := ext.(dbauth.Provider)
	require.True(t, ok, "the aws_iam_db_auth extension must implement dbauth.Provider")
	return p
}

func TestFactory_TypeAndStability(t *testing.T) {
	f := NewFactory()
	assert.Equal(t, "aws_iam_db_auth", f.Type().String())
}

func TestFactory_DefaultConfig(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig()
	_, ok := cfg.(*Config)
	assert.True(t, ok, "default config is *Config")
}

func TestExtension_ImplementsProvider(t *testing.T) {
	p := newProviderExtension(t, &Config{Region: "us-east-1"})
	require.NotNil(t, p)
}

func TestExtension_StartShutdownNoop(t *testing.T) {
	ext, err := createExtension(t.Context(), extension.Settings{}, &Config{Region: "us-east-1"})
	require.NoError(t, err)
	require.NoError(t, ext.Start(t.Context(), nil))
	require.NoError(t, ext.Shutdown(t.Context()))
}

func TestGetCredential(t *testing.T) {
	e := newTestExtension("us-east-1", staticCredentials())

	earliestExpiry := time.Now().Add(rdsTokenLifetime)
	cred, err := e.GetCredential(t.Context(), dbauth.Request{Endpoint: "db:5432", Username: "monitor"})
	latestExpiry := time.Now().Add(rdsTokenLifetime)
	require.NoError(t, err)

	assert.Nil(t, cred.Username, "AWS IAM supplies no username; consumer uses its configured one")
	require.NotNil(t, cred.NotAfter)
	assert.False(t, cred.NotAfter.Before(earliestExpiry))
	assert.False(t, cred.NotAfter.After(latestExpiry))

	u := parseToken(t, cred.Secret)
	assert.Equal(t, "db:5432", u.Host)
	assert.Equal(t, "connect", u.Query().Get("Action"))
	assert.Equal(t, "monitor", u.Query().Get("DBUser"))
	assert.Equal(t, "900", u.Query().Get("X-Amz-Expires"))
	assert.True(t, strings.HasPrefix(u.Query().Get("X-Amz-Credential"), "test-access-key/"))
	assert.Contains(t, u.Query().Get("X-Amz-Credential"), "/us-east-1/rds-db/aws4_request")
}

func TestGetCredential_MintError(t *testing.T) {
	sentinel := errors.New("mint failed")
	e := newTestExtension("us-east-1", aws.CredentialsProviderFunc(
		func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{}, sentinel
		},
	))
	_, err := e.GetCredential(t.Context(), dbauth.Request{Endpoint: "db:5432", Username: "monitor"})
	require.ErrorIs(t, err, sentinel)
	assert.ErrorContains(t, err, `aws_iam_db_auth: mint RDS token for "db:5432"`)
}
