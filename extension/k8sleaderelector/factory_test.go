// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sleaderelector

import (
	"context"
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
					context.Background(),
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
