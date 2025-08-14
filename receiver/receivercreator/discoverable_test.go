// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
)

// mockDiscoverableConfig implements both component.Config and Discoverable
type mockDiscoverableConfig struct {
	validateFunc func(rawCfg map[string]any, discoveredEndpoint string) error
}

func (m *mockDiscoverableConfig) ValidateDiscovery(rawCfg map[string]any, discoveredEndpoint string) error {
	if m.validateFunc != nil {
		return m.validateFunc(rawCfg, discoveredEndpoint)
	}
	return nil
}

// mockNonDiscoverableConfig is a regular config that doesn't implement Discoverable
type mockNonDiscoverableConfig struct{}

func TestMergeTemplatedAndDiscoveredConfigs_WithDiscoverableReceiver(t *testing.T) {
	tests := []struct {
		name             string
		templated        userConfigMap
		discovered       userConfigMap
		validateFunc     func(rawCfg map[string]any, discoveredEndpoint string) error
		expectedEndpoint string
		expectError      bool
	}{
		{
			name: "successful discoverable validation",
			templated: userConfigMap{
				"job_name": "test-job",
				"config": map[string]any{
					"scrape_configs": []any{
						map[string]any{
							"job_name": "discovered-app",
							"static_configs": []any{
								map[string]any{
									"targets": []any{"`endpoint`"},
								},
							},
						},
					},
				},
			},
			discovered: userConfigMap{
				endpointConfigKey:       "10.1.2.3:8080",
				tmpSetEndpointConfigKey: struct{}{},
			},
			validateFunc: func(_ map[string]any, _ string) error {
				// Mock successful validation
				return nil
			},
			expectedEndpoint: "10.1.2.3:8080",
			expectError:      false,
		},
		{
			name: "failed discoverable validation",
			templated: userConfigMap{
				"config": map[string]any{
					"scrape_configs": []any{
						map[string]any{
							"job_name": "malicious-job",
							"static_configs": []any{
								map[string]any{
									"targets": []any{"evil.com:9090"},
								},
							},
						},
					},
				},
			},
			discovered: userConfigMap{
				endpointConfigKey:       "10.1.2.3:8080",
				tmpSetEndpointConfigKey: struct{}{},
			},
			validateFunc: func(_ map[string]any, _ string) error {
				return assert.AnError // Mock validation failure
			},
			expectedEndpoint: "10.1.2.3:8080",
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock factory that returns discoverable config
			factory := receiver.NewFactory(
				component.MustNewType("mock_discoverable"),
				func() component.Config { return &mockDiscoverableConfig{validateFunc: tt.validateFunc} },
			)

			result, endpoint, err := mergeTemplatedAndDiscoveredConfigs(factory, tt.templated, tt.discovered)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "discoverable validation failed")
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				// Verify endpoint was not injected for discoverable receivers
				resultMap := result.ToStringMap()
				_, hasEndpoint := resultMap[endpointConfigKey]
				assert.False(t, hasEndpoint, "Discoverable receivers should not have endpoint field injected")
			}

			assert.Equal(t, tt.expectedEndpoint, endpoint)
		})
	}
}

func TestMergeTemplatedAndDiscoveredConfigs_WithNonDiscoverableReceiver(t *testing.T) {
	// Create mock factory that returns non-discoverable config
	factory := receiver.NewFactory(
		component.MustNewType("mock_non_discoverable"),
		func() component.Config { return &mockNonDiscoverableConfig{} },
	)

	templated := userConfigMap{
		"collection_interval": "30s",
	}
	discovered := userConfigMap{
		endpointConfigKey:       "10.1.2.3:8080",
		tmpSetEndpointConfigKey: struct{}{},
	}

	result, endpoint, err := mergeTemplatedAndDiscoveredConfigs(factory, templated, discovered)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "10.1.2.3:8080", endpoint)

	// Verify the old behavior still works for non-discoverable receivers
	// (endpoint injection logic should still run)
	resultMap := result.ToStringMap()
	assert.Equal(t, "30s", resultMap["collection_interval"])
}
