// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package leaderelector

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/leaderelector/internal/metadata"
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
				require.EqualValues(t, metadata.Type, ft)
			},
		},
		{
			desc: "creates a new factory with correct type",
			testFunc: func(t *testing.T) {
				t.Helper()
				factory := NewFactory()
				ft := factory.Type()
				require.EqualValues(t, metadata.Type, ft)
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
			desc: "creates a new factory and CreateExtension returns no error",
			testFunc: func(t *testing.T) {
				t.Helper()
				fakeClient := fake.NewClientset()

				cfg := CreateDefaultConfig().(*Config)
				cfg.makeClient = func(apiConfig k8sconfig.APIConfig) (kubernetes.Interface, error) {
					return fakeClient, nil
				}

				_, err := NewFactory().Create(
					context.Background(),
					extensiontest.NewNopSettings(),
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
