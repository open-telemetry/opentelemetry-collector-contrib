// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

type sqlServerScraper struct {
	logger           *zap.Logger
	config           *Config
	watcherRecorders []watcherRecorder
	mb               *metadata.MetricsBuilder
}

// watcherRecorder is a struct containing perf counter watcher along with corresponding value recorder.
type watcherRecorder struct {
	watcher  winperfcounters.PerfCounterWatcher
	recorder recordFunc
}

// curriedRecorder is a recorder function that already has value to be recorded,
// it needs metadata.MetricsBuilder and timestamp as arguments.
type curriedRecorder func(*metadata.MetricsBuilder, pcommon.Timestamp)

// newSQLServerScraper returns a new sqlServerScraper.
func newSQLServerScraper(params receiver.CreateSettings, cfg *Config) *sqlServerScraper {
	return &sqlServerScraper{
		logger: params.Logger,
		config: cfg,
		mb:     metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, params),
	}
}

// start creates and sets the watchers for the scraper.
func (s *sqlServerScraper) start(_ context.Context, _ component.Host) error {
	s.watcherRecorders = []watcherRecorder{}

	for _, pcr := range perfCounterRecorders {
		for perfCounterName, recorder := range pcr.recorders {
			perfCounterObj := defaultObjectName + ":" + pcr.object
			if s.config.InstanceName != "" {
				// The instance name must be preceded by "MSSQL$" to indicate that it is a named instance
				perfCounterObj = "\\" + s.config.ComputerName + "\\MSSQL$" + s.config.InstanceName + ":" + pcr.object
			}

			w, err := winperfcounters.NewWatcher(perfCounterObj, pcr.instance, perfCounterName)
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
func (s *sqlServerScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	recordersByDatabase, errs := recordersPerDatabase(s.watcherRecorders)

	for dbName, recorders := range recordersByDatabase {
		s.emitMetricGroup(recorders, dbName)
	}

	return s.mb.Emit(), errs
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
			dbName := counterValue.InstanceName

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
		recorder(s.mb, now)
	}

	rb := s.mb.NewResourceBuilder()
	if databaseName != "" {
		rb.SetSqlserverDatabaseName(databaseName)
	}
	if s.config.InstanceName != "" {
		rb.SetSqlserverComputerName(s.config.ComputerName)
		rb.SetSqlserverInstanceName(s.config.InstanceName)
	}
	s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

// shutdown stops all of the watchers for the scraper.
func (s sqlServerScraper) shutdown(_ context.Context) error {
	var errs error
	for _, wr := range s.watcherRecorders {
		err := wr.watcher.Close()
		if err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	return errs
}
