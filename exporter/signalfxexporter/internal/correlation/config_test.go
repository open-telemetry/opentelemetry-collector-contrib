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
	config.HTTPClientSettings.Endpoint = "https://localhost"
	require.NoError(t, config.validate())
}

func TestInvalidConfig(t *testing.T) {
	invalid := Config{}
	noEndpointErr := invalid.validate()
	require.Error(t, noEndpointErr)

	invalid = Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ":123:456"},
	}
	invalidURLErr := invalid.validate()
	require.Error(t, invalidURLErr)
}
