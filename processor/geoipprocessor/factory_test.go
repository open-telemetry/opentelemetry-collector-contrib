// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package geoipprocessor

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider"
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

	_, err = factory.CreateLogsProcessor(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.EqualError(t, err, fmt.Sprintf("geoIP provider factory not found for key: %q", errorKey))

	_, err = factory.CreateTracesProcessor(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.EqualError(t, err, fmt.Sprintf("geoIP provider factory not found for key: %q", errorKey))
}

func TestCreateProcessor_FailedProvider(t *testing.T) {
	baseMockFactory.CreateGeoIPProviderF = func(context.Context, processor.Settings, provider.Config) (provider.GeoIPProvider, error) {
		return nil, errors.New("error creating provider")
	}

	const providerKey string = "mock"
	providerFactories[providerKey] = &baseMockFactory

	factory := NewFactory()
	cfg := &Config{Providers: map[string]provider.Config{providerKey: &providerConfigMock{}}}

	_, err := factory.CreateMetricsProcessor(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.EqualError(t, err, fmt.Errorf("failed to create provider for key %q: %w", providerKey, errors.New("error creating provider")).Error())
}
