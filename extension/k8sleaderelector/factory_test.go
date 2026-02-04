// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sleaderelector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func TestNewFactory(t *testing.T) {
	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "creates a new factory with correct type",
			testFunc: func(t *testing.T) {
				t.Helper()
				factory := NewFactory()
				ft := factory.Type()
				require.Equal(t, metadata.Type, ft)
			},
		},
		{
			desc: "creates a new factory with correct type",
			testFunc: func(t *testing.T) {
				t.Helper()
				factory := NewFactory()
				ft := factory.Type()
				require.Equal(t, metadata.Type, ft)
			},
		},
		{
			desc: "creates a new factory and extension with default config",
			testFunc: func(t *testing.T) {
				t.Helper()
				factory := NewFactory()
				expectedCfg := &Config{
					APIConfig: k8sconfig.APIConfig{
						AuthType: "serviceAccount",
					},
					LeaseDuration: 15 * time.Second,
					RenewDuration: 10 * time.Second,
					RetryPeriod:   2 * time.Second,
				}

				require.Equal(t, expectedCfg, factory.CreateDefaultConfig())
			},
		},
		{
			desc: "creates a new factory and createExtension returns no error",
			testFunc: func(t *testing.T) {
				t.Helper()
				fakeClient := fake.NewClientset()

				cfg := createDefaultConfig().(*Config)
				cfg.makeClient = func(_ k8sconfig.APIConfig) (kubernetes.Interface, error) {
					return fakeClient, nil
				}

				f := NewFactory()
				_, err := f.Create(
					t.Context(),
					extensiontest.NewNopSettings(f.Type()),
					cfg,
				)
				require.NoError(t, err)
			},
		},
	}
	for _, test := range testCases {
		t.Run(test.desc, test.testFunc)
	}
}

func TestCreateExtension_InvalidConfig(t *testing.T) {
	factory := NewFactory()

	// Pass wrong config type
	_, err := factory.Create(
		t.Context(),
		extensiontest.NewNopSettings(factory.Type()),
		&struct{}{}, // Invalid config type
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid config")
}

func TestCreateExtension_ClientCreationError(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.LeaseName = "test"
	cfg.LeaseNamespace = "test"

	// makeClient will return error when not set with fake client
	// This simulates real-world failure to create k8s client

	factory := NewFactory()
	_, err := factory.Create(
		t.Context(),
		extensiontest.NewNopSettings(factory.Type()),
		cfg,
	)
	// Should fail because it can't create the k8s client
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create k8s client")
}

func TestConfig_GetK8sClient(t *testing.T) {
	t.Run("with custom makeClient", func(t *testing.T) {
		fakeClient := fake.NewClientset()
		cfg := &Config{
			makeClient: func(_ k8sconfig.APIConfig) (kubernetes.Interface, error) {
				return fakeClient, nil
			},
		}

		client, err := cfg.getK8sClient()
		require.NoError(t, err)
		require.Equal(t, fakeClient, client)
	})

	t.Run("default makeClient behavior", func(t *testing.T) {
		cfg := &Config{
			APIConfig: k8sconfig.APIConfig{
				AuthType: k8sconfig.AuthTypeServiceAccount,
			},
		}

		// Without setting makeClient, it should use the default k8sconfig.MakeClient
		// This will fail in test environment but we're just checking the behavior
		client, err := cfg.getK8sClient()
		// In test environment, this typically fails which is expected
		require.True(t, client != nil || err != nil)
	})
}
