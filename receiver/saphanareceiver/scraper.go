// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package saphanareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/saphanareceiver"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/saphanareceiver/internal/metadata"
)

// Runs intermittently, fetching info from SAP HANA, creating metrics/datapoints,
// and feeding them to a metricsConsumer.
type sapHanaScraper struct {
	cfg     *Config
	mb      *metadata.MetricsBuilder
	factory sapHanaConnectionFactory
}

func newSapHanaScraper(settings receiver.CreateSettings, cfg *Config, factory sapHanaConnectionFactory) (scraperhelper.Scraper, error) {
	rs := &sapHanaScraper{
		cfg:     cfg,
		mb:      metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		factory: factory,
	}
	return scraperhelper.NewScraper(metadata.Type, rs.scrape)
}

func (s *sapHanaScraper) resourceMetricsBuilder(resourceAttributes map[string]string) (*metadata.ResourceMetricsBuilder, error) {
	rb := s.mb.NewResourceBuilder()
	rb.SetDbSystem("saphana")
	for attribute, value := range resourceAttributes {
		if attribute == "host" {
			rb.SetSaphanaHost(value)
		} else {
			return nil, fmt.Errorf("unsupported resource attribute: %s", attribute)
		}
	}
	return s.mb.ResourceMetricsBuilder(rb.Emit()), nil
}

// Scrape is called periodically, querying SAP HANA and building Metrics to send to
// the next consumer.
func (s *sapHanaScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	client := newSapHanaClient(s.cfg, s.factory)
	if err := client.Connect(ctx); err != nil {
		return pmetric.NewMetrics(), err
	}

	defer client.Close()

	errs := &scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now())

	for _, query := range queries {
		if query.Enabled == nil || query.Enabled(s.cfg) {
			query.CollectMetrics(ctx, s, client, now, errs)
		}
	}

	return s.mb.Emit(), errs.Combine()
}
