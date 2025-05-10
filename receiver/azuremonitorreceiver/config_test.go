// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr string
	}{
		{
			id: component.NewIDWithName(metadata.Type, "valid_subscription_ids"),
			expected: func() component.Config {
				cfg := createDefaultConfig().(*Config)
				cfg.SubscriptionIDs = []string{"test"}
				cfg.Credentials = defaultCredentials
				return cfg
			}(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "valid_discover_subscription"),
			expected: func() component.Config {
				cfg := createDefaultConfig().(*Config)
				cfg.DiscoverSubscriptions = true
				cfg.Credentials = defaultCredentials
				return cfg
			}(),
		},
		{
			id:          component.NewIDWithName(metadata.Type, "missing_subscription"),
			expectedErr: errMissingSubscriptionIDs.Error(),
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_cloud"),
			expectedErr: errInvalidCloud.Error(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "missing_service_principal"),
			expectedErr: fmt.Sprintf(
				"%s; %s; %s",
				errMissingTenantID.Error(),
				errMissingClientID.Error(),
				errMissingClientSecret.Error(),
			),
		},
		{
			id: component.NewIDWithName(metadata.Type, "missing_service_principal"),
			expectedErr: fmt.Sprintf(
				"%s; %s; %s",
				errMissingTenantID.Error(),
				errMissingClientID.Error(),
				errMissingClientSecret.Error(),
			),
		},
		{
			id: component.NewIDWithName(metadata.Type, "missing_workload_identity"),
			expectedErr: fmt.Sprintf(
				"%s; %s; %s",
				errMissingTenantID.Error(),
				errMissingClientID.Error(),
				errMissingFedTokenFile.Error(),
			),
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_credentials"),
			expectedErr: `credentials "invalid" is not supported`,
		},
		{
			id: component.NewIDWithName(metadata.Type, "valid_authenticator"),
			expected: func() component.Config {
				cfg := createDefaultConfig().(*Config)
				cfg.DiscoverSubscriptions = true
				cfg.Authentication = &AuthConfig{
					AuthenticatorID: component.MustNewIDWithName("azureauth", "monitor"),
				}
				cfg.Credentials = "does-not-matter"
				return cfg
			}(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "valid_authenticator_2"),
			expected: func() component.Config {
				cfg := createDefaultConfig().(*Config)
				cfg.DiscoverSubscriptions = true
				cfg.Authentication = &AuthConfig{
					AuthenticatorID: component.MustNewID("azureauth"),
				}
				cfg.Credentials = "does-not-matter"
				return cfg
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.Name(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			err = xconfmap.Validate(cfg)
			if tt.expectedErr != "" {
				assert.ErrorContains(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, cfg)
			}
		})
	}
}
