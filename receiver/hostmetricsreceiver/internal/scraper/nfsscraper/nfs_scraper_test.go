// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nfsscraper

import (
	"context"
	"testing"

	//	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper/internal/metadata"
)

func TestScrape(t *testing.T) {
	if !supportedOS {
		t.Skip()
	}

	ctx := context.Background()

	s := newNfsScraper(ctx, scrapertest.NewNopSettings(metadata.Type), &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	})

	_, err := s.scrape(ctx)
	require.NoErrorf(t, err, "scrape error %+v", err)

	//	assert.Equal(t, 0, metrics.MetricCount())
	//	assert.Equal(t, 0, metrics.DataPointCount())
}
