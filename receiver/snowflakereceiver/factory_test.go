// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snowflakereceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestFacoryCreate(t *testing.T) {
	factory := NewFactory()
	require.EqualValues(t, "snowflake", factory.Type())
}

func TestDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	require.Error(t, component.ValidateConfig(cfg), "Validation succeeded on invalid cfg")

	cfg.Account = "account"
	cfg.Username = "uname"
	cfg.Password = "pwd"
	cfg.Warehouse = "warehouse"
	require.NoError(t, component.ValidateConfig(cfg), "Failed to validate valid cfg")

	require.EqualValues(t, defaultDB, cfg.Database)
	require.EqualValues(t, defaultRole, cfg.Role)
	require.EqualValues(t, defaultSchema, cfg.Schema)
	require.EqualValues(t, defaultInterval, cfg.CollectionInterval)
}

func TestCreateMetricsReceiver(t *testing.T) {
	tests := []struct {
		desc string
		run  func(t *testing.T)
	}{
		{
			desc: "Defaults with valid config",
			run: func(t *testing.T) {
				t.Parallel()

				cfg := createDefaultConfig().(*Config)
				cfg.Account = "account"
				cfg.Username = "uname"
				cfg.Password = "pwd"
				cfg.Warehouse = "warehouse"

				_, err := createMetricsReceiver(
					context.Background(),
					receivertest.NewNopCreateSettings(),
					cfg,
					consumertest.NewNop(),
				)

				require.NoError(t, err, "failed to create metrics receiver with valid inputs")
			},
		},
		{
			desc: "Missing consumer",
			run: func(t *testing.T) {
				t.Parallel()

				cfg := createDefaultConfig().(*Config)

				_, err := createMetricsReceiver(
					context.Background(),
					receivertest.NewNopCreateSettings(),
					cfg,
					nil,
				)

				require.Error(t, err, "created metrics receiver without consumer")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, test.run)
	}
}
