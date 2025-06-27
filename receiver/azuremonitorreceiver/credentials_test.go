// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
)

func TestLoadCredentials(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(*testing.T)
	}{
		{
			name: "default_credentials",
			testFunc: func(t *testing.T) {
				cfg := &Config{
					Credentials: defaultCredentials,
				}
				cred, err := loadCredentials(zap.NewNop(), cfg, componenttest.NewNopHost())
				require.NoError(t, err)
				require.NotNil(t, cred)
				require.IsType(t, &azidentity.DefaultAzureCredential{}, cred)
			},
		},
		{
			name: "service_principal",
			testFunc: func(t *testing.T) {
				cfg := &Config{
					Credentials:  servicePrincipal,
					TenantID:     "test",
					ClientID:     "test",
					ClientSecret: "test",
				}
				cred, err := loadCredentials(zap.NewNop(), cfg, componenttest.NewNopHost())
				require.NoError(t, err)
				require.NotNil(t, cred)
				require.IsType(t, &azidentity.ClientSecretCredential{}, cred)
			},
		},
		{
			name: "workload_identity",
			testFunc: func(t *testing.T) {
				cfg := &Config{
					Credentials:        workloadIdentity,
					ClientID:           "test",
					FederatedTokenFile: "test",
					TenantID:           "test",
				}
				cred, err := loadCredentials(zap.NewNop(), cfg, componenttest.NewNopHost())
				require.NoError(t, err)
				require.NotNil(t, cred)
				require.IsType(t, &azidentity.WorkloadIdentityCredential{}, cred)
			},
		},
		{
			name: "managed_identity",
			testFunc: func(t *testing.T) {
				cfg := &Config{
					Credentials: managedIdentity,
					ClientID:    "test",
				}
				cred, err := loadCredentials(zap.NewNop(), cfg, componenttest.NewNopHost())
				require.NoError(t, err)
				require.NotNil(t, cred)
				require.IsType(t, &azidentity.ManagedIdentityCredential{}, cred)
			},
		},
		{
			name: "auth",
			testFunc: func(t *testing.T) {
				cfg := &Config{
					Authentication: &AuthConfig{AuthenticatorID: component.MustNewID("test")},
				}
				_, err := loadCredentials(zap.NewNop(), cfg, componenttest.NewNopHost())
				require.ErrorContains(t, err, `unknown azureauth extension "test"`)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.testFunc)
	}
}
