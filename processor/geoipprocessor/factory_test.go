// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package geoipprocessor

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default configuration")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateProcessor(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig()
	params := processortest.NewNopSettings()

	tp, err := factory.CreateTracesProcessor(context.Background(), params, cfg, consumertest.NewNop())
	assert.NotNil(t, tp)
	assert.NoError(t, err)

	mp, err := factory.CreateMetricsProcessor(context.Background(), params, cfg, consumertest.NewNop())
	assert.NotNil(t, mp)
	assert.NoError(t, err)

	lp, err := factory.CreateLogsProcessor(context.Background(), params, cfg, consumertest.NewNop())
	assert.NotNil(t, lp)
	assert.NoError(t, err)

	tp, err = factory.CreateTracesProcessor(context.Background(), params, cfg, consumertest.NewNop())
	assert.NotNil(t, tp)
	assert.NoError(t, err)

	mp, err = factory.CreateMetricsProcessor(context.Background(), params, cfg, consumertest.NewNop())
	assert.NotNil(t, mp)
	assert.NoError(t, err)

	lp, err = factory.CreateLogsProcessor(context.Background(), params, cfg, consumertest.NewNop())
	assert.NotNil(t, lp)
	assert.NoError(t, err)
}

func TestCreateProcessor_ProcessorKeyConfigError(t *testing.T) {
	const errorKey string = "error"

	factory := NewFactory()
	cfg := &Config{Providers: map[string]provider.Config{errorKey: &providerConfigMock{}}}

	_, err := factory.CreateMetricsProcessor(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.EqualError(t, err, fmt.Sprintf("geoIP provider factory not found for key: %q", errorKey))
}
