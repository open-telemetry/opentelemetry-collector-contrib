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
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
	windowsapi "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

type sqlServerScraper struct {
	logger         *zap.Logger
	config         *Config
	watchers       []winperfcounters.PerfCounterWatcher
	metricsBuilder *metadata.MetricsBuilder
}

// newSqlServerScraper returns a new sqlServerScraper.
func newSqlServerScraper(logger *zap.Logger, cfg *Config) *sqlServerScraper {
	metricsBuilder := metadata.NewMetricsBuilder(cfg.Metrics)
	return &sqlServerScraper{logger: logger, config: cfg, metricsBuilder: metricsBuilder}
}

// start creates and sets the watchers for the scraper.
func (s *sqlServerScraper) start(ctx context.Context, host component.Host) error {
	watchers := []winperfcounters.PerfCounterWatcher{}
	for _, objCfg := range createWatcherConfigs() {
		objWatchers, err := objCfg.BuildPaths()
		if err != nil {
			s.logger.Warn("some performance counters could not be initialized", zap.Error(err))
		}
		for _, objWatcher := range objWatchers {
			watchers = append(watchers, objWatcher)
		}
	}
	s.watchers = watchers

	return nil
}

// scrape collects windows performance counter data from all watchers and then records/emits it using the metricBuilder
func (s *sqlServerScraper) scrape(ctx context.Context) (pdata.Metrics, error) {
	metricsByDatabase, errs := createMetricGroupPerDatabase(s.watchers)

	for key, metricGroup := range metricsByDatabase {
		s.emitMetricGroup(metricGroup, key)
	}

	return s.metricsBuilder.Emit(), errs
}

func createMetricGroupPerDatabase(watchers []windowsapi.PerfCounterWatcher) (map[string][]winperfcounters.CounterValue, error) {
	var errs error

	metricsByDatabase := map[string][]winperfcounters.CounterValue{}
	for _, watcher := range watchers {
		counterValues, err := watcher.ScrapeData()
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		for _, counterValue := range counterValues {
			key := counterValue.Attributes["instance"]

			if metricsByDatabase[key] == nil {
				metricsByDatabase[key] = []winperfcounters.CounterValue{counterValue}
			} else {
				metricsByDatabase[key] = append(metricsByDatabase[key], counterValue)
			}
		}
	}

	return metricsByDatabase, errs
}

func (s *sqlServerScraper) emitMetricGroup(metricGroup []winperfcounters.CounterValue, databaseName string) {
	now := pdata.NewTimestampFromTime(time.Now())

	for _, metric := range metricGroup {
		s.metricsBuilder.RecordAnyDataPoint(now, metric.Value, metric.MetricRep.Name, metric.MetricRep.Attributes)
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
	for _, watcher := range s.watchers {
		err := watcher.Close()
		if err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	return errs
}

// createWatcherConfigs returns established performance counter configs for each metric.
func createWatcherConfigs() []windowsapi.ObjectConfig {
	return []windowsapi.ObjectConfig{
		{
			Object: "SQLServer:General Statistics",
			Counters: []windowsapi.CounterConfig{
				{
					MetricRep: windowsapi.MetricRep{
						Name: "sqlserver.user.connection.count",
					},
					Name: "User Connections",
				},
			},
		},
		{
			Object: "SQLServer:SQL Statistics",
			Counters: []windowsapi.CounterConfig{
				{
					MetricRep: windowsapi.MetricRep{
						Name: "sqlserver.batch.request.rate",
					},

					Name: "Batch Requests/sec",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "sqlserver.batch.sql_compilation.rate",
					},

					Name: "SQL Compilations/sec",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "sqlserver.batch.sql_recompilation.rate",
					},
					Name: "SQL Re-Compilations/sec",
				},
			},
		},
		{
			Object:    "SQLServer:Locks",
			Instances: []string{"_Total"},
			Counters: []windowsapi.CounterConfig{
				{
					MetricRep: windowsapi.MetricRep{
						Name: "sqlserver.lock.wait.rate",
					},
					Name: "Lock Waits/sec",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "sqlserver.lock.wait_time.avg",
					},
					Name: "Average Wait Time (ms)",
				},
			},
		},
		{
			Object: "SQLServer:Buffer Manager",
			Counters: []windowsapi.CounterConfig{
				{
					MetricRep: windowsapi.MetricRep{
						Name: "sqlserver.page.buffer_cache.hit_ratio",
					},
					Name: "Buffer cache hit ratio",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "sqlserver.page.checkpoint.flush.rate",
					},
					Name: "Checkpoint pages/sec",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "sqlserver.page.lazy_write.rate",
					},
					Name: "Lazy Writes/sec",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "sqlserver.page.life_expectancy",
					},
					Name: "Page life expectancy",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "sqlserver.page.operation.rate",
						Attributes: map[string]string{
							"type": "read",
						},
					},
					Name: "Page reads/sec",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "sqlserver.page.operation.rate",
						Attributes: map[string]string{
							"type": "write",
						},
					},
					Name: "Page writes/sec",
				},
			},
		},
		{
			Object:    "SQLServer:Access Methods",
			Instances: []string{"_Total"},
			Counters: []windowsapi.CounterConfig{
				{
					MetricRep: windowsapi.MetricRep{
						Name: "sqlserver.page.split.rate",
					},
					Name: "Page Splits/sec",
				},
			},
		},
		{
			Object:    "SQLServer:Databases",
			Instances: []string{"*"},
			Counters: []windowsapi.CounterConfig{
				{
					MetricRep: windowsapi.MetricRep{
						Name: "sqlserver.transaction_log.flush.data.rate",
					},
					Name: "Log Bytes Flushed/sec",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "sqlserver.transaction_log.flush.rate",
					},
					Name: "Log Flushes/sec",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "sqlserver.transaction_log.flush.wait.rate",
					},
					Name: "Log Flush Waits/sec",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "sqlserver.transaction_log.growth.count",
					},
					Name: "Log Growths",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "sqlserver.transaction_log.shrink.count",
					},
					Name: "Log Shrinks",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "sqlserver.transaction_log.usage",
					},
					Name: "Percent Log Used",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "sqlserver.transaction.rate",
					},
					Name: "Transactions/sec",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "sqlserver.transaction.write.rate",
					},
					Name: "Write Transactions/sec",
				},
			},
		},
	}
}
