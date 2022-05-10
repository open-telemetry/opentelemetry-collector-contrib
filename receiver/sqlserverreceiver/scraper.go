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

//go:build windows
// +build windows

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

type sqlServerScraper struct {
	logger           *zap.Logger
	config           *Config
	watcherRecorders []watcherRecorder
	metricsBuilder   *metadata.MetricsBuilder
}

// watcherRecorder is a struct containing perf counter watcher along with corresponding value recorder.
type watcherRecorder struct {
	watcher  winperfcounters.PerfCounterWatcher
	recorder recordFunc
}

// curriedRecorder is a recorder function that already has value to be recorded,
// it needs metadata.MetricsBuilder and timestamp as arguments.
type curriedRecorder func(*metadata.MetricsBuilder, pcommon.Timestamp)

// newSqlServerScraper returns a new sqlServerScraper.
func newSqlServerScraper(logger *zap.Logger, cfg *Config) *sqlServerScraper {
	metricsBuilder := metadata.NewMetricsBuilder(cfg.Metrics)
	return &sqlServerScraper{logger: logger, config: cfg, metricsBuilder: metricsBuilder}
}

// start creates and sets the watchers for the scraper.
func (s *sqlServerScraper) start(ctx context.Context, host component.Host) error {
	s.watcherRecorders = []watcherRecorder{}

	for _, pcr := range perfCounterRecorders {
		for perfCounterName, recorder := range pcr.recorders {
			w, err := winperfcounters.NewWatcher(pcr.object, pcr.instance, perfCounterName)
			if err != nil {
				s.logger.Warn(err.Error())
				continue
			}
			s.watcherRecorders = append(s.watcherRecorders, watcherRecorder{w, recorder})
		}
	}

	return nil
}

// scrape collects windows performance counter data from all watchers and then records/emits it using the metricBuilder
func (s *sqlServerScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	recordersByDatabase, errs := recordersPerDatabase(s.watcherRecorders)

	for dbName, recorders := range recordersByDatabase {
		s.emitMetricGroup(recorders, dbName)
	}

	return s.metricsBuilder.Emit(), errs
}

// recordersPerDatabase scrapes perf counter values using provided []watcherRecorder and returns
// a map of database name to curriedRecorder that includes the recorded value in its closure.
func recordersPerDatabase(watcherRecorders []watcherRecorder) (map[string][]curriedRecorder, error) {
	var errs error

	dbToRecorders := make(map[string][]curriedRecorder)
	for _, wr := range watcherRecorders {
		counterValues, err := wr.watcher.ScrapeData()
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}

		for _, counterValue := range counterValues {
			dbName := counterValue.Attributes["instance"]

			// it's important to initialize new values for the closure.
			val := counterValue.Value
			recorder := wr.recorder

			if _, ok := dbToRecorders[dbName]; !ok {
				dbToRecorders[dbName] = []curriedRecorder{}
			}
			dbToRecorders[dbName] = append(dbToRecorders[dbName], func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp) {
				recorder(mb, ts, val)
			})
		}
	}

	return dbToRecorders, errs
}

func (s *sqlServerScraper) emitMetricGroup(recorders []curriedRecorder, databaseName string) {
	now := pcommon.NewTimestampFromTime(time.Now())

	for _, recorder := range recorders {
		recorder(s.metricsBuilder, now)
	}

	if databaseName != "" {
		s.metricsBuilder.EmitForResource(
			metadata.WithSqlserverDatabaseName(databaseName),
		)
	} else {
		s.metricsBuilder.EmitForResource()
	}
}

// shutdown stops all of the watchers for the scraper.
func (s sqlServerScraper) shutdown(ctx context.Context) error {
	var errs error
	for _, wr := range s.watcherRecorders {
		err := wr.watcher.Close()
		if err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	return errs
}
