// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	_, err := factory.CreateMetricsReceiver(
		context.Background(),
		receiver.CreateSettings{
			TelemetrySettings: component.TelemetrySettings{
				TracerProvider: trace.NewNoopTracerProvider(),
				MeterProvider:  noop.NewMeterProvider(),
			},
		},
		factory.CreateDefaultConfig(),
		consumertest.NewNop(),
	)
	require.NoError(t, err)
}

func TestGetInstanceName(t *testing.T) {
	instanceName := getInstanceName("oracle://example.com:1521/mydb")
	assert.Equal(t, "example.com:1521/mydb", instanceName)
}

func TestGetDataSource(t *testing.T) {
	endpoint := "example1.com:1521"
	password := "password1"
	service := "mydb1"
	username := "username1"
	nonDefaultDataSource := fmt.Sprintf("oracle://%s:%s@%s/%s", username, password, endpoint, service)
	defaultDataSource := "oracle://username:password@example.com:1521/mydb"

	testCases := []struct {
		name     string
		config   *Config
		expected string
	}{
		{
			name: "Default data source",
			config: &Config{
				DataSource: defaultDataSource,
			},
			expected: defaultDataSource,
		},
		{
			name: "Default data source takes priority over other config options",
			config: &Config{
				DataSource: defaultDataSource,
				Endpoint:   endpoint,
				Password:   password,
				Service:    service,
				Username:   username,
			},
			expected: defaultDataSource,
		},
		{
			name: "Individual config options properly render data source",
			config: &Config{
				Endpoint: endpoint,
				Password: password,
				Service:  service,
				Username: username,
			},
			expected: nonDefaultDataSource,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dataSource := getDataSource(*tc.config)
			require.Equal(t, dataSource, tc.expected)
		})
	}
}
