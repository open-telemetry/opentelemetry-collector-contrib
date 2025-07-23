// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package iisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver"

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	t.Run("NewFactoryCorrectType", func(t *testing.T) {
		factory := NewFactory()
		require.Equal(t, metadata.Type, factory.Type())
	})

	t.Run("NewFactoryDefaultConfig", func(t *testing.T) {
		factory := NewFactory()

		var expectedCfg component.Config = &Config{
			ControllerConfig: scraperhelper.ControllerConfig{
				CollectionInterval: 60 * time.Second,
				InitialDelay:       time.Second,
			},
			MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		}

		require.Equal(t, expectedCfg, factory.CreateDefaultConfig())
	})
}
