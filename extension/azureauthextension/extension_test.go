// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureauthextension

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/stretchr/testify/mock"
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
	tests := map[string]struct {
		mockSetup   func(credential *mockTokenCredential)
		expectedErr string
	}{
		"successful": {
			mockSetup: func(m *mockTokenCredential) {
				m.On("GetToken").Return(azcore.AccessToken{}, nil)
			},
		},
		"unsuccessful": {
			mockSetup: func(m *mockTokenCredential) {
				m.On("GetToken").Return(azcore.AccessToken{}, errors.New("test"))
			},
			expectedErr: "azure authenticator failed to get token",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			credentials := &mockTokenCredential{}
			test.mockSetup(credentials)

			auth := &authenticator{credential: credentials}
			_, err := auth.updateToken(context.Background())
			if test.expectedErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, test.expectedErr)
		})
	}
}

func TestTrackToken(t *testing.T) {
	expiresOn := time.Now().Add(time.Hour)
	token := "token"

	// GetToken is only called when the token needs to
	// be updated. Let's ensure it's only called once,
	// despite concurrent access.
	mockCred := &mockTokenCredential{}
	mockCred.On("GetToken").
		Return(azcore.AccessToken{
			Token:     token,
			ExpiresOn: expiresOn,
		}, nil).Once()

	a := &authenticator{
		credential: mockCred,
		logger:     zap.NewNop(),
	}

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	go a.trackToken(ctx)

	require.Eventuallyf(t, func() bool {
		_, err := a.getCurrentToken()
		return err == nil
	}, time.Millisecond*10, time.Millisecond, "failed to wait for token to be available at start")

	type result struct {
		token azcore.AccessToken
		err   error
	}
	numGoroutines := 10
	results := make(chan result, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			got, err := a.getCurrentToken()
			results <- result{
				token: got,
				err:   err,
			}
		}()
	}

	for i := 0; i < numGoroutines; i++ {
		r := <-results
		require.NoError(t, r.err)
		require.Equal(t, azcore.AccessToken{
			Token:     token,
			ExpiresOn: expiresOn,
		}, r.token)
	}

	close(results)
}

func TestGetCurrentToken(t *testing.T) {
	auth := authenticator{}
	_, err := auth.getCurrentToken()
	require.ErrorIs(t, err, errUnavailableToken)

	token := azcore.AccessToken{Token: "test"}
	auth.token.Store(token)
	got, err := auth.getCurrentToken()
	require.NoError(t, err)
	require.Equal(t, token, got)
}

func TestAuthenticate(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	token := "token"
	credential := &mockTokenCredential{}
	credential.On("GetToken").Return(azcore.AccessToken{
		Token: token,
	})
	auth := &authenticator{
		credential: credential,
		logger:     zap.NewNop(),
	}
	auth.token.Store(azcore.AccessToken{Token: token})

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
			_, err := auth.Authenticate(ctx, test.headers)
			if test.expectedErr == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, test.expectedErr)
		})
	}
}

type mockTokenCredential struct {
	mock.Mock
}

var _ azcore.TokenCredential = (*mockTokenCredential)(nil)

func (m *mockTokenCredential) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	args := m.Called()
	return args.Get(0).(azcore.AccessToken), args.Error(1)
}
