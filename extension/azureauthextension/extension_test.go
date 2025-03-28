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
	t.Parallel()

	token := "token"
	expires := time.Now().Add(time.Hour)
	auth := &authenticator{
		token: nil,
		credential: mockTokenCredential{
			value:   token,
			expires: expires,
		},
	}

	tests := map[string]struct {
		headers     map[string][]string
		expectedErr string
	}{
		"missing_authorization_header": {
			headers:     map[string][]string{},
			expectedErr: errMissingAuthorizationHeader.Error(),
		},
		"empty_authorization_header": {
			headers: map[string][]string{
				authorizationHeader: {},
			},
			expectedErr: errEmptyAuthorizationHeader.Error(),
		},
		"unexpected_authorization_value_format": {
			headers: map[string][]string{
				authorizationHeader: {"unexpected-format"},
			},
			expectedErr: errUnexpectedAuthorizationFormat.Error(),
		},
		"wrong_scheme": {
			headers: map[string][]string{
				authorizationHeader: {"wrong scheme"},
			},
			expectedErr: `expected "Bearer" scheme, got "wrong"`,
		},
		"invalid_token": {
			headers: map[string][]string{
				authorizationHeader: {"Bearer invalid"},
			},
			expectedErr: errUnexpectedToken.Error(),
		},
		"valid_token": {
			headers: map[string][]string{
				authorizationHeader: {"Bearer " + token},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := auth.Authenticate(context.Background(), test.headers)
			if test.expectedErr == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, test.expectedErr)
		})
	}
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
