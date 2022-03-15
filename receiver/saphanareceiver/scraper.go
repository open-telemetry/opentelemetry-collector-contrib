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

const instrumentationLibraryName = "otelcol/saphana"

// Runs intermittently, fetching info from SAP HANA, creating metrics/datapoints,
// and feeding them to a metricsConsumer.
type sapHanaScraper struct {
	settings component.ReceiverCreateSettings
	cfg      *Config
	mb       *metadata.MetricsBuilder
	uptime   time.Duration
}

func newSapHanaScraper(settings component.ReceiverCreateSettings, cfg *Config) (scraperhelper.Scraper, error) {
	rs := &sapHanaScraper{
		settings: settings,
		cfg:      cfg,
		mb:       metadata.NewMetricsBuilder(cfg.Metrics),
	}
	return scraperhelper.NewScraper(typeStr, rs.scrape)
}

// Scrape is called periodically, querying SAP HANA and building Metrics to send to
// the next consumer.
func (s *sapHanaScraper) scrape(ctx context.Context) (pdata.Metrics, error) {
	metrics := pdata.NewMetrics()

	client := newSapHanaClient(s.cfg)
	if err := client.Connect(); err != nil {
		return metrics, err
	}

	errs := &scrapererror.ScrapeErrors{}
	now := pdata.NewTimestampFromTime(time.Now())

	ilms := metrics.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty()
	ilms.InstrumentationLibrary().SetName(instrumentationLibraryName)

	if err := s.collectColumnMemoryMetric(ctx, client, now, ilms); err != nil {
		errs.AddPartial(1, err)
	}

	return metrics, errs.Combine()
}

func (s *sapHanaScraper) collectColumnMemoryMetric(ctx context.Context, client sapHanaClient, now pdata.Timestamp, metrics pdata.InstrumentationLibraryMetricsSlice) error {
	if rows, err := client.collectStatsFromQuery(
		ctx,
		"SELECT HOST as host, SUM(MEMORY_SIZE_IN_MAIN) as main, SUM(MEMORY_SIZE_IN_DELTA) as delta FROM M_CS_ALL_COLUMNS GROUP BY HOST",
		[]string{"host"},
		"main", "delta",
	); err != nil {
		return err
	} else {
		for _, data := range rows {
			s.mb.RecordSaphanaColumnMemoryUsedDataPoint(now, data.stats["main"], data.labels["host"], metadata.AttributeColumnMemoryType.Main)
			s.mb.RecordSaphanaColumnMemoryUsedDataPoint(now, data.stats["delta"], data.labels["host"], metadata.AttributeColumnMemoryType.Delta)
		}
	}

	return nil
}
