// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitproviderreceiver

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitproviderreceiver/internal"
)

var creationSet = receivertest.NewNopCreateSettings()

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

	tReceiver, err := factory.CreateTracesReceiver(context.Background(), creationSet, cfg, consumertest.NewNop())
	assert.Equal(t, err, component.ErrDataTypeIsNotSupported)
	assert.Nil(t, tReceiver)

	mReceiver, err := factory.CreateMetricsReceiver(context.Background(), creationSet, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, mReceiver)

	tLogs, err := factory.CreateLogsReceiver(context.Background(), creationSet, cfg, consumertest.NewNop())
	assert.Equal(t, err, component.ErrDataTypeIsNotSupported)
	assert.Nil(t, tLogs)
}

func TestCreateReceiver_ScraperKeyConfigError(t *testing.T) {
	const errorKey string = "error"

	factory := NewFactory()
	cfg := &Config{Scrapers: map[string]internal.Config{errorKey: &mockConfig{}}}

	_, err := factory.CreateMetricsReceiver(context.Background(), creationSet, cfg, consumertest.NewNop())
	assert.EqualError(t, err, fmt.Sprintf("git provider scraper factory not found for key: %q", errorKey))
}
