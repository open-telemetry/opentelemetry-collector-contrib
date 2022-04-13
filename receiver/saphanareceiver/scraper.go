// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package saphanareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/saphanareceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/saphanareceiver/internal/metadata"
)

// Runs intermittently, fetching info from SAP HANA, creating metrics/datapoints,
// and feeding them to a metricsConsumer.
type sapHanaScraper struct {
	settings component.TelemetrySettings
	cfg      *Config
	mb       *metadata.MetricsBuilder
	factory  sapHanaConnectionFactory
}

func newSapHanaScraper(settings component.TelemetrySettings, cfg *Config, factory sapHanaConnectionFactory) (scraperhelper.Scraper, error) {
	rs := &sapHanaScraper{
		settings: settings,
		cfg:      cfg,
		mb:       metadata.NewMetricsBuilder(cfg.Metrics),
		factory:  factory,
	}
	return scraperhelper.NewScraper(typeStr, rs.scrape)
}

// Scrape is called periodically, querying SAP HANA and building Metrics to send to
// the next consumer.
func (s *sapHanaScraper) scrape(ctx context.Context) (pdata.Metrics, error) {
	client := newSapHanaClient(s.cfg, s.factory)
	if err := client.Connect(ctx); err != nil {
		return pdata.NewMetrics(), err
	}

	defer client.Close()

	errs := &scrapererror.ScrapeErrors{}
	now := pdata.NewTimestampFromTime(time.Now())

	for _, query := range queries {
		if query.Enabled == nil || query.Enabled(s.cfg) {
			if err := query.CollectMetrics(ctx, s, client, now); err != nil {
				errs.AddPartial(len(query.orderedStats), err)
			}
		}
	}

	return s.mb.Emit(), errs.Combine()
}
