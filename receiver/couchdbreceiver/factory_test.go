// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package couchdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	ft := factory.Type()
	require.EqualValues(t, "couchdb", ft)
}

func TestValidConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Username = "otel"
	cfg.Password = "otel"

	require.EqualValues(t, defaultEndpoint, cfg.Endpoint)
	require.NoError(t, component.ValidateConfig(cfg))
}

func TestCreateMetricsReceiver(t *testing.T) {
	testCases := []struct {
		desc string
		run  func(t *testing.T)
	}{
		{
			desc: "Default config",
			run: func(t *testing.T) {
				t.Parallel()

				_, err := createMetricsReceiver(
					context.Background(),
					receivertest.NewNopCreateSettings(),
					createDefaultConfig(),
					consumertest.NewNop(),
				)

				require.NoError(t, err)

			},
		},
		{
			desc: "Nil consumer",
			run: func(t *testing.T) {
				t.Parallel()
				_, err := createMetricsReceiver(
					context.Background(),
					receivertest.NewNopCreateSettings(),
					createDefaultConfig(),
					nil,
				)
				require.ErrorIs(t, err, component.ErrNilNextConsumer)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, testCase.run)
	}
}
