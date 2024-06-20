// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lokiexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
)

const (
	validEndpoint = "http://loki:3100/loki/api/v1/push"
)

func TestExporter_new(t *testing.T) {
	t.Run("with valid config", func(t *testing.T) {
		config := &Config{
			ClientConfig: confighttp.ClientConfig{
				Endpoint: validEndpoint,
			},
		}
		exp, err := newExporter(config, componenttest.NewNopTelemetrySettings())
		require.NoError(t, err)
		require.NotNil(t, exp)
	})
}

func TestExporter_startReturnsNillWhenValidConfig(t *testing.T) {
	config := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: validEndpoint,
		},
	}
	exp, err := newExporter(config, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	require.NotNil(t, exp)
	require.NoError(t, exp.start(context.Background(), componenttest.NewNopHost()))
}

func TestExporter_startReturnsErrorWhenInvalidHttpClientSettings(t *testing.T) {
	config := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "",
			TLSSetting: configtls.ClientConfig{
				Config: configtls.Config{
					MinVersion: "invalid",
				},
			},
		},
	}
	exp, err := newExporter(config, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	require.NotNil(t, exp)
	require.Error(t, exp.start(context.Background(), componenttest.NewNopHost()))
}

func TestExporter_stopAlwaysReturnsNil(t *testing.T) {
	config := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: validEndpoint,
		},
	}
	exp, err := newExporter(config, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	require.NotNil(t, exp)
	require.NoError(t, exp.stop(context.Background()))
}
