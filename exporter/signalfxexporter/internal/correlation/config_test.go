// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package correlation

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
)

func TestValidConfig(t *testing.T) {
	config := DefaultConfig()
	config.Endpoint = "https://localhost"
	require.NoError(t, config.validate())
}

func TestInvalidConfig(t *testing.T) {
	invalid := Config{}
	noEndpointErr := invalid.validate()
	require.Error(t, noEndpointErr)

	clientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfig.MaxIdleConns = 0
	clientConfig.IdleConnTimeout = 0
	clientConfig.ForceAttemptHTTP2 = false
	clientConfig.Endpoint = ":123:456"
	invalid = Config{
		ClientConfig: clientConfig,
	}
	invalidURLErr := invalid.validate()
	require.Error(t, invalidURLErr)
}
