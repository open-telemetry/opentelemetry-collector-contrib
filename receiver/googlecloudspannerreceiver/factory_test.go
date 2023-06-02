// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudspannerreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
}

func TestType(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, component.Type(metadata.Type), factory.Type())
}

func TestCreateMetricsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	receiverConfig := cfg.(*Config)

	receiver, err := factory.CreateMetricsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		receiverConfig,
		consumertest.NewNop(),
	)

	assert.NoError(t, err)
	assert.NotNil(t, receiver, "failed to create metrics receiver")

	_, err = factory.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), receiverConfig, nil)
	require.Error(t, err)
}
