// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsekshyperpodreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsekshyperpodreceiver/internal/metadata"
)

func TestFactory_CreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	rCfg, ok := cfg.(*Config)
	require.True(t, ok, "expected *Config type from CreateDefaultConfig")

	assert.Equal(t, 60*time.Second, rCfg.CollectionInterval)
	assert.Empty(t, rCfg.ClusterName)
}

func TestFactory_CreateMetricsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ControllerConfig = scraperhelper.NewDefaultControllerConfig()
	cfg.CollectionInterval = 60 * time.Second

	set := receivertest.NewNopSettings(metadata.Type)
	receiver, err := factory.CreateMetrics(t.Context(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, receiver)
}
