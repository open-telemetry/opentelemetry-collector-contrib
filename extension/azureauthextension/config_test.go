// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureauthextension

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/azureauthextension/internal/metadata"
)

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		cfg         *Config
		expectedErr string
	}{
		"no_authentication_method": {
			cfg:         &Config{},
			expectedErr: errEmptyAuthentication.Error(),
		},
		"use_default_only": {
			cfg: &Config{UseDefault: true},
		},
		"server_both_fields_empty": {
			cfg: &Config{
				UseDefault: true,
				Server:     configoptional.Some(Server{}),
			},
			expectedErr: errors.Join(errEmptyServerIssuerURL, errEmptyServerAudience).Error(),
		},
		"server_valid": {
			cfg: &Config{
				UseDefault: true,
				Server: configoptional.Some(Server{
					IssuerURL: "https://issuer.example",
					Audience:  "api://aud",
				}),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := tc.cfg.Validate()
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, errLoad := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, errLoad)

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr string
	}{
		{
			id:          component.NewIDWithName(metadata.Type, ""),
			expectedErr: errEmptyAuthentication.Error(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "use_default"),
			expected: &Config{
				UseDefault: true,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "valid_server_auth"),
			expected: &Config{
				UseDefault: true,
				Server: configoptional.Some(Server{
					IssuerURL: "https://login.microsoftonline.com/test/v2.0",
					Audience:  "api://collector-ingest",
				}),
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "server_auth_empty_issuer"),
			expectedErr: errEmptyServerIssuerURL.Error(),
		},
		{
			id:          component.NewIDWithName(metadata.Type, "server_auth_empty_audience"),
			expectedErr: errEmptyServerAudience.Error(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "valid_service_principal_secret"),
			expected: &Config{
				ServicePrincipal: configoptional.Some(ServicePrincipal{
					TenantID:     "test",
					ClientID:     "test",
					ClientSecret: "test",
				}),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "valid_service_principal_certificate"),
			expected: &Config{
				ServicePrincipal: configoptional.Some(ServicePrincipal{
					TenantID:              "test",
					ClientID:              "test",
					ClientCertificatePath: "test",
				}),
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "service_principal_mutually_exclusive"),
			expectedErr: fmt.Sprintf("%s: %s", "service_principal", errMutuallyExclusiveAuth.Error()),
		},
		{
			id:          component.NewIDWithName(metadata.Type, "service_principal_empty_client_id"),
			expectedErr: fmt.Sprintf("%s: %s", "service_principal", errEmptyClientID.Error()),
		},
		{
			id:          component.NewIDWithName(metadata.Type, "service_principal_empty_tenant_id"),
			expectedErr: fmt.Sprintf("%s: %s", "service_principal", errEmptyTenantID.Error()),
		},
		{
			id:          component.NewIDWithName(metadata.Type, "service_principal_empty_client_credential"),
			expectedErr: fmt.Sprintf("%s: %s", "service_principal", errEmptyClientCredential.Error()),
		},
		{
			id: component.NewIDWithName(metadata.Type, "valid_workload_identity"),
			expected: &Config{
				Workload: configoptional.Some(WorkloadIdentity{
					TenantID:           "test",
					ClientID:           "test",
					FederatedTokenFile: "test",
				}),
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "workload_identity_empty_client_id"),
			expectedErr: fmt.Sprintf("%s: %s", "workload_identity", errEmptyClientID.Error()),
		},
		{
			id:          component.NewIDWithName(metadata.Type, "workload_identity_empty_tenant_id"),
			expectedErr: fmt.Sprintf("%s: %s", "workload_identity", errEmptyTenantID.Error()),
		},
		{
			id:          component.NewIDWithName(metadata.Type, "workload_identity_empty_federated_token_file"),
			expectedErr: fmt.Sprintf("%s: %s", "workload_identity", errEmptyFederatedTokenFile.Error()),
		},
	}

	for _, tt := range tests {
		name := strings.ReplaceAll(tt.id.String(), "/", "_")
		t.Run(name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			err = xconfmap.Validate(cfg)
			if tt.expectedErr != "" {
				require.Error(t, err)
				require.EqualError(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, cfg)
			}
		})
	}
}
