// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
)

type splunkScraper struct {
    splunkClient *splunkEntClient
    settings     component.TelemetrySettings
    conf         *Config
    mb           *metadata.MetricsBuilder
}

func newSplunkMetricsScraper(params receiver.CreateSettings, cfg *Config) splunkScraper {
    return splunkScraper{
        settings: params.TelemetrySettings,
        conf:     cfg,
        mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, params),
    }
}

// Create a client instance and add to the splunkScraper
func (s *splunkScraper) start(_ context.Context, _ component.Host) (err error) {
    return nil 
}

// The big one: Describes how all scraping tasks should be performed. Part of the scraper interface
func (s *splunkScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
    errs := &scrapererror.ScrapeErrors{}
    return s.mb.Emit(), errs.Combine()
}

