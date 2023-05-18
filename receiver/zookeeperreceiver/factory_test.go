// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zookeeperreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestFactory(t *testing.T) {
	f := NewFactory()
	require.Equal(t, component.Type("zookeeper"), f.Type())

	cfg := f.CreateDefaultConfig()
	rCfg := cfg.(*Config)

	// Assert defaults.
	assert.Equal(t, 10*time.Second, rCfg.CollectionInterval)
	assert.Equal(t, 10*time.Second, rCfg.Timeout)
	assert.Equal(t, ":2181", rCfg.Endpoint)

	tests := []struct {
		name    string
		config  component.Config
		wantErr bool
	}{
		{
			name:   "Happy path",
			config: createDefaultConfig(),
		},
		{
			name:    "Invalid endpoint",
			config:  &Config{},
			wantErr: true,
		},
		{
			name: "Invalid timeout",
			config: &Config{
				TCPAddr: confignet.TCPAddr{
					Endpoint: ":2181",
				},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, err := f.CreateMetricsReceiver(
				context.Background(),
				receivertest.NewNopCreateSettings(),
				test.config,
				consumertest.NewNop(),
			)

			if test.wantErr {
				require.Error(t, err)
				require.Nil(t, r)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, r)
		})
	}
}
