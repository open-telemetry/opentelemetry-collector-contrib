// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/confighttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/metadata"
)

// TestConfig ensures a config created with the factory is the same as one created manually with
// the exported Config struct.
func TestConfig(t *testing.T) {
	factory := Factory{}
	defaultConfig := factory.CreateDefaultConfig()

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Timeout = 15 * time.Second

	expectedConfig := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		ClientConfig:         clientConfig,
	}

	assert.Equal(t, expectedConfig, defaultConfig)
}
