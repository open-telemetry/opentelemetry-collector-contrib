// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lokiexporter

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
)

const (
	validEndpoint = "http://loki:3100/loki/api/v1/push"
)

func TestExporter_new(t *testing.T) {
	t.Run("with valid config", func(t *testing.T) {
		config := &Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint: validEndpoint,
			},
		}
		exp := newExporter(config, componenttest.NewNopTelemetrySettings())
		require.NotNil(t, exp)
	})
}

func TestExporter_startReturnsNillWhenValidConfig(t *testing.T) {
	config := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: validEndpoint,
		},
	}
	exp := newExporter(config, componenttest.NewNopTelemetrySettings())
	require.NotNil(t, exp)
	require.NoError(t, exp.start(context.Background(), componenttest.NewNopHost()))
}

func TestExporter_startReturnsErrorWhenInvalidHttpClientSettings(t *testing.T) {
	config := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "",
			CustomRoundTripper: func(next http.RoundTripper) (http.RoundTripper, error) {
				return nil, fmt.Errorf("this causes HTTPClientSettings.ToClient() to error")
			},
		},
	}
	exp := newExporter(config, componenttest.NewNopTelemetrySettings())
	require.NotNil(t, exp)
	require.Error(t, exp.start(context.Background(), componenttest.NewNopHost()))
}

func TestExporter_stopAlwaysReturnsNil(t *testing.T) {
	config := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: validEndpoint,
		},
	}
	exp := newExporter(config, componenttest.NewNopTelemetrySettings())
	require.NotNil(t, exp)
	require.NoError(t, exp.stop(context.Background()))
}
