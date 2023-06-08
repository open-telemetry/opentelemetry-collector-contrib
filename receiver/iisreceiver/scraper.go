// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package iisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver"

import (
	"context"
	"fmt"
	"regexp"
	"strings"
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
	queueMaxAgeWatchers     []instanceWatcher
	metricBuilder           *metadata.MetricsBuilder

	// for mocking
	newWatcher         func(string, string, string) (winperfcounters.PerfCounterWatcher, error)
	newWatcherFromPath func(string) (winperfcounters.PerfCounterWatcher, error)
	expandWildcardPath func(string) ([]string, error)
}

// watcherRecorder is a struct containing perf counter watcher along with corresponding value recorder.
type watcherRecorder struct {
	watcher  winperfcounters.PerfCounterWatcher
	recorder recordFunc
}

// instanceWatcher is a struct containing a perf counter watcher, along with the single instance the watcher records.
type instanceWatcher struct {
	watcher  winperfcounters.PerfCounterWatcher
	instance string
}

// newIisReceiver returns an iisReceiver
func newIisReceiver(settings receiver.CreateSettings, cfg *Config, consumer consumer.Metrics) *iisReceiver {
	return &iisReceiver{
		params:             settings.TelemetrySettings,
		config:             cfg,
		consumer:           consumer,
		metricBuilder:      metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		newWatcher:         winperfcounters.NewWatcher,
		newWatcherFromPath: winperfcounters.NewWatcherFromPath,
		expandWildcardPath: winperfcounters.ExpandWildCardPath,
	}
}

// start builds the paths to the watchers
func (rcvr *iisReceiver) start(ctx context.Context, host component.Host) error {
	errs := &scrapererror.ScrapeErrors{}

	rcvr.totalWatcherRecorders = rcvr.buildWatcherRecorders(totalPerfCounterRecorders, errs)
	rcvr.siteWatcherRecorders = rcvr.buildWatcherRecorders(sitePerfCounterRecorders, errs)
	rcvr.appPoolWatcherRecorders = rcvr.buildWatcherRecorders(appPoolPerfCounterRecorders, errs)
	rcvr.queueMaxAgeWatchers = rcvr.buildMaxQueueItemAgeWatchers(errs)

	return errs.Combine()
}

// scrape pulls counter values from the watchers
func (rcvr *iisReceiver) scrape(ctx context.Context) (pmetric.Metrics, error) {
	var errs error
	now := pcommon.NewTimestampFromTime(time.Now())

	// Maintain maps of site -> {val, recordFunc} and app -> {val, recordFunc}
	// so that we can emit all metrics for a particular instance (site, app_pool) at once,
	// keeping them in a single resource metric.

	siteToRecorders := map[string][]valRecorder{}
	rcvr.scrapeInstanceMetrics(rcvr.siteWatcherRecorders, siteToRecorders)
	rcvr.emitInstanceMap(now, siteToRecorders, metadata.WithIisSite)

	appToRecorders := map[string][]valRecorder{}
	rcvr.scrapeInstanceMetrics(rcvr.appPoolWatcherRecorders, appToRecorders)
	rcvr.scrapeMaxQueueAgeMetrics(appToRecorders)
	rcvr.emitInstanceMap(now, appToRecorders, metadata.WithIisApplicationPool)

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

func (rcvr *iisReceiver) scrapeInstanceMetrics(wrs []watcherRecorder, instanceToRecorders map[string][]valRecorder) {
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

}

func (rcvr *iisReceiver) emitInstanceMap(now pcommon.Timestamp, instanceToRecorders map[string][]valRecorder, resourceOption func(string) metadata.ResourceMetricsOption) {
	// record all metrics for each instance, then emit them all as a single resource metric
	for instanceName, recorders := range instanceToRecorders {
		for _, recorder := range recorders {
			recorder.record(rcvr.metricBuilder, now, recorder.val)
		}

		rcvr.metricBuilder.EmitForResource(resourceOption(instanceName))
	}
}

var negativeDenominatorError = "A counter with a negative denominator value was detected.\r\n"

func (rcvr *iisReceiver) scrapeMaxQueueAgeMetrics(appToRecorders map[string][]valRecorder) {
	for _, wr := range rcvr.queueMaxAgeWatchers {
		counterValues, err := wr.watcher.ScrapeData()

		var value float64
		switch {
		case err != nil && strings.HasSuffix(err.Error(), negativeDenominatorError):
			// This error occurs when there are no items in the queue;
			// in this case, we would like to emit a 0 instead of logging an error (this is an expected scenario).
			value = 0
		case err != nil:
			rcvr.params.Logger.Warn("some performance counters could not be scraped; ", zap.Error(err))
			continue
		case len(counterValues) == 0:
			// No counters scraped
			continue
		default:
			value = counterValues[0].Value
		}

		appToRecorders[wr.instance] = append(appToRecorders[wr.instance],
			valRecorder{
				val:    value,
				record: recordMaxQueueItemAge,
			})
	}
}

// shutdown closes the watchers
func (rcvr iisReceiver) shutdown(ctx context.Context) error {
	var errs error
	errs = multierr.Append(errs, closeWatcherRecorders(rcvr.totalWatcherRecorders))
	errs = multierr.Append(errs, closeWatcherRecorders(rcvr.siteWatcherRecorders))
	errs = multierr.Append(errs, closeWatcherRecorders(rcvr.appPoolWatcherRecorders))
	errs = multierr.Append(errs, closeInstanceWatchers(rcvr.queueMaxAgeWatchers))
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

var maxQueueItemAgeInstanceRegex = regexp.MustCompile(`\\HTTP Service Request Queues\((?P<instance>[^)]+)\)\\MaxQueueItemAge$`)

// buildMaxQueueItemAgeWatchers builds a watcher for each individual instance of the MaxQueueItemAge counter.
// This is done in order to capture the error when scraping each individual instance, because we want to ignore
// negative denominator errors.
func (rcvr *iisReceiver) buildMaxQueueItemAgeWatchers(scrapeErrors *scrapererror.ScrapeErrors) []instanceWatcher {
	wrs := []instanceWatcher{}

	paths, err := rcvr.expandWildcardPath(`\HTTP Service Request Queues(*)\MaxQueueItemAge`)
	if err != nil {
		scrapeErrors.AddPartial(1, fmt.Errorf("failed to expand wildcard path for MaxQueueItemAge: %w", err))
		return wrs
	}

	for _, path := range paths {
		matches := maxQueueItemAgeInstanceRegex.FindStringSubmatch(path)
		if len(matches) != 2 {
			scrapeErrors.AddPartial(1, fmt.Errorf("failed to extract instance from %q", path))
			continue
		}

		instanceName := matches[1]

		if instanceName == "_Total" {
			// skip total instance
			continue
		}

		watcher, err := rcvr.newWatcherFromPath(path)
		if err != nil {
			scrapeErrors.AddPartial(1, fmt.Errorf("failed to create watcher from %q: %w", path, err))
			continue
		}

		wrs = append(wrs, instanceWatcher{
			instance: instanceName,
			watcher:  watcher,
		})
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

func closeInstanceWatchers(wrs []instanceWatcher) error {
	var errs error
	for _, wr := range wrs {
		err := wr.watcher.Close()
		if err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	return errs
}
