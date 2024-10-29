// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal"
)

var creationSet = receivertest.NewNopSettings()

type mockConfig struct{}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	tReceiver, err := factory.CreateTraces(context.Background(), creationSet, cfg, consumertest.NewNop())
	assert.Equal(t, err, pipeline.ErrSignalNotSupported)
	assert.Nil(t, tReceiver)

	mReceiver, err := factory.CreateMetrics(context.Background(), creationSet, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, mReceiver)

	tLogs, err := factory.CreateLogs(context.Background(), creationSet, cfg, consumertest.NewNop())
	assert.Equal(t, err, pipeline.ErrSignalNotSupported)
	assert.Nil(t, tLogs)
}

func TestCreateReceiver_ScraperKeyConfigError(t *testing.T) {
	const errorKey string = "error"

	factory := NewFactory()
	cfg := &Config{Scrapers: map[string]internal.Config{errorKey: &mockConfig{}}}

	_, err := factory.CreateMetrics(context.Background(), creationSet, cfg, consumertest.NewNop())
	assert.EqualError(t, err, fmt.Sprintf("failed to create scraper %q: factory not found for scraper %q", errorKey, errorKey))
}
