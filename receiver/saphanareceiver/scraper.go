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
	"fmt"
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

type queryStat struct {
	key                     string
	dataType                string
	addIntMetricFunction    func(*sapHanaScraper, pdata.Timestamp, int64, map[string]string)
	addDoubleMetricFunction func(*sapHanaScraper, pdata.Timestamp, float64, map[string]string)
}

func (q *queryStat) collectStat(s *sapHanaScraper, now pdata.Timestamp, row map[string]string) error {
	switch q.dataType {
	case "int":
		if i, ok := s.parseInt(q.key, row[q.key]); ok {
			q.addIntMetricFunction(s, now, i, row)
		} else {
			return fmt.Errorf("Unable to parse '%s' as an integer for query key %s", row[q.key], q.key)
		}
	case "double":
		if f, ok := s.parseDouble(q.key, row[q.key]); ok {
			q.addDoubleMetricFunction(s, now, f, row)
		} else {
			return fmt.Errorf("Unable to parse '%s' as a double for query key %s", row[q.key], q.key)
		}
	default:
		return fmt.Errorf("Incorrectly configured query, type provided must be 'int' or 'double' but was %s", q.dataType)
	}
	return nil
}

type monitoringQuery struct {
	query         string
	orderedLabels []string
	orderedStats  []queryStat
}

var queries = []monitoringQuery{
	{
		query:         "SELECT HOST as host, SUM(MEMORY_SIZE_IN_MAIN) as main, SUM(MEMORY_SIZE_IN_DELTA) as delta FROM M_CS_ALL_COLUMNS GROUP BY HOST",
		orderedLabels: []string{"host"},
		orderedStats: []queryStat{
			{
				key:      "main",
				dataType: "int",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaColumnMemoryUsedDataPoint(now, i, row["host"], metadata.AttributeColumnMemoryType.Main)
				},
			},
			{
				key:      "delta",
				dataType: "int",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaColumnMemoryUsedDataPoint(now, i, row["host"], metadata.AttributeColumnMemoryType.Delta)
				},
			},
		},
	},
}

func (m *monitoringQuery) columns() []string {
	output := make([]string, len(m.orderedLabels))
	copy(output, m.orderedLabels)
	for _, stat := range m.orderedStats {
		output = append(output, stat.key)
	}
	return output
}

func (m *monitoringQuery) CollectMetrics(s *sapHanaScraper, ctx context.Context, client client, now pdata.Timestamp) error {
	if rows, err := client.collectDataFromQuery(ctx, m.query, m.columns()); err != nil {
		return err
	} else {
		for _, data := range rows {
			for _, stat := range m.orderedStats {
				stat.collectStat(s, now, data)
			}
		}
	}

	return nil
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

	for _, query := range queries {
		if err := query.CollectMetrics(s, ctx, client, now); err != nil {
			errs.AddPartial(len(query.orderedStats), err)
		}
	}

	s.mb.Emit(ilms.Metrics())

	return metrics, errs.Combine()
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

// parseDouble converts string to float64.
func (s *sapHanaScraper) parseDouble(key, value string) (float64, bool) {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		s.logInvalid("float", key, value)
		return 0, false
	}
	return f, true
}

func (s *sapHanaScraper) logInvalid(expectedType, key, value string) {
	s.settings.Logger.Info(
		"invalid value",
		zap.String("expectedType", expectedType),
		zap.String("key", key),
		zap.String("value", value),
	)
}
