// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureauthextension

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewAzureAuthenticator(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	ext, err := newAzureAuthenticator(cfg, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, ext)
}

func TestUpdateToken(t *testing.T) {
	auth := &authenticator{token: nil}

	// Token is nil at the start, so token value
	// in authenticator should be updated. We set
	// a past time to cause the token to be expired
	// in next call.
	value := "first"
	expires := time.Now().Add(-time.Hour)
	auth.credential = mockTokenCredential{
		value:   value,
		expires: expires,
	}
	token, err := auth.updateToken(context.Background())
	require.NoError(t, err)
	require.Equal(t, value, token)

	// Token is no longer nil, and it has expired.
	// It should update again.
	value = "second"
	expires = time.Now().Add(time.Hour)
	auth.credential = mockTokenCredential{
		value:   value,
		expires: expires,
	}
	token, err = auth.updateToken(context.Background())
	require.NoError(t, err)
	require.Equal(t, value, token)

	// Token hasn't expired yet, so the value
	// should remain.
	auth.credential = mockTokenCredential{
		value:   "third",
		expires: expires,
	}
	token, err = auth.updateToken(context.Background())
	require.NoError(t, err)
	require.Equal(t, value, token)
}

func TestAuthenticate(t *testing.T) {
	value := "first"
	expires := time.Now().Add(time.Hour)
	auth := &authenticator{
		token: nil,
		credential: mockTokenCredential{
			value:   value,
			expires: expires,
		},
	}
	headers := map[string][]string{}
	_, err := auth.Authenticate(context.Background(), headers)
	require.NoError(t, err)

	expected := map[string][]string{
		"Authorization": {"Bearer first"},
	}
	require.Equal(t, expected, headers)
}

type mockTokenCredential struct {
	value   string
	expires time.Time
}

var _ azcore.TokenCredential = (*mockTokenCredential)(nil)

func (m mockTokenCredential) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{
		Token:     m.value,
		ExpiresOn: m.expires,
	}, nil
}
