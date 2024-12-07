package valkeyreceiver

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestScrape(t *testing.T) {
	settings := receivertest.NewNopSettings()
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:6379"
	scraper, err := newValkeyScraper(cfg, settings)
	require.NoError(t, err)

	metrics, err := scraper.Scrape(context.Background())
	require.NoError(t, err)

	fmt.Printf("%#v", metrics)
	fmt.Printf("Total metrics: %#v", metrics.MetricCount())
	fmt.Printf("Total scope metrics: %#v", metrics.ResourceMetrics().At(0).ScopeMetrics().Len())
}
