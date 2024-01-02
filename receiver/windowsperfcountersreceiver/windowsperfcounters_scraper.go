// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package windowsperfcountersreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
)

const instanceLabelName = "instance"

type perfCounterMetricWatcher struct {
	winperfcounters.PerfCounterWatcher
	MetricRep
}

type newWatcherFunc func(string, string, string) (winperfcounters.PerfCounterWatcher, error)

// scraper is the type that scrapes various host metrics.
type scraper struct {
	cfg      *Config
	settings component.TelemetrySettings
	watchers []perfCounterMetricWatcher

	// for mocking
	newWatcher newWatcherFunc
}

func newScraper(cfg *Config, settings component.TelemetrySettings) *scraper {
	return &scraper{cfg: cfg, settings: settings, newWatcher: winperfcounters.NewWatcher}
}

func (s *scraper) start(context.Context, component.Host) error {
	watchers, err := s.initWatchers()
	if err != nil {
		s.settings.Logger.Warn("some performance counters could not be initialized", zap.Error(err))
	}
	s.watchers = watchers
	return nil
}

func (s *scraper) initWatchers() ([]perfCounterMetricWatcher, error) {
	var errs error
	var watchers []perfCounterMetricWatcher

	for _, objCfg := range s.cfg.PerfCounters {
		for _, instance := range instancesFromConfig(objCfg) {
			for _, counterCfg := range objCfg.Counters {
				pcw, err := s.newWatcher(objCfg.Object, instance, counterCfg.Name)
				if err != nil {
					errs = multierr.Append(errs, err)
					continue
				}

				watcher := perfCounterMetricWatcher{
					PerfCounterWatcher: pcw,
					MetricRep:          MetricRep{Name: pcw.Path()},
				}
				if counterCfg.MetricRep.Name != "" {
					watcher.MetricRep.Name = counterCfg.MetricRep.Name
					if counterCfg.MetricRep.Attributes != nil {
						watcher.MetricRep.Attributes = counterCfg.MetricRep.Attributes
					}
				}

				watchers = append(watchers, watcher)
			}
		}
	}

	return watchers, errs
}

func (s *scraper) shutdown(context.Context) error {
	var errs error
	for _, watcher := range s.watchers {
		err := watcher.Close()
		if err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	return errs
}

func (s *scraper) scrape(context.Context) (pmetric.Metrics, error) {
	md := pmetric.NewMetrics()
	metricSlice := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	now := pcommon.NewTimestampFromTime(time.Now())
	var errs error

	metricSlice.EnsureCapacity(len(s.watchers))
	metrics := map[string]pmetric.Metric{}
	for name, metricCfg := range s.cfg.MetricMetaData {
		builtMetric := metricSlice.AppendEmpty()

		builtMetric.SetName(name)
		builtMetric.SetDescription(metricCfg.Description)
		builtMetric.SetUnit(metricCfg.Unit)

		if (metricCfg.Sum != SumMetric{}) {
			builtMetric.SetEmptySum().SetIsMonotonic(metricCfg.Sum.Monotonic)

			switch metricCfg.Sum.Aggregation {
			case "cumulative":
				builtMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			case "delta":
				builtMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
			}
		} else {
			builtMetric.SetEmptyGauge()
		}

		metrics[name] = builtMetric
	}

	for _, watcher := range s.watchers {
		counterVals, err := watcher.ScrapeData()
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}

		for _, val := range counterVals {
			var metric pmetric.Metric
			if builtmetric, ok := metrics[watcher.MetricRep.Name]; ok {
				metric = builtmetric
			} else {
				metric = metricSlice.AppendEmpty()
				metric.SetName(watcher.MetricRep.Name)
				metric.SetUnit("1")
				metric.SetEmptyGauge()
			}

			initializeMetricDps(metric, now, val, watcher.MetricRep.Attributes)
		}
	}
	return md, errs
}

func initializeMetricDps(metric pmetric.Metric, now pcommon.Timestamp, counterValue winperfcounters.CounterValue,
	attributes map[string]string) {
	var dps pmetric.NumberDataPointSlice

	if metric.Type() == pmetric.MetricTypeGauge {
		dps = metric.Gauge().DataPoints()
	} else {
		dps = metric.Sum().DataPoints()
	}

	dp := dps.AppendEmpty()
	if counterValue.InstanceName != "" {
		dp.Attributes().PutStr(instanceLabelName, counterValue.InstanceName)
	}
	if attributes != nil {
		for attKey, attVal := range attributes {
			dp.Attributes().PutStr(attKey, attVal)
		}
	}

	dp.SetTimestamp(now)
	dp.SetDoubleValue(counterValue.Value)
}

func instancesFromConfig(oc ObjectConfig) []string {
	if len(oc.Instances) == 0 {
		return []string{""}
	}

	for _, instance := range oc.Instances {
		if instance == "*" {
			return []string{"*"}
		}
	}

	return oc.Instances
}
