// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sshcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sshcheckreceiver"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sshcheckreceiver/internal/configssh"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sshcheckreceiver/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "creates a new factory with correct type",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				require.Equal(t, metadata.Type, factory.Type())
			},
		},
		{
			desc: "creates a new factory with default config",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				var expectedCfg component.Config = &Config{
					ControllerConfig: scraperhelper.ControllerConfig{
						CollectionInterval: 10 * time.Second,
						InitialDelay:       time.Second,
					},
					SSHClientSettings: configssh.SSHClientSettings{
						Timeout: 10 * time.Second,
					},
					MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				}

				require.Equal(t, expectedCfg, factory.CreateDefaultConfig())
			},
		},
		{
			desc: "creates a new factory and CreateMetrics returns no error",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				_, err := factory.CreateMetrics(
					context.Background(),
					receivertest.NewNopSettings(metadata.Type),
					cfg,
					consumertest.NewNop(),
				)
				require.NoError(t, err)
			},
		},
		{
			desc: "creates a new factory and CreateMetrics returns error with incorrect config",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				_, err := factory.CreateMetrics(
					context.Background(),
					receivertest.NewNopSettings(metadata.Type),
					nil,
					consumertest.NewNop(),
				)
				require.ErrorIs(t, err, errConfigNotSSHCheck)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
}
