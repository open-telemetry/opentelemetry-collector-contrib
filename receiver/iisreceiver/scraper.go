// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package iisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver/internal/metadata"
)

type iisReceiver struct {
	params                  component.TelemetrySettings
	config                  *Config
	consumer                consumer.Metrics
	totalWatcherRecorders   []watcherRecorder
	siteWatcherRecorders    []watcherRecorder
	appPoolWatcherRecorders []watcherRecorder
	metricBuilder           *metadata.MetricsBuilder

	// for mocking
	newWatcher func(string, string, string) (winperfcounters.PerfCounterWatcher, error)
}

// watcherRecorder is a struct containing perf counter watcher along with corresponding value recorder.
type watcherRecorder struct {
	watcher  winperfcounters.PerfCounterWatcher
	recorder recordFunc
}

// newIisReceiver returns an iisReceiver
func newIisReceiver(settings receiver.CreateSettings, cfg *Config, consumer consumer.Metrics) *iisReceiver {
	return &iisReceiver{
		params:        settings.TelemetrySettings,
		config:        cfg,
		consumer:      consumer,
		metricBuilder: metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		newWatcher:    winperfcounters.NewWatcher,
	}
}

// start builds the paths to the watchers
func (rcvr *iisReceiver) start(ctx context.Context, host component.Host) error {
	errs := &scrapererror.ScrapeErrors{}

	rcvr.totalWatcherRecorders = rcvr.buildWatcherRecorders(totalPerfCounterRecorders, errs)
	rcvr.siteWatcherRecorders = rcvr.buildWatcherRecorders(sitePerfCounterRecorders, errs)
	rcvr.appPoolWatcherRecorders = rcvr.buildWatcherRecorders(appPoolPerfCounterRecorders, errs)

	return errs.Combine()
}

// scrape pulls counter values from the watchers
func (rcvr *iisReceiver) scrape(ctx context.Context) (pmetric.Metrics, error) {
	var errs error
	now := pcommon.NewTimestampFromTime(time.Now())

	rcvr.scrapeInstanceMetrics(now, rcvr.siteWatcherRecorders, metadata.WithIisSite)
	rcvr.scrapeInstanceMetrics(now, rcvr.appPoolWatcherRecorders, metadata.WithIisApplicationPool)
	rcvr.scrapeTotalMetrics(now)

	return rcvr.metricBuilder.Emit(), errs
}

func (rcvr *iisReceiver) scrapeTotalMetrics(now pcommon.Timestamp) {
	for _, wr := range rcvr.totalWatcherRecorders {
		counterValues, err := wr.watcher.ScrapeData()
		if err != nil {
			rcvr.params.Logger.Warn("some performance counters could not be scraped; ", zap.Error(err))
			continue
		}
		value := 0.0
		for _, counterValue := range counterValues {
			value += counterValue.Value
		}
		wr.recorder(rcvr.metricBuilder, now, value)
	}

	// resource for total metrics is empty
	// this makes it so that the order that the scrape functions are called doesn't matter
	rcvr.metricBuilder.EmitForResource()
}

type valRecorder struct {
	val    float64
	record recordFunc
}

func (rcvr *iisReceiver) scrapeInstanceMetrics(now pcommon.Timestamp, wrs []watcherRecorder, resourceOption func(string) metadata.ResourceMetricsOption) {
	// Maintain a map of instance -> {val, recordFunc}
	// so that we can emit all metrics for a particular instance (site, app_pool) at once,
	// keeping them in a single resource metric.
	instanceToRecorders := map[string][]valRecorder{}

	for _, wr := range wrs {
		counterValues, err := wr.watcher.ScrapeData()
		if err != nil {
			rcvr.params.Logger.Warn("some performance counters could not be scraped; ", zap.Error(err))
			continue
		}

		// This avoids recording the _Total instance.
		// The _Total instance may be the only instance, because some instances require elevated permissions
		// to list and scrape. In these cases, the per-instance metric is not available, and should not be recorded.
		if len(counterValues) == 1 && counterValues[0].InstanceName == "" {
			rcvr.params.Logger.Warn("Performance counter was scraped, but only the _Total instance was available, skipping metric...", zap.String("path", wr.watcher.Path()))
			continue
		}

		for _, cv := range counterValues {
			instanceToRecorders[cv.InstanceName] = append(instanceToRecorders[cv.InstanceName],
				valRecorder{
					val:    cv.Value,
					record: wr.recorder,
				})
		}
	}

	// record all metrics for each instance, then emit them all as a single resource metric
	for instanceName, recorders := range instanceToRecorders {
		for _, recorder := range recorders {
			recorder.record(rcvr.metricBuilder, now, recorder.val)
		}

		rcvr.metricBuilder.EmitForResource(resourceOption(instanceName))
	}
}

// shutdown closes the watchers
func (rcvr iisReceiver) shutdown(ctx context.Context) error {
	var errs error
	errs = multierr.Append(errs, closeWatcherRecorders(rcvr.totalWatcherRecorders))
	errs = multierr.Append(errs, closeWatcherRecorders(rcvr.siteWatcherRecorders))
	errs = multierr.Append(errs, closeWatcherRecorders(rcvr.appPoolWatcherRecorders))
	return errs
}

func (rcvr *iisReceiver) buildWatcherRecorders(confs []perfCounterRecorderConf, scrapeErrors *scrapererror.ScrapeErrors) []watcherRecorder {
	wrs := []watcherRecorder{}

	for _, pcr := range confs {
		for perfCounterName, recorder := range pcr.recorders {
			w, err := rcvr.newWatcher(pcr.object, pcr.instance, perfCounterName)
			if err != nil {
				scrapeErrors.AddPartial(1, err)
				continue
			}
			wrs = append(wrs, watcherRecorder{w, recorder})
		}
	}

	return wrs
}

func closeWatcherRecorders(wrs []watcherRecorder) error {
	var errs error
	for _, wr := range wrs {
		err := wr.watcher.Close()
		if err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	return errs
}
