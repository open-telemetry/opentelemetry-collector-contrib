// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ciscoosreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/metadata"
)

func Test_collectMetrics_success(t *testing.T) {
	tests := []struct {
		name            string
		config          *Config
		wantErr         bool
		expectedMetrics int
	}{
		{
			name: "successful_collection",
			config: &Config{
				CollectionInterval: 30 * time.Second,
				Timeout:            10 * time.Second,
				Collectors: CollectorsConfig{
					BGP: true,
				},
				Devices: []DeviceConfig{
					{
						Host:     "192.168.1.1:22",
						Username: "admin",
						Password: "password",
					},
				},
			},
			wantErr:         false,
			expectedMetrics: 1, // Always generates metrics (even with connection failures)
		},
		{
			name: "connection_failure",
			config: &Config{
				CollectionInterval: 30 * time.Second,
				Timeout:            1 * time.Second, // Short timeout to fail quickly
				Collectors: CollectorsConfig{
					BGP: true,
				},
				Devices: []DeviceConfig{
					{
						Host:     "invalid-host:22",
						Username: "admin",
						Password: "password",
					},
				},
			},
			wantErr:         false, // Connection failures don't cause test failures
			expectedMetrics: 1,     // Always generates metrics (even with connection failures)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumer := &consumertest.MetricsSink{}
			settings := receivertest.NewNopSettings(metadata.Type)

			receiver, err := newModularCiscoReceiver(tt.config, settings, consumer)
			require.NoError(t, err)

			// Test collection without starting the receiver
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			// Cast to access internal method
			r := receiver.(*modularCiscoReceiver)
			r.collectAndSendMetrics(ctx)

			// Verify no errors occurred (connection failures are handled gracefully)
			assert.Len(t, consumer.AllMetrics(), tt.expectedMetrics)
		})
	}
}

func TestNewReceiver(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid_config",
			config: &Config{
				CollectionInterval: 30 * time.Second,
				Timeout:            10 * time.Second,
				Collectors: CollectorsConfig{
					BGP: true,
				},
				Devices: []DeviceConfig{
					{
						Host:     "192.168.1.1:22",
						Username: "admin",
						Password: "password",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty_devices",
			config: &Config{
				CollectionInterval: 30 * time.Second,
				Timeout:            10 * time.Second,
				Collectors: CollectorsConfig{
					BGP: true,
				},
				Devices: []DeviceConfig{},
			},
			wantErr: true,
		},
		{
			name: "invalid_timeout",
			config: &Config{
				CollectionInterval: 30 * time.Second,
				Timeout:            0,
				Collectors: CollectorsConfig{
					BGP: true,
				},
				Devices: []DeviceConfig{
					{
						Host:     "192.168.1.1:22",
						Username: "admin",
						Password: "password",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumer := &consumertest.MetricsSink{}
			settings := receivertest.NewNopSettings(metadata.Type)

			receiver, err := newModularCiscoReceiver(tt.config, settings, consumer)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, receiver)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, receiver)
			}
		})
	}
}

// MockMetricsConsumer for testing - following awss3receiver pattern
type MockMetricsConsumer struct {
	metrics []pmetric.Metrics
	errors  []error
}

func (m *MockMetricsConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (m *MockMetricsConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	m.metrics = append(m.metrics, md)
	return nil
}

func (m *MockMetricsConsumer) GetMetrics() []pmetric.Metrics {
	return m.metrics
}

func (m *MockMetricsConsumer) Reset() {
	m.metrics = make([]pmetric.Metrics, 0)
	m.errors = make([]error, 0)
}
