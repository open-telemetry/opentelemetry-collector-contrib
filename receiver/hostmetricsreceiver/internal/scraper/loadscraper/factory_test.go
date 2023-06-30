// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadscraper

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
