// Copyright The OpenTelemetry Authors
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

package iisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver/internal/metadata"
)

type iisReceiver struct {
	params           component.ReceiverCreateSettings
	config           *Config
	consumer         consumer.Metrics
	watcherRecorders []watcherRecorder
	metricBuilder    *metadata.MetricsBuilder

	// for mocking
	newWatcher func(string, string, string) (winperfcounters.PerfCounterWatcher, error)
}

// watcherRecorder is a struct containing perf counter watcher along with corresponding value recorder.
type watcherRecorder struct {
	watcher  winperfcounters.PerfCounterWatcher
	recorder recordFunc
}

// newIisReceiver returns an iisReceiver
func newIisReceiver(params component.ReceiverCreateSettings, cfg *Config, consumer consumer.Metrics) *iisReceiver {
	return &iisReceiver{
		params:        params,
		config:        cfg,
		consumer:      consumer,
		metricBuilder: metadata.NewMetricsBuilder(cfg.Metrics),
		newWatcher:    winperfcounters.NewWatcher,
	}
}

// start builds the paths to the watchers
func (rcvr *iisReceiver) start(ctx context.Context, host component.Host) error {
	rcvr.watcherRecorders = []watcherRecorder{}

	var errors scrapererror.ScrapeErrors
	for _, pcr := range perfCounterRecorders {
		for perfCounterName, recorder := range pcr.recorders {
			w, err := rcvr.newWatcher(pcr.object, pcr.instance, perfCounterName)
			if err != nil {
				errors.AddPartial(1, err)
				continue
			}
			rcvr.watcherRecorders = append(rcvr.watcherRecorders, watcherRecorder{w, recorder})
		}
	}

	return errors.Combine()
}

// scrape pulls counter values from the watchers
func (rcvr *iisReceiver) scrape(ctx context.Context) (pmetric.Metrics, error) {
	var errs error
	now := pcommon.NewTimestampFromTime(time.Now())

	for _, wr := range rcvr.watcherRecorders {
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

	return rcvr.metricBuilder.Emit(), errs
}

// shutdown closes the watchers
func (rcvr iisReceiver) shutdown(ctx context.Context) error {
	var errs error
	for _, wr := range rcvr.watcherRecorders {
		err := wr.watcher.Close()
		if err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	return errs
}
