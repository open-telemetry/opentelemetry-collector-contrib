// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signaltometricsconnector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/connector/xconnector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/metadata"
)

func TestNewFactoryWithLogs(t *testing.T) {
	for _, tc := range []struct {
		name string
		f    func(*testing.T)
	}{
		{
			name: "factory_type",
			f: func(t *testing.T) {
				factory := NewFactory()
				require.Equal(t, metadata.Type, factory.Type())
			},
		},
		{
			name: "traces_to_metrics",
			f: func(t *testing.T) {
				mc, err := consumer.NewMetrics(func(context.Context, pmetric.Metrics) error {
					return nil
				})
				require.NoError(t, err)

				factory := NewFactory()
				c, err := factory.CreateTracesToMetrics(
					context.Background(),
					connectortest.NewNopSettings(metadata.Type),
					factory.CreateDefaultConfig(),
					mc,
				)
				require.NoError(t, err)
				require.NotNil(t, c)
			},
		},
		{
			name: "logs_to_metrics",
			f: func(t *testing.T) {
				mc, err := consumer.NewMetrics(func(context.Context, pmetric.Metrics) error {
					return nil
				})
				require.NoError(t, err)

				factory := NewFactory()
				c, err := factory.CreateLogsToMetrics(
					context.Background(),
					connectortest.NewNopSettings(metadata.Type),
					factory.CreateDefaultConfig(),
					mc,
				)
				require.NoError(t, err)
				require.NotNil(t, c)
			},
		},
		{
			name: "metrics_to_metrics",
			f: func(t *testing.T) {
				mc, err := consumer.NewMetrics(func(context.Context, pmetric.Metrics) error {
					return nil
				})
				require.NoError(t, err)

				factory := NewFactory()
				c, err := factory.CreateMetricsToMetrics(
					context.Background(),
					connectortest.NewNopSettings(metadata.Type),
					factory.CreateDefaultConfig(),
					mc,
				)
				require.NoError(t, err)
				require.NotNil(t, c)
			},
		},
		{
			name: "profiles_to_metrics",
			f: func(t *testing.T) {
				mc, err := consumer.NewMetrics(func(context.Context, pmetric.Metrics) error {
					return nil
				})
				require.NoError(t, err)

				factory := NewFactory().(xconnector.Factory)
				c, err := factory.CreateProfilesToMetrics(
					context.Background(),
					connectortest.NewNopSettings(metadata.Type),
					factory.CreateDefaultConfig(),
					mc,
				)
				require.NoError(t, err)
				require.NotNil(t, c)
			},
		},
	} {
		t.Run(tc.name, tc.f)
	}
}
