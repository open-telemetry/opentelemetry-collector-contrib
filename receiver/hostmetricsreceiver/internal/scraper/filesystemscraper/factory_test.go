// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filesystemscraper

import (
	"context"
	"testing"

	"github.com/shirou/gopsutil/v3/common"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := &Factory{}
	cfg := factory.CreateDefaultConfig()
	assert.IsType(t, &Config{}, cfg)
}

func TestCreateMetricsScraper(t *testing.T) {
	factory := &Factory{}
	cfg := &Config{}

	scraper, err := factory.CreateMetricsScraper(context.Background(), receivertest.NewNopCreateSettings(), cfg, common.EnvMap{})

	assert.NoError(t, err)
	assert.NotNil(t, scraper)
}

func TestCreateMetricsScraper_Error(t *testing.T) {
	factory := &Factory{}
	cfg := &Config{IncludeDevices: DeviceMatchConfig{Devices: []string{""}}}

	_, err := factory.CreateMetricsScraper(context.Background(), receivertest.NewNopCreateSettings(), cfg, common.EnvMap{})

	assert.Error(t, err)
}
