// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestType(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, metadata.Type, factory.Type())
}

func TestCreateTraces(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).Subscription = "projects/my-project/subscriptions/my-subscription"

	params := receivertest.NewNopSettings()
	tReceiver, err := factory.CreateTraces(context.Background(), params, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tReceiver, "traces receiver creation failed")
}

func TestCreateMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).Subscription = "projects/my-project/subscriptions/my-subscription"

	params := receivertest.NewNopSettings()
	tReceiver, err := factory.CreateMetrics(context.Background(), params, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tReceiver, "metrics receiver creation failed")
}

func TestCreateLogs(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).Subscription = "projects/my-project/subscriptions/my-subscription"

	params := receivertest.NewNopSettings()
	tReceiver, err := factory.CreateLogs(context.Background(), params, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tReceiver, "logs receiver creation failed")
}

func TestEnsureReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	cfg.(*Config).Subscription = "projects/my-project/subscriptions/my-subscription"
	tReceiver, err := factory.CreateTraces(context.Background(), receivertest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	mReceiver, err := factory.CreateMetrics(context.Background(), receivertest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.Equal(t, tReceiver, mReceiver)
	lReceiver, err := factory.CreateLogs(context.Background(), receivertest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.Equal(t, mReceiver, lReceiver)
}
