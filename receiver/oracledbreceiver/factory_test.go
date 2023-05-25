// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver

import (
	"context"
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
