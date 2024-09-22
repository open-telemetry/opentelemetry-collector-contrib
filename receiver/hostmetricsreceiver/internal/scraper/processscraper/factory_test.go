// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processscraper

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := &Factory{}
	cfg := factory.CreateDefaultConfig()
	assert.IsType(t, &Config{}, cfg)
}

func TestCreateResourceMetricsScraper(t *testing.T) {
	factory := &Factory{}
	cfg := &Config{}

	scraper, err := factory.CreateMetricsScraper(context.Background(), receivertest.NewNopSettings(), cfg)

	if runtime.GOOS == "linux" || runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		require.NoError(t, err)
		assert.NotNil(t, scraper)
	} else {
		require.Error(t, err)
		assert.Nil(t, scraper)
	}
}
