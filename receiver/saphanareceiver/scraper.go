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
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/saphanareceiver/internal/metadata"
)

const instrumentationLibraryName = "otelcol/saphana"

// Runs intermittently, fetching info from SAP HANA, creating metrics/datapoints,
// and feeding them to a metricsConsumer.
type sapHanaScraper struct {
	settings component.TelemetrySettings
	cfg      *Config
	mb       *metadata.MetricsBuilder
	uptime   time.Duration
}

func newSapHanaScraper(settings component.TelemetrySettings, cfg *Config) (scraperhelper.Scraper, error) {
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

	if err := s.collectColumnMemoryMetric(ctx, client, now); err != nil {
		errs.AddPartial(1, err)
	}

	s.mb.Emit(ilms.Metrics())

	return metrics, errs.Combine()
}

func (s *sapHanaScraper) collectColumnMemoryMetric(ctx context.Context, client client, now pdata.Timestamp) error {
	if rows, err := client.collectStatsFromQuery(
		ctx,
		"SELECT HOST as host, SUM(MEMORY_SIZE_IN_MAIN) as main, SUM(MEMORY_SIZE_IN_DELTA) as delta FROM M_CS_ALL_COLUMNS GROUP BY HOST",
		[]string{"host"},
		"main", "delta",
	); err != nil {
		return err
	} else {
		for _, data := range rows {
			host := data.labels["host"]
			if i, ok := s.parseInt("main", data.stats["main"]); ok {
				s.mb.RecordSaphanaColumnMemoryUsedDataPoint(now, i, host, metadata.AttributeColumnMemoryType.Main)
			}
			if i, ok := s.parseInt("delta", data.stats["delta"]); ok {
				s.mb.RecordSaphanaColumnMemoryUsedDataPoint(now, i, host, metadata.AttributeColumnMemoryType.Delta)
			}
		}
	}

	return nil
}

// parseInt converts string to int64.
func (s *sapHanaScraper) parseInt(key, value string) (int64, bool) {
	i, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		s.logInvalid("int", key, value)
		return 0, false
	}
	return i, true
}

func (s *sapHanaScraper) logInvalid(expectedType, key, value string) {
	s.settings.Logger.Info(
		"invalid value",
		zap.String("expectedType", expectedType),
		zap.String("key", key),
		zap.String("value", value),
	)
}
