// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/confighttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitproviderreceiver/internal/metadata"
)

// TestConfig ensures a config created with the factory is the same as one created manually with
// the exported Config struct.
func TestConfig(t *testing.T) {
	factory := Factory{}
	defaultConfig := factory.CreateDefaultConfig()

	expectedConfig := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout: 15 * time.Second,
		},
	}

	assert.Equal(t, expectedConfig, defaultConfig)
}
