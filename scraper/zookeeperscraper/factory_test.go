// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zookeeperscraper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/scraper/zookeeperscraper/internal/metadata"
)

func TestFactory(t *testing.T) {
	f := NewFactory()
	require.Equal(t, metadata.Type, f.Type())

	cfg := f.CreateDefaultConfig()
	// Assert defaults.
	assert.Equal(t, "localhost:2181", cfg.(*Config).Endpoint)

	r, err := f.CreateMetrics(context.Background(), scrapertest.NewNopSettings(metadata.Type), cfg)

	require.NoError(t, err)
	require.NotNil(t, r)
}
