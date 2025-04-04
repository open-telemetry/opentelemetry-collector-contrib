// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureauthextension

import (
	"context"
	"testing"

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

func TestGetToken(t *testing.T) {
	m := mockTokenCredential{}
	m.On("GetToken").Return(azcore.AccessToken{Token: "test"}, nil)
	auth := authenticator{
		credential: &m,
	}
	_, err := auth.GetToken(context.Background(), policy.TokenRequestOptions{})
	require.NoError(t, err)
}

type mockTokenCredential struct {
	mock.Mock
}

var _ azcore.TokenCredential = (*mockTokenCredential)(nil)

func (m *mockTokenCredential) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	args := m.Called()
	return args.Get(0).(azcore.AccessToken), args.Error(1)
}
