// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/apiconstants"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestConfig_Validate(t *testing.T) {
	t.Run("Empty configuration", func(t *testing.T) {
		c := &Config{}
		err := c.Validate()
		assert.NoError(t, err)

		assert.Equal(t, apiconstants.GetDefaultOneAgentEndpoint(), c.Endpoint, "Should use default OneAgent endpoint")
	})

	t.Run("Valid configuration", func(t *testing.T) {
		c := &Config{HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: "http://example.com/"}, APIToken: "token"}
		err := c.Validate()
		assert.NoError(t, err)

		assert.Equal(t, "http://example.com/", c.Endpoint, "Should use provided endpoint")
	})

	t.Run("Invalid Endpoint", func(t *testing.T) {
		c := &Config{HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: "example.com"}}
		err := c.Validate()
		assert.Error(t, err)
	})

	t.Run("Invalid QueueSettings", func(t *testing.T) {
		c := &Config{QueueSettings: exporterhelper.QueueSettings{QueueSize: -1, Enabled: true}}
		err := c.Validate()
		assert.Error(t, err)
	})
}
