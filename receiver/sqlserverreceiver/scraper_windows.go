// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

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

// SQL Server Perf Counter (PC) Scraper. This is used to scrape metrics from Windows Perf Counters.
type sqlServerPCScraper struct {
	logger           *zap.Logger
	config           *Config
	watcherRecorders []watcherRecorder
	mb               *metadata.MetricsBuilder
}

// watcherRecorder is a struct containing perf counter watcher along with corresponding value recorder.
type watcherRecorder struct {
	watcher     winperfcounters.PerfCounterWatcher
	recorder    recordFunc
	counterName string
}

// curriedRecorder is a recorder function that already has value to be recorded,
// it needs metadata.MetricsBuilder and timestamp as arguments.
type curriedRecorder func(*metadata.MetricsBuilder, pcommon.Timestamp)

// SQL Server perf counter names used to derive sqlserver.sql.recompilation.ratio.
const (
	sqlCompilationsCounter   = "SQL Compilations/sec"
	sqlReCompilationsCounter = "SQL Re-Compilations/sec"
)

// newSQLServerPCScraper returns a new sqlServerPCScraper.
func newSQLServerPCScraper(params receiver.Settings, cfg *Config) *sqlServerPCScraper {
	return &sqlServerPCScraper{
		logger: params.Logger,
		config: cfg,
		mb:     metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, params),
	}
}

// start creates and sets the watchers for the scraper.
func (s *sqlServerPCScraper) start(context.Context, component.Host) error {
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
			s.watcherRecorders = append(s.watcherRecorders, watcherRecorder{w, recorder, perfCounterName})
		}
	}

	return nil
}

// scrape collects windows performance counter data from all watchers and then records/emits it using the metricBuilder
func (s *sqlServerPCScraper) scrape(context.Context) (pmetric.Metrics, error) {
	recordersByDatabase, compByDB, recompByDB, errs := recordersPerDatabase(s.watcherRecorders)

	for dbName, recorders := range recordersByDatabase {
		s.emitMetricGroup(recorders, dbName, compByDB, recompByDB)
	}

	return s.mb.Emit(), errs
}

// recordersPerDatabase scrapes perf counter values using provided []watcherRecorder and returns
// a map of database name to curriedRecorder that includes the recorded value in its closure.
// It also returns per-database SQL Compilations/sec and SQL Re-Compilations/sec values so that
// the derived sqlserver.sql.recompilation.ratio metric can be computed at emit time.
func recordersPerDatabase(watcherRecorders []watcherRecorder) (map[string][]curriedRecorder, map[string]float64, map[string]float64, error) {
	var errs error

	dbToRecorders := make(map[string][]curriedRecorder)
	compByDB := make(map[string]float64)
	recompByDB := make(map[string]float64)
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

			switch wr.counterName {
			case sqlCompilationsCounter:
				compByDB[dbName] = val
			case sqlReCompilationsCounter:
				recompByDB[dbName] = val
			}

			if _, ok := dbToRecorders[dbName]; !ok {
				dbToRecorders[dbName] = []curriedRecorder{}
			}
			dbToRecorders[dbName] = append(dbToRecorders[dbName], func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp) {
				recorder(mb, ts, val)
			})
		}
	}

	return dbToRecorders, compByDB, recompByDB, errs
}

func (s *sqlServerPCScraper) emitMetricGroup(recorders []curriedRecorder, databaseName string, compByDB, recompByDB map[string]float64) {
	now := pcommon.NewTimestampFromTime(time.Now())

	for _, recorder := range recorders {
		recorder(s.mb, now)
	}

	// Derive sqlserver.sql.recompilation.ratio (recomp / comp * 100) when both
	// SQL Compilations/sec and SQL Re-Compilations/sec were observed for this
	// database/instance. Skip when comp is 0 to avoid division by zero.
	if comp, ok := compByDB[databaseName]; ok && comp > 0 {
		if recomp, ok := recompByDB[databaseName]; ok {
			s.mb.RecordSqlserverSQLRecompilationRatioDataPoint(now, recomp/comp*100)
		}
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
func (s sqlServerPCScraper) shutdown(context.Context) error {
	var errs error
	for _, wr := range s.watcherRecorders {
		err := wr.watcher.Close()
		if err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	return errs
}
